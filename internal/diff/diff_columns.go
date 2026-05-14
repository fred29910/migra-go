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
		if target.DefaultExpr == nil {
			c.addOp(NewDropDefaultOp(schema, table, source.Name))
		} else {
			c.addOp(NewSetDefaultOp(schema, table, source.Name, *target.DefaultExpr))
		}
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
