package parser

import (
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// AlterTableHandler handles ALTER TABLE statements.
type AlterTableHandler struct{}

// Handle converts a pg_query AlterTableStmt into schema mutations.
func (h *AlterTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.AlterTableStmt)
	if !ok {
		return nil, fmt.Errorf("AlterTableHandler: expected pg_nodes.AlterTableStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var mutations []SchemaMutation
	for _, item := range stmt.Cmds.Items {
		cmd, ok := item.(pg_nodes.AlterTableCmd)
		if !ok {
			continue
		}
		switch cmd.Subtype {
		case pg_nodes.AT_AddColumn:
			if cmd.Def != nil {
				if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
					col := parserutil.ParseColumnDef(colDef)
					mutations = append(mutations, AddColumnMutation{
						Schema: schemaName,
						Table:  tableName,
						Column: *col,
					})
				}
			}

		case pg_nodes.AT_DropColumn:
			colName := ""
			if cmd.Name != nil {
				colName = *cmd.Name
			}
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP COLUMN missing column name\n")
				continue
			}
			mutations = append(mutations, DropColumnMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_nodes.AT_AlterColumnType:
			colName := ""
			if cmd.Name != nil {
				colName = *cmd.Name
			}
			if colName == "" || cmd.Def == nil {
				fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN TYPE missing column name or type\n")
				continue
			}
			if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
				col := parserutil.ParseColumnDef(colDef)
				mutations = append(mutations, AlterColumnTypeMutation{
					Schema: schemaName,
					Table:  tableName,
					Column: colName,
					ToType: col.DataType,
				})
			}

		case pg_nodes.AT_SetNotNull:
			colName := ""
			if cmd.Name != nil {
				colName = *cmd.Name
			}
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: SET NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, SetNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_nodes.AT_DropNotNull:
			colName := ""
			if cmd.Name != nil {
				colName = *cmd.Name
			}
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, DropNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_nodes.AT_ColumnDefault:
			colName := ""
			if cmd.Name != nil {
				colName = *cmd.Name
			}
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN DEFAULT missing column name\n")
				continue
			}
			if cmd.Def != nil {
				// SET DEFAULT
				defaultExpr := extractDefaultExpr(cmd.Def)
				mutations = append(mutations, SetDefaultMutation{
					Schema:      schemaName,
					Table:       tableName,
					Column:      colName,
					DefaultExpr: defaultExpr,
				})
			} else {
				// DROP DEFAULT
				mutations = append(mutations, DropDefaultMutation{
					Schema: schemaName,
					Table:  tableName,
					Column: colName,
				})
			}

		default:
			fmt.Fprintf(os.Stderr, "warning: unsupported ALTER TABLE subcommand: %v\n", cmd.Subtype)
		}
	}
	return mutations, nil
}

// extractDefaultExpr extracts default expression from a node
func extractDefaultExpr(node pg_nodes.Node) string {
	if d, ok := node.(interface{ Deparse() string }); ok {
		var result string
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = fmt.Sprintf("%v", node)
				}
			}()
			result = d.Deparse()
		}()
		return result
	}
	return fmt.Sprintf("%v", node)
}
