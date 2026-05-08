package parser

import (
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateTableHandler handles CREATE TABLE statements.
type CreateTableHandler struct{}

// Handle converts a pg_query CreateStmt into schema mutations.
func (h *CreateTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt := node.(pg_nodes.CreateStmt)
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	for _, item := range stmt.TableElts.Items {
		switch elt := item.(type) {
		case pg_nodes.ColumnDef:
			columns = append(columns, *parserutil.ParseColumnDef(elt))
			// MVP: skip constraints
		}
	}

	return []SchemaMutation{
		CreateTableMutation{
			Schema:  schemaName,
			Name:    tableName,
			Columns: columns,
		},
	}, nil
}
