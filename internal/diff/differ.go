package diff

import (
	"sort"

	"github.com/fred29910/migra-go/internal/model"
)

// Engine defines the interface for schema diff computation.
type Engine interface {
	Diff(source, target *model.Schema) []Operation
	Warnings() []string
}

// Compile-time check: Differ must satisfy Engine.
var _ Engine = (*Differ)(nil)

// Differ performs diff between two schemas
type Differ struct {
	lastWarnings []string
}

// NewDiffer creates a new Differ
func NewDiffer() *Differ {
	return &Differ{
		lastWarnings: make([]string, 0),
	}
}

func (d *Differ) Warnings() []string {
	out := make([]string, len(d.lastWarnings))
	copy(out, d.lastWarnings)
	return out
}

// Diff compares two schemas and returns a list of operations
func (d *Differ) Diff(source, target *model.Schema) []Operation {
	ctx := &diffContext{ops: make([]Operation, 0, 16), warnings: make([]string, 0, 4)}
	ctx.diffSchemas(source, target)
	d.lastWarnings = append(d.lastWarnings[:0], ctx.warnings...)
	return ctx.ops
}

// diffSchemas compares namespaces in two schemas
func (c *diffContext) diffSchemas(source, target *model.Schema) {
	// Check all namespaces in target (sorted for deterministic output)
	targetNames := make([]string, 0, len(target.Schemas))
	for name := range target.Schemas {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, name := range targetNames {
		targetNs := target.Schemas[name]
		if sourceNs, exists := source.Schemas[name]; exists {
			c.diffNamespace(sourceNs, targetNs)
		} else {
			// Entire namespace needs to be created
			c.diffNamespace(nil, targetNs)
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
			c.warnf("namespace drop is not implemented yet (ignored): %s", name)
		}
	}
}

// diffNamespace compares two namespaces
func (c *diffContext) diffNamespace(source, target *model.Namespace) {
	if target == nil {
		return
	}

	// Compare tables
	c.diffTables(source, target)

	// Compare types (enums)
	c.diffTypes(source, target)
}

// diffTypes compares enum types between two namespaces
func (c *diffContext) diffTypes(source, target *model.Namespace) {
	if source == nil {
		// All types in target are new
		targetTypeNames := make([]string, 0, len(target.Types))
		for name := range target.Types {
			targetTypeNames = append(targetTypeNames, name)
		}
		sort.Strings(targetTypeNames)
		for _, name := range targetTypeNames {
			c.addOp(&AddEnumTypeOp{
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
			c.addOp(&AddEnumTypeOp{
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
			c.addOp(&DropEnumTypeOp{
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
