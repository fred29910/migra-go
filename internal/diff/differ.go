package diff

import (
	"github.com/fred29910/migra-go/internal/model"
)

// Differ performs diff between two schemas
type Differ struct {
	ops []Operation
}

// NewDiffer creates a new Differ
func NewDiffer() *Differ {
	return &Differ{
		ops: make([]Operation, 0),
	}
}

// Diff compares two schemas and returns a list of operations
// source: the source schema (e.g., from SQL files)
// target: the target schema (e.g., from database)
// Returns operations needed to transform source -> target
func (d *Differ) Diff(source, target *model.Schema) []Operation {
	d.ops = make([]Operation, 0)

	// Compare namespaces (schemas)
	d.diffSchemas(source, target)

	return d.ops
}

// diffSchemas compares namespaces in two schemas
func (d *Differ) diffSchemas(source, target *model.Schema) {
	// Check all namespaces in target
	for name, targetNs := range target.Schemas {
		if sourceNs, exists := source.Schemas[name]; exists {
			d.diffNamespace(sourceNs, targetNs)
		} else {
			// Entire namespace needs to be created
			d.diffNamespace(nil, targetNs)
		}
	}

	// Check for namespaces that exist in source but not in target
	for name, sourceNs := range source.Schemas {
		if _, exists := target.Schemas[name]; !exists {
			// Namespace dropped - MVP: skip or handle explicitly
			_ = sourceNs
		}
	}
}

// diffNamespace compares two namespaces
func (d *Differ) diffNamespace(source, target *model.Namespace) {
	if target == nil {
		return
	}

	// Compare tables
	d.diffTables(source, target)

	// Compare types (enums)
	d.diffTypes(source, target)
}

// diffTypes compares enum types between two namespaces
func (d *Differ) diffTypes(source, target *model.Namespace) {
	if source == nil {
		// All types in target are new
		for name, enumType := range target.Types {
			d.addOp(&AddEnumTypeOp{
				baseOperation: baseOperation{
					kind:      KindAddEnumType,
					objectKey: model.NewObjectKey(target.Name, name, model.KindType),
				},
				Schema: target.Name,
				Type:   enumType,
			})
		}
		return
	}

	// Find types to add
	for name, enumType := range target.Types {
		if _, exists := source.Types[name]; !exists {
			d.addOp(&AddEnumTypeOp{
				baseOperation: baseOperation{
					kind:      KindAddEnumType,
					objectKey: model.NewObjectKey(target.Name, name, model.KindType),
				},
				Schema: target.Name,
				Type:   enumType,
			})
		}
	}

	// Find types to drop
	for name := range source.Types {
		if _, exists := target.Types[name]; !exists {
			d.addOp(&DropEnumTypeOp{
				baseOperation: baseOperation{
					kind:      KindDropEnumType,
					objectKey: model.NewObjectKey(source.Name, name, model.KindType),
				},
				Schema: source.Name,
				Name:   name,
			})
		}
	}
}

// addOp adds a new operation
func (d *Differ) addOp(op Operation) {
	d.ops = append(d.ops, op)
}
