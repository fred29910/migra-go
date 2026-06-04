package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

func loadConstraints(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		c.conname,
		c.contype::text,
		t.relname AS table_name,
		array_agg(a.attname ORDER BY k.ordinality) AS column_names,
		pg_get_constraintdef(c.oid) AS definition
	FROM pg_constraint c
	JOIN pg_class t ON t.oid = c.conrelid
	JOIN pg_namespace n ON n.oid = t.relnamespace
	LEFT JOIN LATERAL unnest(c.conkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON true
	LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
	WHERE n.nspname = $1 AND c.contype IN ('p', 'u', 'c')
	GROUP BY c.oid, c.conname, c.contype, t.relname`

	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query constraints: %w", err)
	}
	defer rows.Close()

	var (
		conName     string
		conType     string
		tableName   string
		columnNames []string
		definition  string
	)

	for rows.Next() {
		err := rows.Scan(&conName, &conType, &tableName, &columnNames, &definition)
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
			Columns:    columnNames,
		}

		switch conType {
		case "p":
			constraint.Type = "primary_key"
			// Also set the PrimaryKey field on the table using structured column names
			if len(columnNames) > 0 {
				table.PrimaryKey = &model.PrimaryKey{
					Name:    conName,
					Columns: columnNames,
				}
			}
		case "u":
			constraint.Type = "unique"
		case "c":
			constraint.Type = "check"
		}

		table.Constraints[conName] = constraint
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate constraint rows: %w", err)
	}
	return nil
}

// pgConstraintAction delegates to the shared FK action code mapping in the model package.
// The same single-character codes appear in pg_constraint.confupdtype/confdeltype
// and are used by pg_query for CREATE TABLE parsing.
func pgConstraintAction(code string) string {
	return model.FKActionCode(code)
}

func loadForeignKeys(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		c.conname,
		t.relname AS table_name,
		array_agg(a.attname ORDER BY k.ordinality) AS column_names,
		pg_get_constraintdef(c.oid) AS definition,
		rt.relname AS ref_table,
		rn.nspname AS ref_schema,
		array_agg(ra.attname ORDER BY rk.ordinality) AS ref_column_names,
		c.confupdtype,
		c.confdeltype
	FROM pg_constraint c
	JOIN pg_class t ON t.oid = c.conrelid
	JOIN pg_namespace n ON n.oid = t.relnamespace
	LEFT JOIN LATERAL unnest(c.conkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON true
	LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
	JOIN pg_class rt ON rt.oid = c.confrelid
	JOIN pg_namespace rn ON rn.oid = rt.relnamespace
	LEFT JOIN LATERAL unnest(c.confkey::int[]) WITH ORDINALITY AS rk(attnum, ordinality) ON true
	LEFT JOIN pg_attribute ra ON ra.attrelid = rt.oid AND ra.attnum = rk.attnum
	WHERE n.nspname = $1 AND c.contype = 'f'
	GROUP BY c.oid, c.conname, t.relname, rt.relname, rn.nspname`

	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		conName        string
		tableName      string
		columnNames    []string
		definition     string
		refTable       string
		refSchema      string
		refColumnNames []string
		confUpdType    string
		confDelType    string
	)

	for rows.Next() {
		err := rows.Scan(&conName, &tableName, &columnNames, &definition, &refTable, &refSchema, &refColumnNames, &confUpdType, &confDelType)
		if err != nil {
			return fmt.Errorf("scan foreign key row: %w", err)
		}

		table, exists := ns.Tables[tableName]
		if !exists {
			continue
		}

		constraint := &model.Constraint{
			Name:       conName,
			Type:       "foreign_key",
			Table:      tableName,
			Definition: definition,
			Columns:    columnNames,
			RefSchema:  refSchema,
			RefTable:   refTable,
			RefColumns: refColumnNames,
			OnDelete:   pgConstraintAction(confDelType),
			OnUpdate:   pgConstraintAction(confUpdType),
		}

		table.Constraints[conName] = constraint
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign key rows: %w", err)
	}
	return nil
}
