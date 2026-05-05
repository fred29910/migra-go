package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/fred29910/migra-go/internal/model"
)

// loadConstraints loads constraints from pg_constraint
func loadConstraints(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		pgc.conname,
		pgc.contype,
		pg_get_constraintdef(pgc.oid) AS definition,
		tbl.relname as table_name
	FROM pg_constraint pgc
	JOIN pg_class tbl ON tbl.oid = pgc.conrelid
	JOIN pg_namespace nsp ON nsp.oid = tbl.relnamespace
	WHERE nsp.nspname = $1
	AND pgc.contype IN ('p', 'u', 'c')` // p=primary key, u=unique, c=check

	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query constraints: %w", err)
	}
	defer rows.Close()

	var (
		conName     string
		conType     string
		definition  string
		tableName   string
	)

	for rows.Next() {
		err := rows.Scan(&conName, &conType, &definition, &tableName)
		if err != nil {
			return fmt.Errorf("scan constraint row: %w", err)
		}

		table, exists := ns.Tables[tableName]
		if !exists {
			continue
		}

		constraint := &model.Constraint{
			Name:       conName,
			Table:      tableName,
			Definition: definition,
		}

		switch conType {
		case "p":
			constraint.Type = "primary_key"
			// Also set the PrimaryKey field on the table
			// Parse definition to extract column names
			columns := parseConstraintColumns(definition)
			table.PrimaryKey = &model.PrimaryKey{
				Name:    conName,
				Columns: columns,
			}
		case "u":
			constraint.Type = "unique"
		case "c":
			constraint.Type = "check"
		}

		table.Constraints[conName] = constraint
	}

	return rows.Err()
}

// parseConstraintColumns extracts column names from constraint definition
// Example: "PRIMARY KEY (id)" -> ["id"]
// Example: "UNIQUE (col1, col2)" -> ["col1", "col2"]
func parseConstraintColumns(def string) []string {
	// Find content within parentheses
	start := strings.Index(def, "(")
	end := strings.LastIndex(def, ")")
	if start == -1 || end == -1 || start >= end {
		return []string{}
	}

	content := def[start+1 : end]
	parts := strings.Split(content, ",")
	columns := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		// Remove quotes if present
		col = strings.Trim(col, "\"")
		columns = append(columns, col)
	}
	return columns
}
