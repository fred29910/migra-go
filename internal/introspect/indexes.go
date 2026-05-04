package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/migra-go/migra-go/internal/model"
)

// loadIndexes loads indexes from pg_indexes
func loadIndexes(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT indexname, indexdef, tablename, indexname
	FROM pg_indexes
	WHERE schemaname = $1`

	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer rows.Close()

	var (
		indexName  string
		indexDef   string
		tableName  string
		indexName2 string // duplicate due to query
	)

	for rows.Next() {
		err := rows.Scan(&indexName, &indexDef, &tableName, &indexName2)
		if err != nil {
			return fmt.Errorf("scan index row: %w", err)
		}

		table, exists := ns.Tables[tableName]
		if !exists {
			continue
		}

		// Parse index definition to extract columns and uniqueness
		unique := strings.Contains(strings.ToLower(indexDef), "unique")
		columns := parseIndexColumns(indexDef)

		index := &model.Index{
			Name:    indexName,
			Table:   tableName,
			Columns: columns,
			Unique:  unique,
			Method:  "btree", // Default, could parse from definition
		}

		table.Indexes[indexName] = index
	}

	return rows.Err()
}

// parseIndexColumns extracts column names from index definition
// Example: "CREATE UNIQUE INDEX idx_name ON table USING btree (col1, col2)"
func parseIndexColumns(indexDef string) []string {
	// Find content within parentheses after the table clause
	start := strings.LastIndex(indexDef, "(")
	end := strings.LastIndex(indexDef, ")")
	if start == -1 || end == -1 || start >= end {
		return []string{}
	}

	content := indexDef[start+1 : end]
	// Handle cases like "col1, col2 DESC"
	parts := strings.Split(content, ",")
	columns := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		// Remove modifiers like DESC, ASC, NULLS FIRST, etc.
		if idx := strings.Index(col, " "); idx != -1 {
			col = col[:idx]
		}
		// Remove quotes
		col = strings.Trim(col, "\"")
		columns = append(columns, col)
	}
	return columns
}
