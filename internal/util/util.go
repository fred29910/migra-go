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

var builtinTypeNames = map[string]bool{
	"int": true, "integer": true, "int4": true, "int8": true, "int2": true,
	"bigint": true, "smallint": true,
	"serial": true, "bigserial": true, "smallserial": true,
	"boolean": true, "bool": true,
	"text": true, "varchar": true, "character varying": true,
	"char": true, "character": true,
	"numeric": true, "decimal": true,
	"real": true, "float4": true, "double precision": true, "float8": true,
	"json": true, "jsonb": true,
	"uuid": true, "inet": true, "cidr": true, "macaddr": true, "macaddr8": true,
	"interval": true, "date": true,
	"time": true, "timetz": true,
	"timestamp": true, "timestamptz": true,
	"timestamp without time zone": true, "timestamp with time zone": true,
	"bytea": true, "money": true, "oid": true, "void": true, "name": true,
	"bpchar": true, "int2vector": true, "oidvector": true,
	"pg_node_tree": true, "pg_ddl_command": true, "pg_snapshot": true,
	"tsvector": true, "tsquery": true, "gtsvector": true,
	"xml": true, "point": true, "line": true, "lseg": true, "box": true,
	"path": true, "polygon": true, "circle": true,
}

// IsBuiltinType reports whether dt is a PostgreSQL built-in type.
// Handles length modifiers (varchar(N)), array suffixes (int[]),
// and case-insensitive matching.
func IsBuiltinType(dt string) bool {
	dt = strings.ToLower(strings.TrimSpace(dt))
	if dt == "" {
		return false
	}
	// Strip array suffix recursively
	if strings.HasSuffix(dt, "[]") {
		return IsBuiltinType(strings.TrimSuffix(dt, "[]"))
	}
	// Strip length modifier: varchar(255) -> varchar, numeric(10,2) -> numeric
	if idx := strings.IndexByte(dt, '('); idx > 0 {
		dt = dt[:idx]
	}
	return builtinTypeNames[dt]
}
