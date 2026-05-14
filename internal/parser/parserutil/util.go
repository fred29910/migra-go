package parserutil

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// ParseRelation extracts table/view name and schema from RangeVar.
func ParseRelation(relation *pg_nodes.RangeVar) (tableName, schemaName string) {
	if relation == nil {
		return "", "public"
	}
	if relation.Relname != nil {
		tableName = *relation.Relname
	}
	if relation.Schemaname != nil {
		schemaName = *relation.Schemaname
	} else {
		schemaName = "public"
	}
	return
}

// ParseColumnDef extracts a model.Column from pg_query ColumnDef node.
func ParseColumnDef(elt pg_nodes.ColumnDef) *model.Column {
	col := &model.Column{IsNullable: true}
	if elt.Colname != nil {
		col.Name = *elt.Colname
	}
	if elt.TypeName != nil {
		col.DataType = ParseTypeName(*elt.TypeName)
	}
	for _, item := range elt.Constraints.Items {
		switch c := item.(type) {
		case pg_nodes.Constraint:
			switch c.Contype {
			case pg_nodes.CONSTR_NOTNULL:
				col.IsNullable = false
			case pg_nodes.CONSTR_DEFAULT:
				if c.RawExpr != nil {
					expr, ok := ParseExpression(c.RawExpr)
					if ok {
						col.DefaultExpr = &expr
					}
				}
			}
		}
	}
	return col
}

// ParseTypeName maps pg_query TypeName to a standard SQL type string.
func ParseTypeName(typeName pg_nodes.TypeName) string {
	parts := make([]string, 0)
	for _, item := range typeName.Names.Items {
		if s, ok := item.(pg_nodes.String); ok {
			if s.Str != "pg_catalog" {
				parts = append(parts, s.Str)
			}
		}
	}
	typeStr := strings.Join(parts, ".")
	typeStr = MapTypeName(typeStr)
	if len(typeName.Typmods.Items) > 0 {
		mods := make([]string, 0)
		for _, item := range typeName.Typmods.Items {
			if a, ok := item.(pg_nodes.A_Const); ok {
				if a.Val != nil {
					if i, ok := a.Val.(pg_nodes.Integer); ok {
						mods = append(mods, fmt.Sprintf("%d", i.Ival))
					}
				}
			}
		}
		if len(mods) > 0 {
			typeStr += "(" + strings.Join(mods, ",") + ")"
		}
	}
	return strings.ToLower(typeStr)
}

// ParseExpression extracts expression as string from AST node.
func ParseExpression(expr pg_nodes.Node) (string, bool) {
	switch e := expr.(type) {
	case pg_nodes.A_Const:
		if e.Val != nil {
			switch v := e.Val.(type) {
			case pg_nodes.String:
				return "'" + v.Str + "'", true
			case pg_nodes.Integer:
				return fmt.Sprintf("%d", v.Ival), true
			case pg_nodes.Float:
				return v.Str, true
			}
		}
	case pg_nodes.FuncCall:
		parts := make([]string, 0, len(e.Funcname.Items))
		for _, item := range e.Funcname.Items {
			if s, ok := item.(pg_nodes.String); ok {
				parts = append(parts, s.Str)
			}
		}
		argStrs := make([]string, 0, len(e.Args.Items))
		for _, item := range e.Args.Items {
			if s, ok := ParseExpression(item); ok {
				argStrs = append(argStrs, s)
			}
		}
		return strings.Join(parts, ".") + "(" + strings.Join(argStrs, ", ") + ")", true
	}
	if d, ok := expr.(interface{ Deparse() string }); ok {
		var out string
		func() {
			defer func() {
				if r := recover(); r != nil {
					out = ""
				}
			}()
			out = d.Deparse()
		}()
		out = strings.TrimSpace(out)
		if out != "" {
			return out, true
		}
	}
	return "", false
}

var typeNameMapping = map[string]string{
	"int4":        "integer",
	"int8":        "bigint",
	"float4":      "real",
	"float8":      "double precision",
	"serial":      "integer",
	"bigserial":   "bigint",
	"smallserial": "smallint",
}

// MapTypeName maps PostgreSQL internal type names to standard SQL names.
func MapTypeName(name string) string {
	if mapped, ok := typeNameMapping[name]; ok {
		return mapped
	}
	return name
}
