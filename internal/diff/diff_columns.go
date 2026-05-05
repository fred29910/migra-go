package diff

import (
	"github.com/fred29910/migra-go/internal/model"
)

// diffColumn compares two columns and generates operations for differences
func (c *diffContext) diffColumn(schema, table string, source, target *model.Column) {
	// Check for data type change
	if source.DataType != target.DataType {
		c.addOp(NewAlterColumnTypeOp(schema, table, source.Name, source.DataType, target.DataType))
	}

	// Check for nullable change
	if source.IsNullable && !target.IsNullable {
		// Column changed from nullable to not nullable
		c.addOp(NewSetNotNullOp(schema, table, source.Name))
	} else if !source.IsNullable && target.IsNullable {
		// Column changed from not nullable to nullable
		c.addOp(NewDropNotNullOp(schema, table, source.Name))
	}

	// Check for default expression change
	if !sameDefault(source.DefaultExpr, target.DefaultExpr) {
		// MVP: handle default changes as part of alter column
		// TODO: implement SetDefaultOp and DropDefaultOp
		_ = source.DefaultExpr
		_ = target.DefaultExpr
		c.warnf("column %s.%s.%s default change is not implemented yet (ignored)", schema, table, source.Name)
	}
}

// sameDefault checks if two default expressions are the same
func sameDefault(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
