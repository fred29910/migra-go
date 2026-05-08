package parser

import (
	"fmt"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateIndexHandler handles CREATE INDEX statements.
type CreateIndexHandler struct{}

// Handle converts a pg_query IndexStmt into schema mutations.
// Currently returns an error as index parsing is not yet fully implemented.
func (h *CreateIndexHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	_, ok := node.(pg_nodes.IndexStmt)
	if !ok {
		return nil, fmt.Errorf("CreateIndexHandler: expected pg_nodes.IndexStmt, got %T", node)
	}
	return nil, fmt.Errorf("CREATE INDEX is not yet supported (MVP scope only includes CREATE TABLE and ALTER TABLE ADD COLUMN)")
}
