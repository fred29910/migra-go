package normalize

import (
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// CanonicalizeSchema normalizes a schema to reduce false diffs
// Returns a new normalized schema
func CanonicalizeSchema(s *model.Schema) (*model.Schema, error) {
	normalized := &model.Schema{
		Schemas: make(map[string]*model.Namespace),
	}

	for name, ns := range s.Schemas {
		normalized.Schemas[name] = canonicalizeNamespace(ns)
	}

	return normalized, nil
}

func canonicalizeNamespace(ns *model.Namespace) *model.Namespace {
	result := &model.Namespace{
		Name:   ns.Name,
		Tables: make(map[string]*model.Table),
		Types:  make(map[string]*model.EnumType),
	}

	// Normalize tables
	for name, table := range ns.Tables {
		result.Tables[name] = canonicalizeTable(table)
	}

	// Normalize types
	for name, enumType := range ns.Types {
		result.Types[name] = canonicalizeEnumType(enumType)
	}

	return result
}

func canonicalizeTable(table *model.Table) *model.Table {
	result := &model.Table{
		Schema:      table.Schema,
		Name:        table.Name,
		Columns:     make([]*model.Column, len(table.Columns)),
		PrimaryKey:  table.PrimaryKey,
		Constraints: make(map[string]*model.Constraint),
		Indexes:     make(map[string]*model.Index),
	}

	// Normalize columns
	for i, col := range table.Columns {
		result.Columns[i] = canonicalizeColumn(col)
	}

	// Normalize constraints
	for name, constraint := range table.Constraints {
		result.Constraints[name] = canonicalizeConstraint(constraint)
	}

	// Normalize indexes
	for name, index := range table.Indexes {
		result.Indexes[name] = canonicalizeIndex(index)
	}

	return result
}

func canonicalizeColumn(col *model.Column) *model.Column {
	result := &model.Column{
		Name:         normalizeIdentifier(col.Name),
		DataType:     normalizeDataType(col.DataType),
		IsNullable:   col.IsNullable,
		IsIdentity:   col.IsIdentity,
		IdentityKind: col.IdentityKind,
	}

	if col.DefaultExpr != nil {
		normalized := normalizeDefaultExpr(*col.DefaultExpr)
		result.DefaultExpr = &normalized
	}

	return result
}

func canonicalizeEnumType(enumType *model.EnumType) *model.EnumType {
	return &model.EnumType{
		Name:   normalizeIdentifier(enumType.Name),
		Labels: enumType.Labels, // Labels order is significant
	}
}

func canonicalizeConstraint(c *model.Constraint) *model.Constraint {
	return &model.Constraint{
		Name:       normalizeIdentifier(c.Name),
		Type:       c.Type,
		Definition: normalizeConstraintDef(c.Definition),
		Table:      c.Table,
	}
}

func canonicalizeIndex(idx *model.Index) *model.Index {
	return &model.Index{
		Name:    normalizeIdentifier(idx.Name),
		Table:   idx.Table,
		Columns: idx.Columns,
		Unique:  idx.Unique,
		Method:  idx.Method,
	}
}

// normalizeDataType normalizes type aliases to canonical names
func normalizeDataType(dt string) string {
	dt = strings.ToLower(strings.TrimSpace(dt))

	// Type aliases mapping
	aliases := map[string]string{
		"int4":                        "integer",
		"int8":                        "bigint",
		"int2":                        "smallint",
		"bool":                        "boolean",
		"character varying":           "varchar",
		"timestamp without time zone": "timestamp",
		"timestamp with time zone":    "timestamptz",
	}

	if canonical, ok := aliases[dt]; ok {
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
