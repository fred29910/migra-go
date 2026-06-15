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
	"time"

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

// PushService handles the execution of schema changes with interactive confirmation.
type PushService struct {
	connectFunc connectFunc
	stdinReader *bufio.Reader
}

// NewPushService creates a new PushService with default dependencies.
func NewPushService() *PushService {
	return &PushService{
		connectFunc: defaultConnectToDB,
		stdinReader: bufio.NewReader(os.Stdin),
	}
}

// ExecutePlan executes a plan of operations with interactive confirmation.
func (s *PushService) ExecutePlan(ctx context.Context, cfg app.Config, sourceSchema *model.Schema, ops []diff.Operation) error {
	// Create a cancellable context for the execution phase so that
	// interrupt signals can trigger a graceful rollback instead of
	// calling os.Exit(1).
	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()

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

	conn, err := s.connectFunc(ctx, cfg.Target)
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	tx, err := conn.Begin(execCtx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	txActive := true
	defer func() {
		if txActive {
			_ = tx.Rollback(ctx)
		}
	}()

	// checkInterrupt returns a non-nil error if the context has been
	// cancelled (e.g. by an interrupt signal).
	checkInterrupt := func() error {
		if err := execCtx.Err(); err != nil {
			fmt.Println("Rolling back transaction...")
			_ = tx.Rollback(ctx)
			txActive = false
			return fmt.Errorf("execution interrupted: %w", err)
		}
		return nil
	}

	renderer := render.NewRenderer()

	next:
	for i, op := range ops {
		// Check for interrupt at the start of each iteration.
		if err := checkInterrupt(); err != nil {
			return err
		}

		sql := renderer.Render(op)
		if sql == "" || strings.HasPrefix(sql, "-- Unknown") {
			continue
		}

		isDestructive := op.IsDestructive()

		if isNonTransactionalSQL(sql) {
			fmt.Printf("\nNon-transactional DDL detected at SQL #%d:\n  %s\nThis operation cannot be executed within a transaction.\nPlease execute it separately outside this tool.\n", i+1, sql)
			_ = tx.Rollback(ctx)
			txActive = false
			return fmt.Errorf("non-transactional DDL at SQL #%d", i+1)
		}

		// Interactive confirmation
		prompt := fmt.Sprintf("Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
		if isDestructive {
			prompt = fmt.Sprintf("DANGER: Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
		}

		fmt.Print(prompt)

		input := readUserInput(s.stdinReader)

		// Check for interrupt after blocking read.
		if err := checkInterrupt(); err != nil {
			return err
		}

		switch input {
		case "y":
			if isDestructive && !cfg.UnsafeDrop {
				fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
				continue next
			}
			if err := execSQL(ctx, tx, sql, i+1); err != nil {
				fmt.Println("Rolling back transaction...")
				_ = tx.Rollback(ctx)
				txActive = false
				return fmt.Errorf("execution failed at SQL #%d, transaction rolled back: %w", i+1, err)
			}
			continue next
		case "n":
			fmt.Println("Cancelled")
			_ = tx.Rollback(ctx)
			txActive = false
			return nil
		case "a":
			if isDestructive && !cfg.UnsafeDrop {
				fmt.Printf("Destructive operation detected in auto-mode, reverting to interactive mode.\nSQL #%d: %s\n\n", i+1, sql)
				continue
			}
			// Fall through to auto mode
			fallthrough
		case "auto":
			// Auto mode: execute without confirmation
			if err := checkInterrupt(); err != nil {
				return err
			}
			if err := execSQL(ctx, tx, sql, i+1); err != nil {
				fmt.Println("Rolling back transaction...")
				_ = tx.Rollback(ctx)
				txActive = false
				return fmt.Errorf("execution failed at SQL #%d, transaction rolled back: %w", i+1, err)
			}
			continue next
		case "s":
			fmt.Printf("SQL #%d skipped\n", i+1)
			continue next
		default:
			fmt.Println("Invalid input. Use: y, n, a, or s")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	txActive = false
	fmt.Println("\nAll SQL executed successfully, transaction committed")

	return nil
}

// readUserInput reads user input from stdin.
func readUserInput(reader *bufio.Reader) string {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "n"
	}
	return strings.ToLower(strings.TrimSpace(input))
}

// execSQL executes a single SQL statement and logs the result.
func execSQL(ctx context.Context, tx sqlTx, sql string, idx int) error {
	_, err := tx.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("error executing SQL #%d: %w", idx, err)
	}
	fmt.Printf("SQL #%d executed\n", idx)
	return nil
}

// isDestructiveOperation checks if an operation is destructive.
func isDestructiveOperation(isDestructive, unsafeDrop bool) bool {
	return isDestructive && !unsafeDrop
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