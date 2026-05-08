package parser

import (
	"fmt"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateEnumHandler handles CREATE TYPE ... AS ENUM statements.
type CreateEnumHandler struct{}

// Handle converts a pg_query CreateEnumStmt into schema mutations.
// Currently returns an error as enum parsing is not yet fully implemented.
func (h *CreateEnumHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	_, ok := node.(pg_nodes.CreateEnumStmt)
	if !ok {
		return nil, fmt.Errorf("CreateEnumHandler: expected pg_nodes.CreateEnumStmt, got %T", node)
	}
	return nil, fmt.Errorf("CREATE TYPE ENUM is not yet supported (MVP scope only includes CREATE TABLE and ALTER TABLE ADD COLUMN)")
}
