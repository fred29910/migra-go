package normalize

import (
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// CanonicalizeSchema normalizes a schema to reduce false diffs
// Performs in-place normalization of the schema.
func CanonicalizeSchema(s *model.Schema) error {
	for _, ns := range s.Schemas {
		canonicalizeNamespaceInPlace(ns)
	}
	return nil
}

func canonicalizeNamespaceInPlace(ns *model.Namespace) {
	// Normalize tables
	for _, table := range ns.Tables {
		canonicalizeTableInPlace(table)
	}

	// Normalize types
	for _, enumType := range ns.Types {
		canonicalizeEnumTypeInPlace(enumType)
	}
}

func canonicalizeTableInPlace(table *model.Table) {
	// ColumnByName might change if names are normalized
	newColumnByName := make(map[string]*model.Column)

	// Normalize columns
	for _, col := range table.Columns {
		canonicalizeColumnInPlace(col)
		newColumnByName[col.Name] = col
	}
	table.ColumnByName = newColumnByName

	// Normalize constraints
	for name, constraint := range table.Constraints {
		canonicalizeConstraintInPlace(constraint)
		// Assuming constraint name doesn't change enough to break the map keys for now.
		// If it did, we'd need to rebuild the map like ColumnByName.
		_ = name
	}

	// Normalize indexes
	for name, index := range table.Indexes {
		canonicalizeIndexInPlace(index)
		_ = name
	}
}

func canonicalizeColumnInPlace(col *model.Column) {
	col.Name = normalizeIdentifier(col.Name)
	col.DataType = normalizeDataType(col.DataType)
	if col.DefaultExpr != nil {
		normalized := normalizeDefaultExpr(*col.DefaultExpr)
		col.DefaultExpr = &normalized
	}
}

func canonicalizeEnumTypeInPlace(enumType *model.EnumType) {
	enumType.Name = normalizeIdentifier(enumType.Name)
}

func canonicalizeConstraintInPlace(c *model.Constraint) {
	c.Name = normalizeIdentifier(c.Name)
	c.Definition = normalizeConstraintDef(c.Definition)
}

func canonicalizeIndexInPlace(idx *model.Index) {
	idx.Name = normalizeIdentifier(idx.Name)
}

// typeAliases maps type aliases to canonical names
var typeAliases = map[string]string{
	"int4":                        "integer",
	"int8":                        "bigint",
	"int2":                        "smallint",
	"bool":                        "boolean",
	"character varying":           "varchar",
	"timestamp without time zone": "timestamp",
	"timestamp with time zone":    "timestamptz",
}

// normalizeDataType normalizes type aliases to canonical names
func normalizeDataType(dt string) string {
	dt = strings.ToLower(strings.TrimSpace(dt))

	if canonical, ok := typeAliases[dt]; ok {
		return canonical
	}
	return dt
}

// normalizeIdentifier normalizes quoted/unquoted identifiers
func normalizeIdentifier(id string) string {
	// Remove unnecessary quotes and lowercase
	id = strings.TrimSpace(id)
	if len(id) >= 2 && id[0] == '"' && id[len(id)-1] == '"' {
		// Quoted identifier - remove quotes and keep as-is (case-sensitive)
		return id[1 : len(id)-1]
	}
	// Unquoted identifier - lowercase
	return strings.ToLower(id)
}

// normalizeDefaultExpr normalizes default value expressions
func normalizeDefaultExpr(expr string) string {
	// Remove unnecessary parentheses
	expr = strings.TrimSpace(expr)
	for len(expr) >= 2 && expr[0] == '(' && expr[len(expr)-1] == ')' {
		// Check if parentheses are redundant (not a subquery)
		if !isBalancedParens(expr[1 : len(expr)-1]) {
			break
		}
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	return expr
}

// normalizeConstraintDef normalizes constraint definitions
func normalizeConstraintDef(def string) string {
	// Normalize whitespace and case
	def = strings.ToLower(def)
	def = strings.Join(strings.Fields(def), " ")
	return def
}

// isBalancedParens checks if parentheses are balanced
func isBalancedParens(s string) bool {
	count := 0
	for _, ch := range s {
		switch ch {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}
