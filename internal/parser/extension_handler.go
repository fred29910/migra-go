package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateExtensionHandler handles CREATE EXTENSION statements.
type CreateExtensionHandler struct{}

func (h *CreateExtensionHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateExtensionStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateExtensionHandler: expected CreateExtensionStmt, got %T", node)
	}
	ext := model.Extension{Name: stmt.Extname}
	schemaName := "public"
	for _, item := range stmt.Options {
		def := item.GetDefElem()
		if def == nil {
			continue
		}
		switch strings.ToLower(def.Defname) {
		case "schema":
			if s := def.Arg.GetString_(); s != nil {
				schemaName = s.Sval
			}
		case "new_version":
			if s := def.Arg.GetString_(); s != nil {
				ext.Version = s.Sval
			}
		}
	}
	return []SchemaMutation{CreateExtensionMutation{Schema: schemaName, Extension: ext}}, nil
}

// CreateExtensionMutation describes creating an extension.
type CreateExtensionMutation struct {
	Schema    string
	Extension model.Extension
}

func (m CreateExtensionMutation) Kind() MutationKind {
	return MutationKind("create_extension")
}

func (m CreateExtensionMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Extension.Name, model.KindExtension)
}

func (m CreateExtensionMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	ext := m.Extension
	ns.Extensions[ext.Name] = &ext
	return nil
}
