package push

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mockTx / mockConnector — test doubles for the storage interfaces
// ---------------------------------------------------------------------------

type mockTx struct {
	execCalled     int
	execErr        error
	rollbackCalled int
	commitCalled   int
	commitErr      error
	execSQLs       []string
}

func (m *mockTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	m.execCalled++
	m.execSQLs = append(m.execSQLs, sql)
	return pgconn.CommandTag{}, m.execErr
}

func (m *mockTx) Rollback(_ context.Context) error {
	m.rollbackCalled++
	return nil
}

func (m *mockTx) Commit(_ context.Context) error {
	m.commitCalled++
	return m.commitErr
}

type mockConnector struct {
	beginCalled int
	beginTx     *mockTx
	beginErr    error
}

func (m *mockConnector) Begin(_ context.Context) (sqlTx, error) {
	m.beginCalled++
	return m.beginTx, m.beginErr
}

func (m *mockConnector) Close(_ context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// isNonTransactionalSQL
// ---------------------------------------------------------------------------

func TestIsNonTransactionalSQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"CREATE INDEX CONCURRENTLY", "CREATE INDEX CONCURRENTLY idx_name ON users(name);", true},
		{"DROP INDEX CONCURRENTLY", "DROP INDEX CONCURRENTLY idx_name;", true},
		{"REINDEX INDEX CONCURRENTLY", "REINDEX INDEX CONCURRENTLY idx_name;", true},
		{"lowercase concurrently", "create index concurrently idx on t(c);", true},
		{"normal CREATE INDEX", "CREATE INDEX idx_name ON users(name);", false},
		{"plain SELECT", "SELECT 1;", false},
		{"CONCURRENTLY in comment only", "-- CREATE INDEX CONCURRENTLY\nCREATE INDEX idx ON t(c);", false},
		{"CONCURRENTLY as table name", "CREATE TABLE concurrently (id int);", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonTransactionalSQL(tt.sql)
			require.Equal(t, tt.want, got, "isNonTransactionalSQL(%q)", tt.sql)
		})
	}
}

// ---------------------------------------------------------------------------
// stripSQLComments
// ---------------------------------------------------------------------------

func TestStripSQLComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"no comments", "CREATE TABLE t (id int);", "CREATE TABLE t (id int);"},
		{"inline comment", "CREATE TABLE t (id int); -- pk", "CREATE TABLE t (id int); "},
		{"full line comment", "-- comment\nCREATE TABLE t (id int);", "\nCREATE TABLE t (id int);"},
		{"multiple comments", "-- h\nSELECT * FROM t; -- e", "\nSELECT * FROM t; "},
		{"CONCURRENTLY with trailing comment", "CREATE INDEX CONCURRENTLY idx\n  ON t(c); -- online", "CREATE INDEX CONCURRENTLY idx\n  ON t(c); "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSQLComments(tt.sql)
			require.Equal(t, tt.want, got, "stripSQLComments(%q)", tt.sql)
		})
	}
}

// ---------------------------------------------------------------------------
// readUserInput
// ---------------------------------------------------------------------------

func TestReadUserInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"yes", "y\n", "y"},
		{"no", "n\n", "n"},
		{"apply all", "a\n", "a"},
		{"skip", "s\n", "s"},
		{"uppercase Y", "Y\n", "y"},
		{"mixed case", "Auto\n", "auto"},
		{"whitespace trimmed", "  y  \n", "y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := readUserInput(reader)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestReadUserInput_EOF(t *testing.T) {
	got := readUserInput(bufio.NewReader(strings.NewReader("")))
	require.Equal(t, "n", got)
}

// ---------------------------------------------------------------------------
// execSQL
// ---------------------------------------------------------------------------

func TestExecSQL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockTx{}
		svc := &PushService{}
		err := svc.executeOne(context.Background(), mock, "CREATE TABLE t (id int);", 1)
		require.NoError(t, err)
		require.Equal(t, 1, mock.execCalled)
		require.Equal(t, []string{"CREATE TABLE t (id int);"}, mock.execSQLs)
	})

	t.Run("error", func(t *testing.T) {
		mock := &mockTx{execErr: errors.New("syntax error")}
		svc := &PushService{}
		err := svc.executeOne(context.Background(), mock, "BAD SQL;", 5)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SQL #5")
		require.Contains(t, err.Error(), "syntax error")
	})
}

