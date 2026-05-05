package diff

import (
	"fmt"
	"sort"

	"github.com/fred29910/migra-go/internal/model"
)

// Differ performs diff between two schemas
type Differ struct {
	ops      []Operation
	warnings []string
}

// NewDiffer creates a new Differ
func NewDiffer() *Differ {
	return &Differ{
		ops:      make([]Operation, 0),
		warnings: make([]string, 0),
	}
}

func (d *Differ) warnf(format string, args ...any) {
	d.warnings = append(d.warnings, fmt.Sprintf(format, args...))
}

func (d *Differ) Warnings() []string {
	out := make([]string, len(d.warnings))
	copy(out, d.warnings)
	return out
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
	// Check all namespaces in target (sorted for deterministic output)
	targetNames := make([]string, 0, len(target.Schemas))
	for name := range target.Schemas {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, name := range targetNames {
		targetNs := target.Schemas[name]
		if sourceNs, exists := source.Schemas[name]; exists {
			d.diffNamespace(sourceNs, targetNs)
		} else {
			// Entire namespace needs to be created
			d.diffNamespace(nil, targetNs)
		}
	}

	// Check for namespaces that exist in source but not in target (sorted for deterministic output)
	sourceNames := make([]string, 0, len(source.Schemas))
	for name := range source.Schemas {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	for _, name := range sourceNames {
		if _, exists := target.Schemas[name]; !exists {
			// Namespace dropped - MVP: skip or handle explicitly
			_ = source.Schemas[name]
			d.warnf("namespace drop is not implemented yet (ignored): %s", name)
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
		targetTypeNames := make([]string, 0, len(target.Types))
		for name := range target.Types {
			targetTypeNames = append(targetTypeNames, name)
		}
		sort.Strings(targetTypeNames)
		for _, name := range targetTypeNames {
			d.addOp(&AddEnumTypeOp{
				baseOperation: baseOperation{
					kind:      KindAddEnumType,
					objectKey: model.NewObjectKey(target.Name, name, model.KindType),
				},
				Schema: target.Name,
				Type:   target.Types[name],
			})
		}
		return
	}

	// Find types to add
	targetTypeNames := make([]string, 0, len(target.Types))
	for name := range target.Types {
		targetTypeNames = append(targetTypeNames, name)
	}
	sort.Strings(targetTypeNames)
	for _, name := range targetTypeNames {
		if _, exists := source.Types[name]; !exists {
			d.addOp(&AddEnumTypeOp{
				baseOperation: baseOperation{
					kind:      KindAddEnumType,
					objectKey: model.NewObjectKey(target.Name, name, model.KindType),
				},
				Schema: target.Name,
				Type:   target.Types[name],
			})
		}
	}

	// Find types to drop
	sourceTypeNames := make([]string, 0, len(source.Types))
	for name := range source.Types {
		sourceTypeNames = append(sourceTypeNames, name)
	}
	sort.Strings(sourceTypeNames)
	for _, name := range sourceTypeNames {
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
