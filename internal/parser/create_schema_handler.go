package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateSchemaHandler handles CREATE SCHEMA statements.
// Note: The SchemaElts field (child statements like CREATE TABLE embedded in
// CREATE SCHEMA) is intentionally not processed here, because pg_query_go
// already returns them as separate top-level statements in the AST.
type CreateSchemaHandler struct{}

// Handle converts a pg_query CreateSchemaStmt into schema mutations.
func (h *CreateSchemaHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateSchemaStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateSchemaHandler: expected CreateSchemaStmt, got %T", node)
	}
	schemaName := stmt.Schemaname
	if schemaName == "" {
		return nil, fmt.Errorf("CreateSchemaHandler: schema name is empty")
	}
	return []SchemaMutation{CreateSchemaMutation{Schema: schemaName}}, nil
}

// CreateSchemaMutation describes creating a namespace/schema.
type CreateSchemaMutation struct {
	Schema string
}

// Kind returns the mutation kind.
func (m CreateSchemaMutation) Kind() MutationKind { return MutKindCreateSchema }
// Target returns the ObjectKey of the schema to be created.
func (m CreateSchemaMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, "", model.KindSchema)
}
// Apply creates the namespace in the given schema.
func (m CreateSchemaMutation) Apply(schema *model.Schema) error {
	schema.GetOrCreateNamespace(m.Schema)
	return nil
}
