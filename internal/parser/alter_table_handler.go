package parser

import (
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// AlterTableHandler handles ALTER TABLE statements.
type AlterTableHandler struct{}

// Handle converts a pg_query AlterTableStmt into schema mutations.
func (h *AlterTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt := node.(pg_nodes.AlterTableStmt)
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
		// MVP: Skip other alter subcommands
		default:
			// Collect warnings or skip silently
		}
	}

	return mutations, nil
}
