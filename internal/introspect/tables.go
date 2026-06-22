package introspect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

func loadTables(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		c.relname                            AS table_name,
		a.attname                            AS column_name,
		format_type(a.atttypid, a.atttypmod) AS data_type,
		a.attnotnull                         AS is_not_null,
		pg_get_expr(ad.adbin, ad.adrelid)    AS column_default,
		a.attnum                             AS ordinal_position,
		a.attidentity                        AS is_identity,
		COALESCE(coll.collname, '')          AS collation_name
	FROM pg_catalog.pg_class c
	JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
	LEFT JOIN pg_catalog.pg_attrdef ad
		ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum AND a.atthasdef
	LEFT JOIN pg_catalog.pg_collation coll ON coll.oid = a.attcollation
	WHERE n.nspname = $1
		AND c.relkind = 'r'
		AND a.attnum > 0
		AND NOT a.attisdropped
	ORDER BY c.relname, a.attnum`

	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var (
		tableName     string
		colName       string
		dataType      string
		isNotNull     bool
		colDefault    sql.NullString
		ordinalPos    int
		isIdentity    string
		collationName string
	)

	currentTable := ""
	for rows.Next() {
		err := rows.Scan(&tableName, &colName, &dataType, &isNotNull, &colDefault, &ordinalPos, &isIdentity, &collationName)
		if err != nil {
			return fmt.Errorf("scan table row: %w", err)
		}

		if tableName != currentTable {
			if _, exists := ns.Tables[tableName]; !exists {
				ns.Tables[tableName] = model.NewTable(schemaName, tableName)
			}
			currentTable = tableName
		}

		table := ns.Tables[tableName]
		col := &model.Column{
			Name:       colName,
			DataType:   dataType,
			IsNullable: !isNotNull,
			Collation:  collationName,
		}
		if colDefault.Valid {
			defaultStr := colDefault.String
			col.DefaultExpr = &defaultStr
		}
		if isIdentity == "a" || isIdentity == "d" {
			col.IsIdentity = true
			switch isIdentity {
			case "a":
				col.IdentityKind = "ALWAYS"
			case "d":
				col.IdentityKind = "BY DEFAULT"
			}
		}
		table.AddColumn(col)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table rows: %w", err)
	}
	return nil
}
