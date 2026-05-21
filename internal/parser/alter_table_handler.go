package parser

import (
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// AlterTableHandler handles ALTER TABLE statements.
type AlterTableHandler struct{}

// Handle converts a pg_query AlterTableStmt into schema mutations.
func (h *AlterTableHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetAlterTableStmt()
	if stmt == nil {
		return nil, fmt.Errorf("AlterTableHandler: expected AlterTableStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var mutations []SchemaMutation
	for _, item := range stmt.Cmds {
		cmd := item.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		switch cmd.Subtype {
		case pg_query.AlterTableType_AT_AddColumn:
			if cmd.Def != nil {
				if colDef := cmd.Def.GetColumnDef(); colDef != nil {
					col := parserutil.ParseColumnDef(colDef)
					mutations = append(mutations, AddColumnMutation{
						Schema: schemaName,
						Table:  tableName,
						Column: *col,
					})
				}
			}

		case pg_query.AlterTableType_AT_DropColumn:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP COLUMN missing column name\n")
				continue
			}
			mutations = append(mutations, DropColumnMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_AlterColumnType:
			colName := cmd.Name
			if colName == "" || cmd.Def == nil {
				fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN TYPE missing column name or type\n")
				continue
			}
			if colDef := cmd.Def.GetColumnDef(); colDef != nil {
				col := parserutil.ParseColumnDef(colDef)
				usingExpr := ""
				if colDef.RawDefault != nil {
					usingExpr = parserutil.FormatExpression(colDef.RawDefault)
				}
				mutations = append(mutations, AlterColumnTypeMutation{
					Schema:    schemaName,
					Table:     tableName,
					Column:    colName,
					ToType:    col.DataType,
					UsingExpr: usingExpr,
				})
			}

		case pg_query.AlterTableType_AT_SetNotNull:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: SET NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, SetNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_DropNotNull:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, DropNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_ColumnDefault:
			colName := cmd.Name
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

		case pg_query.AlterTableType_AT_AddConstraint:
			if cmd.Def == nil {
				continue
			}
			c := cmd.Def.GetConstraint()
			if c == nil {
				continue
			}
			constraint, ok := parseTableConstraint(tableName, c)
			if !ok {
				fmt.Fprintf(os.Stderr, "warning: unsupported ADD CONSTRAINT type: %v\n", c.Contype)
				continue
			}
			mutations = append(mutations, AddConstraintMutation{
				Schema: schemaName, Table: tableName, Constraint: constraint,
			})

		case pg_query.AlterTableType_AT_DropConstraint:
			if cmd.Name == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP CONSTRAINT missing constraint name\n")
				continue
			}
			mutations = append(mutations, DropConstraintMutation{
				Schema: schemaName, Table: tableName, Name: cmd.Name,
			})

		default:
			// NOTE: Some ALTER TABLE subcommands (e.g., RENAME COLUMN) are parsed by
			// pg_query_go as top-level RenameStmt nodes, not as AlterTableCmd subtypes.
			// If you encounter an unhandled subtype here, check whether the statement
			// type is handled by a dedicated handler (e.g., RenameStmtHandler) or if
			// a new handler needs to be registered in registry.go.
			fmt.Fprintf(os.Stderr, "warning: unsupported ALTER TABLE subcommand (subtype=%v). "+
				"This may be handled by a different top-level statement handler.\n", cmd.Subtype)
		}
	}
	return mutations, nil
}

// extractDefaultExpr extracts default expression from a node
func extractDefaultExpr(node *pg_query.Node) string {
	return fmt.Sprintf("%v", node)
}
