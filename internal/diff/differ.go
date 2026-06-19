package diff

import (
	"context"
	"sort"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// Engine defines the interface for schema diff computation.
type Engine interface {
	Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error)
}

// Compile-time check: Differ must satisfy Engine.
var _ Engine = (*Differ)(nil)

// Differ performs diff between two schemas.
// Note: Differ is not safe for concurrent use.
type Differ struct{}

// NewDiffer creates a new Differ.
func NewDiffer() *Differ {
	return &Differ{}
}

// Diff compares two schemas and returns operations, warnings, and error.
func (d *Differ) Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error) {
	c := newDiffContext(ctx)
	c.diffSchemas(source, target)
	if c.cancelErr != nil {
		return c.ops, c.warnings, c.cancelErr
	}
	return c.ops, c.warnings, nil
}

// diffSchemas compares namespaces in two schemas.
func (c *diffContext) diffSchemas(source, target *model.Schema) {
	// Check all namespaces in target (sorted for deterministic output)
	targetNames := make([]string, 0, len(target.Schemas))
	for name := range target.Schemas {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, name := range targetNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		targetNs := target.Schemas[name]
		if sourceNs, exists := source.Schemas[name]; exists {
			c.diffNamespace(sourceNs, targetNs)
		} else {
			// Entire namespace needs to be created
			c.addOp(NewCreateSchemaOp(name))
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
		if err := c.checkCancelled(); err != nil {
			return
		}
		if _, exists := target.Schemas[name]; !exists {
			// The public schema is always present in PostgreSQL databases
			// and should never be dropped in normal migrations.
			if name == "public" {
				c.warnf("dropping 'public' schema is not allowed")
				continue
			}
			c.addOp(NewDropSchemaOp(name))
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

	// Compare P3 objects
	c.diffViews(source, target)
	c.diffSequences(source, target)
	c.diffExtensions(source, target)
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
			if err := c.checkCancelled(); err != nil {
				return
			}
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
		if err := c.checkCancelled(); err != nil {
			return
		}
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

	// Find types that exist in both
	for _, name := range targetTypeNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if sourceType, exists := source.Types[name]; exists {
			c.diffEnumType(target.Name, name, sourceType, target.Types[name])
		}
	}

	// Find types to drop
	sourceTypeNames := make([]string, 0, len(source.Types))
	for name := range source.Types {
		sourceTypeNames = append(sourceTypeNames, name)
	}
	sort.Strings(sourceTypeNames)
	for _, name := range sourceTypeNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
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

func (c *diffContext) diffEnumType(schema, name string, sourceType, targetType *model.EnumType) {
	if isEnumAppend(sourceType.Labels, targetType.Labels) {
		for _, label := range targetType.Labels[len(sourceType.Labels):] {
			c.addOp(NewAddEnumLabelOp(schema, name, label))
		}
	} else if !util.SameStringSlice(sourceType.Labels, targetType.Labels) {
		c.warnf("enum %s.%s change is not append-only and is not implemented", schema, name)
	}
}

func isEnumAppend(sourceLabels, targetLabels []string) bool {
	if len(targetLabels) <= len(sourceLabels) {
		return false
	}
	for i, label := range sourceLabels {
		if targetLabels[i] != label {
			return false
		}
	}
	return true
}

// diffViews compares views between two namespaces
func (c *diffContext) diffViews(source, target *model.Namespace) {
	if source == nil {
		names := make([]string, 0, len(target.Views))
		for name := range target.Views {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := c.checkCancelled(); err != nil {
				return
			}
			view := target.Views[name]
			if view.Materialized {
				c.addOp(NewCreateMaterializedViewOp(target.Name, view))
			} else {
				c.addOp(NewCreateViewOp(target.Name, view))
			}
		}
		return
	}

	// Find views to add or replace
	targetNames := make([]string, 0, len(target.Views))
	for name := range target.Views {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		tgtView := target.Views[name]
		if src, ok := source.Views[name]; !ok {
			if tgtView.Materialized {
				c.addOp(NewCreateMaterializedViewOp(target.Name, tgtView))
			} else {
				c.addOp(NewCreateViewOp(target.Name, tgtView))
			}
		} else if src.Materialized != tgtView.Materialized {
			// View type changed (regular <-> materialized): drop old, create new
			if src.Materialized {
				c.addOp(NewDropMaterializedViewOp(source.Name, name))
			} else {
				drop := NewDropViewOp(source.Name, name)
				drop.IsRecreate = true
				c.addOp(drop)
			}
			if tgtView.Materialized {
				c.addOp(NewCreateMaterializedViewOp(target.Name, tgtView))
			} else {
				c.addOp(NewCreateViewOp(target.Name, tgtView))
			}
		} else if src.Definition != tgtView.Definition {
			if tgtView.Materialized {
				// Materialized views don't support OR REPLACE; drop + recreate
				c.addOp(NewDropMaterializedViewOp(source.Name, name))
				c.addOp(NewCreateMaterializedViewOp(target.Name, tgtView))
			} else {
				c.addOp(NewReplaceViewOp(target.Name, tgtView))
			}
		}
	}

	// Find views to drop
	sourceNames := make([]string, 0, len(source.Views))
	for name := range source.Views {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if _, ok := target.Views[name]; !ok {
			if source.Views[name].Materialized {
				c.addOp(NewDropMaterializedViewOp(source.Name, name))
			} else {
				c.addOp(NewDropViewOp(source.Name, name))
			}
		}
	}
}

// diffSequences compares sequences between two namespaces
func (c *diffContext) diffSequences(source, target *model.Namespace) {
	if source == nil {
		names := make([]string, 0, len(target.Sequences))
		for name := range target.Sequences {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := c.checkCancelled(); err != nil {
				return
			}
			c.addOp(NewCreateSequenceOp(target.Name, target.Sequences[name]))
		}
		return
	}

	// Find sequences to add or alter
	targetNames := make([]string, 0, len(target.Sequences))
	for name := range target.Sequences {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if src, ok := source.Sequences[name]; !ok {
			c.addOp(NewCreateSequenceOp(target.Name, target.Sequences[name]))
		} else if !sameSequenceContent(src, target.Sequences[name]) {
			c.addOp(NewAlterSequenceOp(target.Name, src, target.Sequences[name]))
		}
	}

	// Find sequences to drop
	sourceNames := make([]string, 0, len(source.Sequences))
	for name := range source.Sequences {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if _, ok := target.Sequences[name]; !ok {
			c.addOp(NewDropSequenceOp(source.Name, name))
		}
	}
}

// sameSequenceContent checks if two sequences have the same content
func sameSequenceContent(a, b *model.Sequence) bool {
	return a.DataType == b.DataType &&
		a.StartValue == b.StartValue &&
		a.IncrementBy == b.IncrementBy &&
		a.MinValue == b.MinValue &&
		a.MaxValue == b.MaxValue &&
		a.CacheSize == b.CacheSize &&
		a.Cycle == b.Cycle &&
		a.OwnedByTable == b.OwnedByTable &&
		a.OwnedByColumn == b.OwnedByColumn
}

// diffExtensions compares extensions between two namespaces
func (c *diffContext) diffExtensions(source, target *model.Namespace) {
	if source == nil {
		names := make([]string, 0, len(target.Extensions))
		for name := range target.Extensions {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := c.checkCancelled(); err != nil {
				return
			}
			c.addOp(NewCreateExtensionOp(target.Name, target.Extensions[name]))
		}
		return
	}

	// Find extensions to add or update
	targetNames := make([]string, 0, len(target.Extensions))
	for name := range target.Extensions {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if src, ok := source.Extensions[name]; !ok {
			c.addOp(NewCreateExtensionOp(target.Name, target.Extensions[name]))
		} else if src.Version != "" && target.Extensions[name].Version != "" && src.Version != target.Extensions[name].Version {
			c.addOp(NewAlterExtensionUpdateOp(target.Name, target.Extensions[name]))
		}
	}

	// Find extensions to drop
	sourceNames := make([]string, 0, len(source.Extensions))
	for name := range source.Extensions {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if _, ok := target.Extensions[name]; !ok {
			c.addOp(NewDropExtensionOp(source.Name, name))
		}
	}
}
