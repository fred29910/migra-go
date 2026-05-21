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

	// Check for collation change
	if source.Collation != target.Collation {
		dataType := target.DataType
		if dataType == "" {
			dataType = source.DataType
		}
		c.addOp(NewAlterColumnCollationOp(schema, table, source.Name, dataType, source.Collation, target.Collation))
	}

	// Check for identity change
	if source.IsIdentity != target.IsIdentity || source.IdentityKind != target.IdentityKind {
		switch {
		case !source.IsIdentity && target.IsIdentity:
			c.addOp(NewAddIdentityOp(schema, table, source.Name, target.IdentityKind))
		case source.IsIdentity && target.IsIdentity:
			c.addOp(NewSetIdentityOp(schema, table, source.Name, target.IdentityKind))
		case source.IsIdentity && !target.IsIdentity:
			c.addOp(NewDropIdentityOp(schema, table, source.Name))
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
