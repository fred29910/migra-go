package util

import (
	"regexp"
	"strings"
)

// QuoteIdentifier quotes a SQL identifier using the PostgreSQL standard
// (double quotes, escaping internal double quotes by doubling them).
func QuoteIdentifier(name string) string {
	// Always quote to be safe with reserved words
	// Escape internal double quotes by doubling them (SQL standard)
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// QuoteQualifiedIdentifier quotes a schema-qualified identifier (schema.name).
func QuoteQualifiedIdentifier(schema, name string) string {
	return QuoteIdentifier(schema) + "." + QuoteIdentifier(name)
}

// QuoteIdentifierList quotes a list of identifiers and joins them with commas.
func QuoteIdentifierList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = QuoteIdentifier(item)
	}
	return strings.Join(quoted, ", ")
}

// typeAliases maps type aliases to canonical names (exact matches only, no length suffix).
var typeAliases = map[string]string{
	"int4":                        "integer",
	"int8":                        "bigint",
	"int2":                        "smallint",
	"bool":                        "boolean",
	"character varying":           "varchar",
	"character":                   "char",
	"timestamp without time zone": "timestamp",
	"timestamp with time zone":    "timestamptz",
}

// charVaryingWithLenRe matches "character varying(N)" (case-insensitive).
var charVaryingWithLenRe = regexp.MustCompile(`(?i)^character varying\((\d+)\)$`)

// characterWithLenRe matches "character(N)" / "char(N)" (case-insensitive).
var characterWithLenRe = regexp.MustCompile(`(?i)^character\((\d+)\)$`)

// NormalizeDataType normalizes type aliases to canonical names.
func NormalizeDataType(dt string) string {
	dt = strings.TrimSpace(dt)
	lower := strings.ToLower(dt)

	if strings.HasSuffix(lower, "[]") {
		base := strings.TrimSuffix(lower, "[]")
		return NormalizeDataType(base) + "[]"
	}

	if m := charVaryingWithLenRe.FindStringSubmatch(lower); m != nil {
		return "varchar(" + m[1] + ")"
	}

	if m := characterWithLenRe.FindStringSubmatch(lower); m != nil {
		return "char(" + m[1] + ")"
	}

	if canonical, ok := typeAliases[lower]; ok {
		return canonical
	}
	return lower
}

// SameStringSlice reports whether a and b contain the same elements in the same order.
func SameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
