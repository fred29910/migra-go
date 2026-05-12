package parser

import (
	"fmt"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateEnumHandler handles CREATE TYPE ... AS ENUM statements.
type CreateEnumHandler struct{}

// Handle converts a pg_query CreateEnumStmt into schema mutations.
func (h *CreateEnumHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.CreateEnumStmt)
	if !ok {
		return nil, fmt.Errorf("CreateEnumHandler: expected pg_nodes.CreateEnumStmt, got %T", node)
	}
	schemaName := "public"
	typeName := ""
	if len(stmt.TypeName.Items) > 0 {
		parts := make([]string, 0, len(stmt.TypeName.Items))
		for _, item := range stmt.TypeName.Items {
			if s, ok := item.(pg_nodes.String); ok {
				parts = append(parts, s.Str)
			}
		}
		if len(parts) == 1 {
			typeName = parts[0]
		} else if len(parts) >= 2 {
			schemaName = parts[len(parts)-2]
			typeName = parts[len(parts)-1]
		}
	}
	if typeName == "" {
		return nil, fmt.Errorf("CreateEnumHandler: unable to extract type name from CreateEnumStmt")
	}
	labels := make([]string, 0, len(stmt.Vals.Items))
	for _, item := range stmt.Vals.Items {
		if s, ok := item.(pg_nodes.String); ok {
			labels = append(labels, s.Str)
		}
	}
	return []SchemaMutation{CreateEnumTypeMutation{Schema: schemaName, Name: typeName, Labels: labels}}, nil
}
