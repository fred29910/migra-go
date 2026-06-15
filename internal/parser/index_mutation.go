package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// CreateIndexMutation describes creating an index.
type CreateIndexMutation struct {
	Schema string
	Index  model.Index
}

// Kind returns the mutation kind.
func (m CreateIndexMutation) Kind() MutationKind { return MutKindCreateIndex }

// Target returns the ObjectKey of the index to be created.
func (m CreateIndexMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Index.Name, model.KindIndex)
}

// Apply adds the index to the target table in the given schema.
func (m CreateIndexMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	table, exists := ns.Tables[m.Index.Table]
	if !exists {
		return fmt.Errorf("table %s.%s does not exist for index creation", m.Schema, m.Index.Table)
	}

	// Check IfNotExists
	if m.Index.IfNotExists {
		if _, exists := table.Indexes[m.Index.Name]; exists {
			return nil // skip silently
		}
	}

	// Check duplicate
	if _, exists := table.Indexes[m.Index.Name]; exists {
		return fmt.Errorf("index %s already exists on table %s.%s", m.Index.Name, m.Schema, m.Index.Table)
	}

	// Set PrimaryKey if needed
	if m.Index.Primary {
		columns := make([]string, 0, len(m.Index.Elements))
		for _, elem := range m.Index.Elements {
			columns = append(columns, elem.Name)
		}
		table.PrimaryKey = &model.PrimaryKey{
			Name:    m.Index.Name,
			Columns: columns,
		}
	}

	// Add index
	table.Indexes[m.Index.Name] = &m.Index

	return nil
}
