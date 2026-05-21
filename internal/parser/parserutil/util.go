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

// ParseConstraintColumns extracts column names from pg_query constraint key list
func ParseConstraintColumns(keys []*pg_query.Node) []string {
	cols := make([]string, 0, len(keys))
	for _, key := range keys {
		if s := key.GetString_(); s != nil {
			cols = append(cols, s.Sval)
		}
	}
	return cols
}

// DeparseNode converts a pg_query Node back to SQL string using pg_query.Deparse.
// pg_query.Deparse only supports top-level statement nodes (CREATE TABLE, ALTER TABLE, etc.).
// For expression-level nodes (NullTest, AExpr, FuncCall, TypeCast, etc.) it fails,
// so we fall back to FormatExpression which handles common expression types.
func DeparseNode(node *pg_query.Node) string {
	sql, err := pg_query.Deparse(&pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{{Stmt: node}},
	})
	if err == nil {
		return strings.TrimSpace(strings.TrimSuffix(sql, ";"))
	}
	// Fall back to FormatExpression for expression-level nodes
	return FormatExpression(node)
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

// FormatExpression formats a pg_query expression node as a SQL expression string.
// Handles TypeCast, ColumnRef, FuncCall, AConst, NullTest, AExpr (binary ops), and BoolExpr.
// Returns empty string for unsupported node types.
func FormatExpression(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_TypeCast:
		tc := n.TypeCast
		arg := FormatExpression(tc.Arg)
		typeName := ParseTypeName(tc.TypeName)
		return arg + "::" + typeName
	case *pg_query.Node_ColumnRef:
		fields := make([]string, 0)
		for _, f := range n.ColumnRef.Fields {
			if s := f.GetString_(); s != nil {
				fields = append(fields, s.Sval)
			}
		}
		return strings.Join(fields, ".")
	case *pg_query.Node_FuncCall:
		parts := make([]string, 0, len(n.FuncCall.Funcname))
		for _, item := range n.FuncCall.Funcname {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		argStrs := make([]string, 0, len(n.FuncCall.Args))
		for _, item := range n.FuncCall.Args {
			argStrs = append(argStrs, FormatExpression(item))
		}
		return strings.Join(parts, ".") + "(" + strings.Join(argStrs, ", ") + ")"
	case *pg_query.Node_AConst:
		a := n.AConst
		if sval := a.GetSval(); sval != nil {
			return "'" + sval.Sval + "'"
		}
		if ival := a.GetIval(); ival != nil {
			return fmt.Sprintf("%d", ival.Ival)
		}
		if fval := a.GetFval(); fval != nil {
			return fval.Fval
		}
	case *pg_query.Node_NullTest:
		nt := n.NullTest
		arg := FormatExpression(nt.Arg)
		if arg == "" {
			return ""
		}
		switch nt.Nulltesttype {
		case pg_query.NullTestType_IS_NULL:
			return arg + " IS NULL"
		case pg_query.NullTestType_IS_NOT_NULL:
			return arg + " IS NOT NULL"
		}
	case *pg_query.Node_AExpr:
		a := n.AExpr
		if a.Kind != pg_query.A_Expr_Kind_AEXPR_OP || len(a.Name) == 0 {
			return ""
		}
		op := ""
		for _, name := range a.Name {
			if s := name.GetString_(); s != nil {
				op += s.Sval
			}
		}
		left := FormatExpression(a.Lexpr)
		right := FormatExpression(a.Rexpr)
		if left == "" || right == "" || op == "" {
			return ""
		}
		return left + " " + op + " " + right
	case *pg_query.Node_BoolExpr:
		b := n.BoolExpr
		switch b.Boolop {
		case pg_query.BoolExprType_NOT_EXPR:
			if len(b.Args) == 1 {
				arg := FormatExpression(b.Args[0])
				if arg == "" {
					return ""
				}
				return "NOT " + arg
			}
		case pg_query.BoolExprType_AND_EXPR, pg_query.BoolExprType_OR_EXPR:
			parts := make([]string, 0, len(b.Args))
			for _, arg := range b.Args {
				s := FormatExpression(arg)
				if s == "" {
					return ""
				}
				parts = append(parts, s)
			}
			op := "AND"
			if b.Boolop == pg_query.BoolExprType_OR_EXPR {
				op = "OR"
			}
			return "(" + strings.Join(parts, " "+op+" ") + ")"
		}
	}
	return ""
}

// MapTypeName maps PostgreSQL internal type names to standard SQL names.
func MapTypeName(name string) string {
	if mapped, ok := typeNameMapping[name]; ok {
		return mapped
	}
	return name
}
