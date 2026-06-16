package push

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/render"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// sqlTx wraps the minimal methods we need from a transaction.
// pgx.Tx satisfies this interface.
type sqlTx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Rollback(ctx context.Context) error
	Commit(ctx context.Context) error
}

// dbConnector is the minimal interface needed to connect to a database.
// pgx.Connect returns a *pgx.Conn which satisfies this interface.
// The Begin method returns a sqlTx (which pgx.Tx satisfies) so that
// tests can inject a mock transaction.
type dbConnector interface {
	Begin(ctx context.Context) (sqlTx, error)
	Close(ctx context.Context) error
}

// connectFunc is the function signature for connecting to a database.
type connectFunc func(ctx context.Context, connString string) (dbConnector, error)

// transactionState manages a database transaction's lifecycle, tracking
// whether the transaction is active for safe deferred rollback.
type transactionState struct {
	tx     sqlTx
	active bool
}

func (ts *transactionState) rollback(ctx context.Context) {
	if ts.active {
		_ = ts.tx.Rollback(ctx)
		ts.active = false
	}
}

// Service defines the interface for pushing schema changes to a database.
type Service interface {
	ExecutePlan(ctx context.Context, cfg app.PushConfig, sourceSchema *model.Schema, ops []diff.Operation) error
}

// PushService handles the execution of schema changes with interactive confirmation.
type PushService struct {
	connectFunc connectFunc
	stdinReader *bufio.Reader
}

// Compile-time check that PushService satisfies the Service interface.
var _ Service = (*PushService)(nil)

// NewPushService creates a new PushService with default dependencies.
func NewPushService() Service {
	return &PushService{
		connectFunc: defaultConnectToDB,
		stdinReader: bufio.NewReader(os.Stdin),
	}
}

// setupSignalHandler creates a cancellable context for the execution
// phase so that interrupt signals can trigger a graceful rollback
// instead of calling os.Exit(1).
func (s *PushService) setupSignalHandler(ctx context.Context) (context.Context, context.CancelFunc) {
	execCtx, execCancel := context.WithCancel(ctx)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigChan)
		select {
		case <-sigChan:
			fmt.Println("\nInterrupt received, cancelling...")
			execCancel()
		case <-execCtx.Done():
		}
	}()
	return execCtx, execCancel
}

// ExecutePlan executes a plan of operations with interactive confirmation.
func (s *PushService) ExecutePlan(ctx context.Context, cfg app.PushConfig, sourceSchema *model.Schema, ops []diff.Operation) error {
	execCtx, execCancel := s.setupSignalHandler(ctx)
	defer execCancel()

	conn, err := s.connectFunc(ctx, cfg.Target)
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	tx, err := conn.Begin(execCtx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	ts := &transactionState{tx: tx, active: true}
	defer ts.rollback(ctx)

	renderer := render.NewRenderer()

	switch {
	case cfg.Execute:
		return s.executeAuto(execCtx, ts, renderer, ops, cfg)
	default:
		return s.executeInteractive(execCtx, ts, renderer, ops, cfg)
	}
}

// executeOne executes a single SQL statement and logs the result.
func (s *PushService) executeOne(ctx context.Context, tx sqlTx, sql string, idx int) error {
	_, err := tx.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("error executing SQL #%d: %w", idx, err)
	}
	fmt.Printf("SQL #%d executed\n", idx)
	return nil
}

func (s *PushService) failWithRollback(ctx context.Context, ts *transactionState, err error) error {
	fmt.Println("Rolling back transaction...")
	ts.rollback(ctx)
	return err
}

func (s *PushService) commitTransaction(ctx context.Context, ts *transactionState) error {
	if err := ts.tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	ts.active = false
	fmt.Println("\nAll SQL executed successfully, transaction committed")
	return nil
}

func (s *PushService) executeAuto(ctx context.Context, ts *transactionState, renderer *render.Renderer, ops []diff.Operation, cfg app.PushConfig) error {
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return s.failWithRollback(ctx, ts, fmt.Errorf("execution interrupted: %w", err))
		}
		sql := renderer.Render(op)
		if sql == "" || strings.HasPrefix(sql, "-- Unknown") {
			continue
		}
		if isNonTransactionalSQL(sql) {
			return s.failWithRollback(ctx, ts, fmt.Errorf("non-transactional DDL at SQL #%d", i+1))
		}
		if op.IsDestructive() && !cfg.UnsafeDrop {
			fmt.Printf("Destructive operation blocked at SQL #%d (use --unsafe-drop to allow):\n  %s\n", i+1, sql)
			continue
		}
		if err := s.executeOne(ctx, ts.tx, sql, i+1); err != nil {
			return s.failWithRollback(ctx, ts, err)
		}
	}
	return s.commitTransaction(ctx, ts)
}

