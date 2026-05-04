package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

func (d *Differ) diffConstraints(sourceTable, targetTable *model.Table) {
	// Find constraints to add
	for name, constraint := range targetTable.Constraints {
		if _, exists := sourceTable.Constraints[name]; !exists {
			op := model.NewDiffOp(model.AddConstraint, targetTable.Name+"."+name)
			op.Details["type"] = constraint.Type
			op.Details["definition"] = constraint.Definition
			d.addOp(op)
		}
	}

	// Find constraints to drop
	for name := range sourceTable.Constraints {
		if _, exists := targetTable.Constraints[name]; !exists {
			op := model.NewDiffOp(model.DropConstraint, sourceTable.Name+"."+name)
			d.addOp(op)
		}
	}
}
