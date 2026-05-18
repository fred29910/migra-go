package parser

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateEnumHandler handles CREATE TYPE ... AS ENUM statements.
type CreateEnumHandler struct{}

// Handle converts a pg_query CreateEnumStmt into schema mutations.
func (h *CreateEnumHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateEnumStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateEnumHandler: expected CreateEnumStmt, got %T", node)
	}
	schemaName := "public"
	typeName := ""
	if len(stmt.TypeName) > 0 {
		parts := make([]string, 0, len(stmt.TypeName))
		for _, item := range stmt.TypeName {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
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
	labels := make([]string, 0, len(stmt.Vals))
	for _, item := range stmt.Vals {
		if s := item.GetString_(); s != nil {
			labels = append(labels, s.Sval)
		}
	}
	return []SchemaMutation{CreateEnumTypeMutation{Schema: schemaName, Name: typeName, Labels: labels}}, nil
}
