package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

func (d *Differ) diffIndexes(sourceTable, targetTable *model.Table) {
	// Find indexes to add
	for name, index := range targetTable.Indexes {
		if _, exists := sourceTable.Indexes[name]; !exists {
			op := model.NewDiffOp(model.AddIndex, targetTable.Name+"."+name)
			op.Details["columns"] = joinStrings(index.Columns)
			op.Details["unique"] = fmtBool(index.Unique)
			op.Details["method"] = index.Method
			d.addOp(op)
		}
	}

	// Find indexes to drop
	for name := range sourceTable.Indexes {
		if _, exists := targetTable.Indexes[name]; !exists {
			op := model.NewDiffOp(model.DropIndex, sourceTable.Name+"."+name)
			d.addOp(op)
		}
	}
}
