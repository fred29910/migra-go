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
		array_agg(a.attname ORDER BY k.ordinality) FILTER (WHERE a.attname IS NOT NULL) AS column_names,
		i.indisunique AS is_unique,
		am.amname AS method,
		pg_get_indexdef(i.indexrelid) AS definition,
		COALESCE(pg_get_expr(i.indpred, i.indrelid), '') AS predicate
	FROM pg_index i
	JOIN pg_class idx ON idx.oid = i.indexrelid
	JOIN pg_class t ON t.oid = i.indrelid
	JOIN pg_namespace n ON n.oid = idx.relnamespace
	JOIN pg_am am ON am.oid = idx.relam
	LEFT JOIN LATERAL unnest(i.indkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON true
	LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
	WHERE n.nspname = $1 AND i.indisprimary = false
	GROUP BY idx.oid, idx.relname, t.relname, i.indisunique, am.amname, i.indexrelid, i.indpred, i.indrelid`

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
		definition  string
		predicate   string
	)

	for rows.Next() {
		err := rows.Scan(&indexName, &tableName, &columnNames, &isUnique, &method, &definition, &predicate)
		if err != nil {
			return fmt.Errorf("scan index row: %w", err)
		}

		table, exists := ns.Tables[tableName]
		if !exists {
			continue
		}

		index := &model.Index{
			Name:        indexName,
			Table:       tableName,
			Columns:     columnNames,
			Unique:      isUnique,
			Method:      method,
			Definition:  definition,
			WhereClause: predicate,
		}

		table.Indexes[indexName] = index
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate index rows: %w", err)
	}
	return nil
}
