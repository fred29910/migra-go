package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

func (d *Differ) diffTables(source, target *model.Schema) {
	// Find tables that need to be added (in target but not in source)
	for name := range target.Tables {
		if _, exists := source.Tables[name]; !exists {
			op := model.NewDiffOp(model.AddTable, name)
			d.addOp(op)
		}
	}

	// Find tables that need to be dropped (in source but not in target)
	for name := range source.Tables {
		if _, exists := target.Tables[name]; !exists {
			op := model.NewDiffOp(model.DropTable, name)
			d.addOp(op)
		}
	}

	// Compare tables that exist in both schemas
	for name, targetTable := range target.Tables {
		if sourceTable, exists := source.Tables[name]; exists {
			d.diffColumns(sourceTable, targetTable)
			d.diffConstraints(sourceTable, targetTable)
			d.diffIndexes(sourceTable, targetTable)
		}
	}
}
