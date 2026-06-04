package normalize

import (
	"regexp"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
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

	for _, view := range ns.Views {
		canonicalizeViewInPlace(view)
	}

	for _, seq := range ns.Sequences {
		canonicalizeSequenceInPlace(seq)
	}

	for _, ext := range ns.Extensions {
		canonicalizeExtensionInPlace(ext)
	}
}

func canonicalizeTableInPlace(table *model.Table) {
	// ColumnByName might change if names are normalized
	newColumnByName := make(map[string]*model.Column, len(table.Columns))

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
	col.DataType = util.NormalizeDataType(col.DataType)
	if col.DefaultExpr != nil {
		*col.DefaultExpr = normalizeDefaultExpr(*col.DefaultExpr)
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
	// DB introspect populates Columns but not Elements; SQL file parser populates Elements.
	// Unify them so sameIndexContent comparison is correct and push is idempotent.
	if len(idx.Elements) == 0 && len(idx.Columns) > 0 {
		idx.Elements = make([]model.IndexElem, len(idx.Columns))
		for i, col := range idx.Columns {
			idx.Elements[i] = model.IndexElem{
				Name:          col,
				Ordering:      "default",
				NullsOrdering: "default",
			}
		}
	}
	// Normalize Definition and WhereClause whitespace
	idx.Definition = strings.Join(strings.Fields(idx.Definition), " ")
	idx.WhereClause = strings.Join(strings.Fields(idx.WhereClause), " ")
}

func canonicalizeViewInPlace(v *model.View) {
	v.Name = normalizeIdentifier(v.Name)
	v.Definition = strings.TrimSuffix(strings.TrimSpace(v.Definition), ";")
}

func canonicalizeSequenceInPlace(s *model.Sequence) {
	s.Name = normalizeIdentifier(s.Name)
	s.DataType = util.NormalizeDataType(s.DataType)
}

func canonicalizeExtensionInPlace(e *model.Extension) {
	e.Name = normalizeIdentifier(e.Name)
}

// These two regexes work together in normalizeDefaultExpr and must run in this order:
// 1. nestedTypeCastRe first strips 'literal'::type → 'literal' (nested casts)
// 2. typeCastRe then strips any remaining ::type suffix
// Reversing the order would break cases like nextval('seq'::regclass).
var typeCastRe = regexp.MustCompile(`::[\w\s]+$`)
var nestedTypeCastRe = regexp.MustCompile(`'([^']*)'::[\w\s]+`)

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
	expr = strings.TrimSpace(expr)
	for len(expr) >= 2 && expr[0] == '(' && expr[len(expr)-1] == ')' {
		if !isBalancedParens(expr[1 : len(expr)-1]) {
			break
		}
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}

	expr = nestedTypeCastRe.ReplaceAllString(expr, "'$1'")

	expr = typeCastRe.ReplaceAllString(expr, "")
	return strings.TrimSpace(expr)
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
