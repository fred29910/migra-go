package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

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
	schema := model.NewSchema()

	// Default to public schema if none specified
	schemas := opt.Schemas
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	// Load each schema
	for _, schemaName := range schemas {
		ns, err := loadNamespace(ctx, conn, schemaName)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema %s: %w", schemaName, err)
		}
		if ns != nil {
			schema.Schemas[schemaName] = ns
		}
	}

	return schema, nil
}

// loadNamespace loads a single namespace (schema)
func loadNamespace(ctx context.Context, conn *pgx.Conn, schemaName string) (*model.Namespace, error) {
	ns := model.NewNamespace(schemaName)

	// Load tables and columns
	if err := loadTables(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load tables: %w", err)
	}

	// Load constraints (primary keys, unique, check)
	if err := loadConstraints(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load constraints: %w", err)
	}

	// Load foreign key constraints
	if err := loadForeignKeys(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load foreign keys: %w", err)
	}

	// Load indexes
	if err := loadIndexes(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load indexes: %w", err)
	}

	// Load enum types
	if err := loadEnumTypes(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load enum types: %w", err)
	}

	// Load views
	if err := loadViews(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load views: %w", err)
	}

	// Load sequences
	if err := loadSequences(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load sequences: %w", err)
	}

	// Load extensions
	if err := loadExtensions(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load extensions: %w", err)
	}

	return ns, nil
}
