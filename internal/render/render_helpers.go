package render

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

func renderConstraint(c *model.Constraint) string {
	if c == nil {
		return ""
	}
	definition := renderConstraintDefinition(c)
	if definition == "" {
		return ""
	}
	return fmt.Sprintf("CONSTRAINT %s %s", util.QuoteIdentifier(c.Name), definition)
}

func renderConstraintDefinition(c *model.Constraint) string {
	if c == nil {
		return ""
	}
	switch c.Type {
	case "primary_key":
		return fmt.Sprintf("PRIMARY KEY (%s)", util.QuoteIdentifierList(c.Columns))
	case "foreign_key":
		sql := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
			util.QuoteIdentifierList(c.Columns),
			util.QuoteQualifiedIdentifier(c.RefSchema, c.RefTable),
			util.QuoteIdentifierList(c.RefColumns),
		)
		if c.OnDelete != "" {
			sql += fmt.Sprintf(" ON DELETE %s", c.OnDelete)
		}
		if c.OnUpdate != "" {
			sql += fmt.Sprintf(" ON UPDATE %s", c.OnUpdate)
		}
		return sql
	case "unique":
		return fmt.Sprintf("UNIQUE (%s)", util.QuoteIdentifierList(c.Columns))
	case "check":
		if c.Expression != "" {
			return fmt.Sprintf("CHECK (%s)", c.Expression)
		}
		return ""
	}
	if c.Definition != "" {
		return c.Definition
	}
	return ""
}

func quoteString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func ifExistsPrefix(use bool) string {
	if use {
		return "IF EXISTS "
	}
	return ""
}

func renderCreateIndex(r *Renderer, op *diff.CreateIndexOp) string {
	idx := op.Index
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	concurrently := ""
	if idx.Concurrent {
		concurrently = "CONCURRENTLY "
	}
	ifNotExists := ""
	if idx.IfNotExists {
		ifNotExists = "IF NOT EXISTS "
	}
	method := ""
	if idx.Method != "" {
		method = " USING " + idx.Method
	}

	quotedItems := make([]string, 0, len(idx.Elements))
	if len(idx.Elements) > 0 {
		for _, elem := range idx.Elements {
			if s := renderIndexElem(elem); s != "" {
				quotedItems = append(quotedItems, s)
			}
		}
	} else if len(idx.Columns) > 0 {
		for _, c := range idx.Columns {
			quotedItems = append(quotedItems, util.QuoteIdentifier(c))
		}
	}
	items := strings.Join(quotedItems, ", ")

	sql := fmt.Sprintf("CREATE %s%s%sINDEX %s ON %s%s (%s)",
		unique, concurrently, ifNotExists, util.QuoteIdentifier(idx.Name),
		util.QuoteQualifiedIdentifier(op.Schema, idx.Table), method, items)

	if idx.WhereClause != "" {
		sql += " WHERE " + idx.WhereClause
	}
	return fmt.Sprintf("-- op: add_index risk:low\n%s;", sql)
}

func renderIndexElem(elem model.IndexElem) string {
	item := ""
	if elem.Name != "" {
		item = util.QuoteIdentifier(elem.Name)
	} else if elem.Expr != "" {
		item = "(" + elem.Expr + ")"
	}
	if elem.Collation != "" {
		item += " COLLATE " + util.QuoteIdentifier(elem.Collation)
	}
	if elem.Opclass != "" {
		item += " " + elem.Opclass
	}
	if elem.Ordering == "ASC" || elem.Ordering == "DESC" {
		item += " " + elem.Ordering
	}
	if elem.NullsOrdering == "FIRST" || elem.NullsOrdering == "LAST" {
		item += " NULLS " + elem.NullsOrdering
	}
	return item
}
