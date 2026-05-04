package introspect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/migra-go/migra-go/internal/model"
)

// loadTables loads tables and their columns from information_schema
func loadTables(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT 
		t.table_name,
		c.column_name,
		c.data_type,
		c.is_nullable,
		c.column_default,
		c.ordinal_position
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
		tableName   string
		colName     string
		dataType    string
		isNullable  string
		colDefault  sql.NullString
		ordinalPos  int
	)

	currentTable := ""
	for rows.Next() {
		err := rows.Scan(&tableName, &colName, &dataType, &isNullable, &colDefault, &ordinalPos)
		if err != nil {
			return fmt.Errorf("scan table row: %w", err)
		}

		// Create table if it's new
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
		}
		if colDefault.Valid {
			col.DefaultExpr = &colDefault.String
		}
		table.AddColumn(col)
	}

	return rows.Err()
}