// ---------------------------------------------------------------------------
// ExecutePlan (via mock connector)
// ---------------------------------------------------------------------------

func TestExecutePlan_BasicInteractive(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("y\n")),
	}

	ops := []diff.Operation{diff.NewAddTableOp("public", "users", &model.Table{Name: "users"})}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 1, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_ExecuteMode(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
	}

	ops := []diff.Operation{
		diff.NewAddTableOp("public", "users", &model.Table{Name: "users"}),
		diff.NewAddColumnOp("public", "users", &model.Column{Name: "id", DataType: "integer"}),
	}
	cfg := app.PushConfig{DiffConfig: app.DiffConfig{}, Execute: true}

	err := svc.ExecutePlan(context.Background(), cfg, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 2, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_ExecuteModeBlocksDestructive(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
	}

	ops := []diff.Operation{diff.NewDropTableOp("public", "users")}
	cfg := app.PushConfig{DiffConfig: app.DiffConfig{UnsafeDrop: false}, Execute: true}

	err := svc.ExecutePlan(context.Background(), cfg, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 0, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_UnsafeDropAllowsDestructive(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
	}

	ops := []diff.Operation{diff.NewDropTableOp("public", "users")}
	cfg := app.PushConfig{DiffConfig: app.DiffConfig{UnsafeDrop: true}, Execute: true}

	err := svc.ExecutePlan(context.Background(), cfg, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 1, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_UserCancels(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("n\n")),
	}

	ops := []diff.Operation{diff.NewAddTableOp("public", "users", &model.Table{Name: "users"})}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 0, mockTx.execCalled)
	require.Equal(t, 1, mockTx.rollbackCalled)
}

func TestExecutePlan_Skip(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("s\n")),
	}

	ops := []diff.Operation{diff.NewAddTableOp("public", "users", &model.Table{Name: "users"})}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 0, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_ApplyAll(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("a\n")),
	}

	ops := []diff.Operation{
		diff.NewAddTableOp("public", "t1", &model.Table{Name: "t1"}),
		diff.NewAddTableOp("public", "t2", &model.Table{Name: "t2"}),
	}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 2, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_InvalidInput(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("x\ny\n")),
	}

	ops := []diff.Operation{diff.NewAddTableOp("public", "users", &model.Table{Name: "users"})}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 1, mockTx.execCalled)
	require.Equal(t, 1, mockTx.commitCalled)
}

func TestExecutePlan_CommitError(t *testing.T) {
	mockTx := &mockTx{commitErr: errors.New("disk full")}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("y\n")),
	}

	ops := []diff.Operation{diff.NewAddTableOp("public", "users", &model.Table{Name: "users"})}
	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), ops)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to commit")
}

func TestExecutePlan_ConnectError(t *testing.T) {
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) {
			return nil, errors.New("connection refused")
		},
	}

	err := svc.ExecutePlan(context.Background(), app.PushConfig{DiffConfig: app.DiffConfig{}}, model.NewSchema(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to connect")
}

func TestExecutePlan_DestructiveInteractiveAllow(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("y\n")),
	}

	ops := []diff.Operation{diff.NewDropTableOp("public", "users")}
	cfg := app.PushConfig{DiffConfig: app.DiffConfig{UnsafeDrop: true}}

	err := svc.ExecutePlan(context.Background(), cfg, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 1, mockTx.execCalled)
}

func TestExecutePlan_DestructiveInteractiveBlock(t *testing.T) {
	mockTx := &mockTx{}
	conn := &mockConnector{beginTx: mockTx}
	svc := &PushService{
		connectFunc: func(_ context.Context, _ string) (dbConnector, error) { return conn, nil },
		stdinReader: bufio.NewReader(strings.NewReader("y\n")),
	}

	ops := []diff.Operation{diff.NewDropTableOp("public", "users")}
	cfg := app.PushConfig{DiffConfig: app.DiffConfig{UnsafeDrop: false}}

	err := svc.ExecutePlan(context.Background(), cfg, model.NewSchema(), ops)
	require.NoError(t, err)
	require.Equal(t, 0, mockTx.execCalled)
}
