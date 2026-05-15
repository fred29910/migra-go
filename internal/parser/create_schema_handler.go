package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateSchemaHandler handles CREATE SCHEMA statements.
// Note: The SchemaElts field (child statements like CREATE TABLE embedded in
// CREATE SCHEMA) is intentionally not processed here, because pg_query_go
// already returns them as separate top-level statements in the AST.
type CreateSchemaHandler struct{}

// Handle converts a pg_query CreateSchemaStmt into schema mutations.
func (h *CreateSchemaHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.CreateSchemaStmt)
	if !ok {
		return nil, fmt.Errorf("CreateSchemaHandler: expected pg_nodes.CreateSchemaStmt, got %T", node)
	}
	if stmt.Schemaname == nil {
		return nil, fmt.Errorf("CreateSchemaHandler: schema name is nil")
	}
	schemaName := *stmt.Schemaname
	return []SchemaMutation{CreateSchemaMutation{Schema: schemaName}}, nil
}

// CreateSchemaMutation describes creating a namespace/schema.
type CreateSchemaMutation struct {
	Schema string
}

func (m CreateSchemaMutation) Kind() MutationKind { return MutKindCreateSchema }
func (m CreateSchemaMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, "", model.KindSchema)
}
func (m CreateSchemaMutation) Apply(schema *model.Schema) error {
	schema.GetOrCreateNamespace(m.Schema)
	return nil
}
