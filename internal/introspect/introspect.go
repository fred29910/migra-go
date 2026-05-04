package introspect

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/migra-go/migra-go/internal/model"
)

// LoadOptions contains options for loading schema from database
type LoadOptions struct {
	Schemas []string // If empty, defaults to []string{"public"}
}

// LoadFromDB reads schema information from a PostgreSQL database
func LoadFromDB(ctx context.Context, conn *pgx.Conn, opt LoadOptions) (*model.Schema, error) {
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
	ns := &model.Namespace{
		Name:   schemaName,
		Tables: make(map[string]*model.Table),
		Types:  make(map[string]*model.EnumType),
	}

	// Load tables and columns
	if err := loadTables(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load tables: %w", err)
	}

	// Load constraints (primary keys, unique, check)
	if err := loadConstraints(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load constraints: %w", err)
	}

	// Load indexes
	if err := loadIndexes(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load indexes: %w", err)
	}

	// Load enum types
	if err := loadEnumTypes(ctx, conn, schemaName, ns); err != nil {
		return nil, fmt.Errorf("failed to load enum types: %w", err)
	}

	return ns, nil
}
