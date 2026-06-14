package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// AlterTableHandler handles ALTER TABLE statements.
type AlterTableHandler struct {
	warnFn WarningEmitter
}

// SetWarningEmitter sets the warning emitter for this handler.
func (h *AlterTableHandler) SetWarningEmitter(fn WarningEmitter) {
	h.warnFn = fn
}

func (h *AlterTableHandler) warnf(format string, args ...any) {
	if h.warnFn != nil {
		h.warnFn(format, args...)
	}
}

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
				h.warnf("warning: DROP COLUMN missing column name")
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
				h.warnf("warning: ALTER COLUMN TYPE missing column name or type")
				continue
			}
			if colDef := cmd.Def.GetColumnDef(); colDef != nil {
				col := parserutil.ParseColumnDef(colDef)
				// In pg_query_go's protobuf representation, the USING expression
				// for ALTER COLUMN TYPE (PostgreSQL internal: AlterTableCmd.transform)
				// is mapped into ColumnDef.RawDefault. The AlterTableCmd protobuf
				// has no separate transform field; this is the correct field to read.
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
				h.warnf("warning: SET NOT NULL missing column name")
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
				h.warnf("warning: DROP NOT NULL missing column name")
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
				h.warnf("warning: ALTER COLUMN DEFAULT missing column name")
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
				h.warnf("warning: unsupported ADD CONSTRAINT type: %v", c.Contype)
				continue
			}
			mutations = append(mutations, AddConstraintMutation{
				Schema: schemaName, Table: tableName, Constraint: constraint,
			})

		case pg_query.AlterTableType_AT_DropConstraint:
			if cmd.Name == "" {
				h.warnf("warning: DROP CONSTRAINT missing constraint name")
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
			h.warnf("warning: unsupported ALTER TABLE subcommand (subtype=%v). This may be handled by a different top-level statement handler.", cmd.Subtype)
		}
	}
	return mutations, nil
}

// extractDefaultExpr extracts default expression from a node
func extractDefaultExpr(node *pg_query.Node) string {
	return parserutil.DeparseNode(node)
}
