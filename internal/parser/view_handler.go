package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateViewHandler handles CREATE VIEW statements.
type CreateViewHandler struct{}

func (h *CreateViewHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetViewStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateViewHandler: expected ViewStmt, got %T", node)
	}
	name, schemaName := parserutil.ParseRelation(stmt.View)
	definition := strings.TrimSuffix(parserutil.DeparseNode(stmt.Query), ";")
	return []SchemaMutation{CreateViewMutation{
		Schema: schemaName,
		View:   model.View{Name: name, Definition: definition, Materialized: false},
	}}, nil
}

// CreateMaterializedViewHandler handles CREATE MATERIALIZED VIEW statements.
// In pg_query, CREATE MATERIALIZED VIEW is represented as a CreateTableAsStmt
// with Objtype == ObjectType_OBJECT_MATVIEW.
type CreateMaterializedViewHandler struct{}

func (h *CreateMaterializedViewHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateTableAsStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateMaterializedViewHandler: expected CreateTableAsStmt, got %T", node)
	}
	if stmt.Objtype != pg_query.ObjectType_OBJECT_MATVIEW {
		return nil, fmt.Errorf("CreateMaterializedViewHandler: expected OBJECT_MATVIEW, got %v", stmt.Objtype)
	}
	rel := stmt.Into.GetRel()
	name, schemaName := parserutil.ParseRelation(rel)
	definition := strings.TrimSuffix(parserutil.DeparseNode(stmt.Query), ";")
	return []SchemaMutation{CreateViewMutation{
		Schema: schemaName,
		View:   model.View{Name: name, Definition: definition, Materialized: true},
	}}, nil
}

// CreateViewMutation describes creating a view.
type CreateViewMutation struct {
	Schema string
	View   model.View
}

func (m CreateViewMutation) Kind() MutationKind {
	return MutationKind("create_view")
}

func (m CreateViewMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.View.Name, model.KindView)
}

func (m CreateViewMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	view := m.View
	ns.Views[view.Name] = &view
	return nil
}
