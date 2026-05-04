package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

func (d *Differ) diffColumns(sourceTable, targetTable *model.Table) {
	// Find columns to add (in target but not in source)
	for name, col := range targetTable.Columns {
		if _, exists := sourceTable.Columns[name]; !exists {
			op := model.NewDiffOp(model.AddColumn, targetTable.Name+"."+name)
			op.Details["data_type"] = col.DataType
			op.Details["nullable"] = fmtBool(col.Nullable)
			if col.Default != nil {
				op.Details["default"] = *col.Default
			}
			d.addOp(op)
		}
	}

	// Find columns to drop (in source but not in target)
	for name := range sourceTable.Columns {
		if _, exists := targetTable.Columns[name]; !exists {
			op := model.NewDiffOp(model.DropColumn, sourceTable.Name+"."+name)
			d.addOp(op)
		}
	}

	// Compare columns that exist in both
	for name, targetCol := range targetTable.Columns {
		if sourceCol, exists := sourceTable.Columns[name]; exists {
			d.diffColumn(sourceTable.Name, sourceCol, targetCol)
		}
	}
}

func (d *Differ) diffColumn(tableName string, source, target *model.Column) {
	// Check for type changes
	if source.DataType != target.DataType {
		op := model.NewDiffOp(model.AlterColumn, tableName+"."+source.Name)
		op.Details["change"] = "type"
		op.Details["old_type"] = source.DataType
		op.Details["new_type"] = target.DataType
		d.addOp(op)
	}

	// Check for nullable changes
	if source.Nullable != target.Nullable {
		op := model.NewDiffOp(model.AlterColumn, tableName+"."+source.Name)
		op.Details["change"] = "nullable"
		op.Details["old_nullable"] = fmtBool(source.Nullable)
		op.Details["new_nullable"] = fmtBool(target.Nullable)
		d.addOp(op)
	}

	// Check for default changes
	if !sameDefault(source.Default, target.Default) {
		op := model.NewDiffOp(model.AlterColumn, tableName+"."+source.Name)
		op.Details["change"] = "default"
		if source.Default != nil {
			op.Details["old_default"] = *source.Default
		}
		if target.Default != nil {
			op.Details["new_default"] = *target.Default
		}
		d.addOp(op)
	}
}

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func sameDefault(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
