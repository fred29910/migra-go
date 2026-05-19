package introspect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

// loadTables loads tables and their columns from information_schema
func loadTables(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT 
		t.table_name,
		c.column_name,
		c.data_type,
		c.character_maximum_length,
		c.is_nullable,
		c.column_default,
		c.ordinal_position,
		c.is_identity,
		c.identity_generation,
		COALESCE(c.collation_name, '') AS collation_name
	FROM information_schema.tables t
	JOIN information_schema.columns c ON t.table_name = c.table_name AND t.table_schema = c.table_schema
	WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
	ORDER BY t.table_name, c.ordinal_position`

	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var (
		tableName          string
		colName            string
		dataType           string
		charMaxLen         sql.NullInt64
		isNullable         string
		colDefault         sql.NullString
		ordinalPos         int
		collationName      string
		isIdentity         string
		identityGeneration sql.NullString
	)

	currentTable := ""
	for rows.Next() {
		err := rows.Scan(&tableName, &colName, &dataType, &charMaxLen, &isNullable, &colDefault, &ordinalPos, &isIdentity, &identityGeneration, &collationName)
		if err != nil {
			return fmt.Errorf("scan table row: %w", err)
		}

		if charMaxLen.Valid {
			dataType = fmt.Sprintf("%s(%d)", dataType, charMaxLen.Int64)
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
			IsNullable: isNullable == "YES",
			Collation:  collationName,
		}
		if colDefault.Valid {
			defaultStr := colDefault.String
			col.DefaultExpr = &defaultStr
		}
		if isIdentity == "YES" {
			col.IsIdentity = true
			if identityGeneration.Valid {
				col.IdentityKind = identityGeneration.String
			}
		}
		table.AddColumn(col)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table rows: %w", err)
	}
	return nil
}
