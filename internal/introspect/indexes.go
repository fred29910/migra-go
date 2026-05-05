package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

// loadIndexes loads indexes from pg_index with structured column information
func loadIndexes(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		idx.relname AS index_name,
		t.relname AS table_name,
		array_agg(a.attname ORDER BY k.ordinality) AS column_names,
		i.indisunique AS is_unique,
		am.amname AS method
	FROM pg_index i
	JOIN pg_class idx ON idx.oid = i.indexrelid
	JOIN pg_class t ON t.oid = i.indrelid
	JOIN pg_namespace n ON n.oid = idx.relnamespace
	JOIN pg_am am ON am.oid = idx.relam
	JOIN LATERAL unnest(i.indkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON true
	JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
	WHERE n.nspname = $1 AND i.indisprimary = false
	GROUP BY idx.oid, idx.relname, t.relname, i.indisunique, am.amname`

	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer rows.Close()

	var (
		indexName   string
		tableName   string
		columnNames []string
		isUnique    bool
		method      string
	)

	for rows.Next() {
		err := rows.Scan(&indexName, &tableName, &columnNames, &isUnique, &method)
		if err != nil {
			return fmt.Errorf("scan index row: %w", err)
		}

		table, exists := ns.Tables[tableName]
		if !exists {
			continue
		}

		index := &model.Index{
			Name:    indexName,
			Table:   tableName,
			Columns: columnNames,
			Unique:  isUnique,
			Method:  method,
		}

		table.Indexes[indexName] = index
	}

	return rows.Err()
}
