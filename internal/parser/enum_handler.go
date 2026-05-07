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
	_ = node.(pg_nodes.CreateEnumStmt)
	return nil, fmt.Errorf("CREATE TYPE ENUM parsing not yet fully implemented")
}
