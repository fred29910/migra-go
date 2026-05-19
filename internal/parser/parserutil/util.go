package parserutil

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ParseRelation extracts table/view name and schema from RangeVar.
func ParseRelation(relation *pg_query.RangeVar) (tableName, schemaName string) {
	if relation == nil {
		return "", "public"
	}
	tableName = relation.Relname
	schemaName = relation.Schemaname
	if schemaName == "" {
		schemaName = "public"
	}
	return
}

// ParseColumnDef extracts a model.Column from pg_query ColumnDef node.
func ParseColumnDef(colDef *pg_query.ColumnDef) *model.Column {
	col := &model.Column{IsNullable: !colDef.IsNotNull}
	if colDef.Colname != "" {
		col.Name = colDef.Colname
	}
	if colDef.TypeName != nil {
		col.DataType = ParseTypeName(colDef.TypeName)
	}
	for _, item := range colDef.Constraints {
		if c := item.GetConstraint(); c != nil {
			switch c.Contype {
			case pg_query.ConstrType_CONSTR_NOTNULL:
				col.IsNullable = false
			case pg_query.ConstrType_CONSTR_DEFAULT:
				if c.RawExpr != nil {
					if expr, ok := ParseExpression(c.RawExpr); ok {
						col.DefaultExpr = &expr
					}
				}
			case pg_query.ConstrType_CONSTR_IDENTITY:
				switch c.GeneratedWhen {
				case "a":
					col.IsIdentity = true
					col.IdentityKind = "ALWAYS"
				case "d":
					col.IsIdentity = true
					col.IdentityKind = "BY DEFAULT"
				}
			}
		}
	}
	col.Collation = ExtractCollation(colDef)
	return col
}

// ParseTypeName maps pg_query TypeName to a standard SQL type string.
func ParseTypeName(typeName *pg_query.TypeName) string {
	parts := make([]string, 0)
	for _, item := range typeName.Names {
		if s := item.GetString_(); s != nil {
			if s.Sval != "pg_catalog" {
				parts = append(parts, s.Sval)
			}
		}
	}
	typeStr := strings.Join(parts, ".")
	typeStr = MapTypeName(typeStr)
	if len(typeName.Typmods) > 0 {
		mods := make([]string, 0)
		for _, item := range typeName.Typmods {
			if a := item.GetAConst(); a != nil {
				if ival := a.GetIval(); ival != nil {
					mods = append(mods, fmt.Sprintf("%d", ival.Ival))
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
func ParseExpression(expr *pg_query.Node) (string, bool) {
	switch e := expr.GetNode().(type) {
	case *pg_query.Node_AConst:
		a := e.AConst
		if sval := a.GetSval(); sval != nil {
			return "'" + sval.Sval + "'", true
		}
		if ival := a.GetIval(); ival != nil {
			return fmt.Sprintf("%d", ival.Ival), true
		}
		if fval := a.GetFval(); fval != nil {
			return fval.Fval, true
		}
	case *pg_query.Node_FuncCall:
		parts := make([]string, 0, len(e.FuncCall.Funcname))
		for _, item := range e.FuncCall.Funcname {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		argStrs := make([]string, 0, len(e.FuncCall.Args))
		for _, item := range e.FuncCall.Args {
			if s, ok := ParseExpression(item); ok {
				argStrs = append(argStrs, s)
			}
		}
		return strings.Join(parts, ".") + "(" + strings.Join(argStrs, ", ") + ")", true
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

// ExtractCollation extracts the collation name from a ColumnDef's CollClause.
// Returns empty string if no collation is specified.
func ExtractCollation(colDef *pg_query.ColumnDef) string {
	if colDef.CollClause == nil {
		return ""
	}
	parts := make([]string, 0, len(colDef.CollClause.Collname))
	for _, item := range colDef.CollClause.Collname {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	return strings.Join(parts, ".")
}

// MapTypeName maps PostgreSQL internal type names to standard SQL names.
func MapTypeName(name string) string {
	if mapped, ok := typeNameMapping[name]; ok {
		return mapped
	}
	return name
}
