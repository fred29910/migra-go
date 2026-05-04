package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

// Differ performs diff between two schemas
type Differ struct {
	ops []*model.DiffOp
}

// NewDiffer creates a new Differ
func NewDiffer() *Differ {
	return &Differ{
		ops: make([]*model.DiffOp, 0),
	}
}

// Diff compares two schemas and returns a list of diff operations
func (d *Differ) Diff(source, target *model.Schema) []*model.DiffOp {
	d.ops = make([]*model.DiffOp, 0)

	// Compare tables
	d.diffTables(source, target)

	// Compare types (enums)
	d.diffTypes(source, target)

	// Compare views
	d.diffViews(source, target)

	// Compare functions
	d.diffFuncs(source, target)

	return d.ops
}

// addOp adds a new diff operation
func (d *Differ) addOp(op *model.DiffOp) {
	d.ops = append(d.ops, op)
}

func (d *Differ) diffTypes(source, target *model.Schema) {
	// Find types to add
	for name, enumType := range target.Types {
		if _, exists := source.Types[name]; !exists {
			op := model.NewDiffOp(model.AlterType, name)
			op.Details["action"] = "add"
			op.Details["labels"] = joinStrings(enumType.Labels)
			d.addOp(op)
		}
	}

	// Find types to drop
	for name := range source.Types {
		if _, exists := target.Types[name]; !exists {
			op := model.NewDiffOp(model.AlterType, name)
			op.Details["action"] = "drop"
			d.addOp(op)
		}
	}
}

func (d *Differ) diffViews(source, target *model.Schema) {
	// Find views to add
	for name, view := range target.Views {
		if _, exists := source.Views[name]; !exists {
			op := model.NewDiffOp(model.AddTable, name) // TODO: use AddView when available
			op.Details["type"] = "view"
			op.Details["definition"] = view.Definition
			d.addOp(op)
		}
	}

	// Find views to drop
	for name := range source.Views {
		if _, exists := target.Views[name]; !exists {
			op := model.NewDiffOp(model.DropTable, name) // TODO: use DropView when available
			op.Details["type"] = "view"
			d.addOp(op)
		}
	}
}

func (d *Differ) diffFuncs(source, target *model.Schema) {
	// Find functions to add
	for name, fn := range target.Funcs {
		if _, exists := source.Funcs[name]; !exists {
			op := model.NewDiffOp(model.AddTable, name) // TODO: use AddFunction when available
			op.Details["type"] = "function"
			op.Details["definition"] = fn.Definition
			d.addOp(op)
		}
	}

	// Find functions to drop
	for name := range source.Funcs {
		if _, exists := target.Funcs[name]; !exists {
			op := model.NewDiffOp(model.DropTable, name) // TODO: use DropFunction when available
			op.Details["type"] = "function"
			d.addOp(op)
		}
	}
}

func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
