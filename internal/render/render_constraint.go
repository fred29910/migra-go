package render

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

func renderAddConstraint(r *Renderer, op *diff.AddConstraintOp) string {
	definition := renderConstraintDefinition(op.Constraint)
	if definition == "" {
		return ""
	}
	return fmt.Sprintf("-- op: add_constraint risk:medium\nALTER TABLE %s ADD CONSTRAINT %s %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Table),
		util.QuoteIdentifier(op.Constraint.Name),
		definition,
	)
}

func renderDropConstraint(r *Renderer, op *diff.DropConstraintOp) string {
	return fmt.Sprintf("-- op: drop_constraint risk:medium\nALTER TABLE %s DROP CONSTRAINT %s%s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Table),
		ifExistsPrefix(r.useIfExists),
		util.QuoteIdentifier(op.Name),
	)
}

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