func (s *PushService) promptAndExecute(ctx context.Context, ts *transactionState, renderer *render.Renderer, op diff.Operation, sql string, i int, applyAll *bool, cfg app.PushConfig) error {
	isDestructive := op.IsDestructive()
	idx := i + 1

	if *applyAll {
		if isDestructive && !cfg.UnsafeDrop {
			fmt.Printf("Destructive operation detected in auto-mode, reverting to interactive.\nSQL #%d: %s\n\n", idx, sql)
			*applyAll = false
		} else {
			return s.executeOne(ctx, ts.tx, sql, idx)
		}
	}

	prompt := fmt.Sprintf("Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", idx)
	if isDestructive {
		prompt = fmt.Sprintf("DANGER: Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", idx)
	}

	for {
		fmt.Print(prompt)
		input := strings.ToLower(strings.TrimSpace(readUserInput(s.stdinReader)))
		switch input {
		case "y":
			if isDestructive && !cfg.UnsafeDrop {
				fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
				continue
			}
			return s.executeOne(ctx, ts.tx, sql, idx)
		case "n":
			fmt.Println("Cancelled")
			ts.rollback(ctx)
			return nil
		case "a":
			*applyAll = true
			if isDestructive && !cfg.UnsafeDrop {
				fmt.Printf("Destructive operation detected...\n")
				*applyAll = false
				continue
			}
			return s.executeOne(ctx, ts.tx, sql, idx)
		case "s":
			fmt.Printf("SQL #%d skipped\n", idx)
			return nil
		default:
			fmt.Println("Invalid input. Use: y, n, a, or s")
		}
	}
}

func (s *PushService) executeInteractive(ctx context.Context, ts *transactionState, renderer *render.Renderer, ops []diff.Operation, cfg app.PushConfig) error {
	var applyAll bool
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return s.failWithRollback(ctx, ts, fmt.Errorf("execution interrupted: %w", err))
		}
		sql := renderer.Render(op)
		if sql == "" || strings.HasPrefix(sql, "-- Unknown") {
			continue
		}
		if isNonTransactionalSQL(sql) {
			fmt.Printf("\nNon-transactional DDL detected at SQL #%d...\n", i+1)
			return s.failWithRollback(ctx, ts, fmt.Errorf("non-transactional DDL at SQL #%d", i+1))
		}
		if err := s.promptAndExecute(ctx, ts, renderer, op, sql, i, &applyAll, cfg); err != nil {
			return err
		}
		if !ts.active {
			return nil
		}
	}
	return s.commitTransaction(ctx, ts)
}

// readUserInput reads user input from stdin.
func readUserInput(reader *bufio.Reader) string {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "n"
	}
	return strings.ToLower(strings.TrimSpace(input))
}

var nonTransactionalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bCREATE\s+INDEX\s+CONCURRENTLY\b`),
	regexp.MustCompile(`(?i)\bDROP\s+INDEX\s+CONCURRENTLY\b`),
	regexp.MustCompile(`(?i)\bREINDEX\s+INDEX\s+CONCURRENTLY\b`),
}

// stripSQLComments removes SQL comments from a statement.
func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// isNonTransactionalSQL checks if a SQL statement is non-transactional.
func isNonTransactionalSQL(sql string) bool {
	cleaned := stripSQLComments(sql)
	for _, pattern := range nonTransactionalPatterns {
		if pattern.MatchString(cleaned) {
			return true
		}
	}
	return false
}

// pgxConnAdapter wraps *pgx.Conn so that Begin returns sqlTx (which pgx.Tx satisfies).
type pgxConnAdapter struct {
	conn *pgx.Conn
}

func (a *pgxConnAdapter) Begin(ctx context.Context) (sqlTx, error) {
	return a.conn.Begin(ctx)
}

func (a *pgxConnAdapter) Close(ctx context.Context) error {
	return a.conn.Close(ctx)
}

// defaultConnectToDB is the default database connector.
var defaultConnectToDB connectFunc = func(ctx context.Context, connString string) (dbConnector, error) {
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, err
	}
	return &pgxConnAdapter{conn: conn}, nil
}