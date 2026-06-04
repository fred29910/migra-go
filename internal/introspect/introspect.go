package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

// Querier abstracts the query capability needed from a database connection.
// Both *pgx.Conn and pgxmock implement this interface.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// LoadOptions contains options for loading schema from database
type LoadOptions struct {
	Schemas []string // If empty, defaults to []string{"public"}
}

// LoadFromDB reads schema information from a PostgreSQL database
func LoadFromDB(ctx context.Context, connStr string, opt LoadOptions) (*model.Schema, error) {
	// Parse connection config (supports PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE env vars,
	// as well as service= and .pgpass via pgx)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	return LoadFromDBWithConn(ctx, conn, opt)
}

// LoadFromDBWithConn reads schema using an existing connection
func LoadFromDBWithConn(ctx context.Context, conn *pgx.Conn, opt LoadOptions) (*model.Schema, error) {
	return loadFromQuerier(ctx, conn, opt)
}

func loadFromQuerier(ctx context.Context, q Querier, opt LoadOptions) (*model.Schema, error) {
	schema := model.NewSchema()

	schemas := opt.Schemas
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	for _, schemaName := range schemas {
		ns, err := loadNamespace(ctx, q, schemaName)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema %s: %w", schemaName, err)
		}
		if ns != nil {
			schema.Schemas[schemaName] = ns
		}
	}

	return schema, nil
}

func loadNamespace(ctx context.Context, q Querier, schemaName string) (*model.Namespace, error) {
	ns := model.NewNamespace(schemaName)

	if err := loadTables(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load tables: %w", err)
	}
	if err := loadConstraints(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load constraints: %w", err)
	}
	if err := loadForeignKeys(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load foreign keys: %w", err)
	}
	if err := loadIndexes(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load indexes: %w", err)
	}
	if err := loadEnumTypes(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load enum types: %w", err)
	}
	if err := loadViews(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load views: %w", err)
	}
	if err := loadSequences(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load sequences: %w", err)
	}
	if err := loadExtensions(ctx, q, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load extensions: %w", err)
	}

	return ns, nil
}
