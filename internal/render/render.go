package render

import (
	"fmt"
	"strings"

	"github.com/migra-go/migra-go/internal/diff"
	"github.com/migra-go/migra-go/internal/model"
)

// Renderer converts diff operations into SQL statements
type Renderer struct {
	sql        []string
	format     string // "sql" or "json"
	useIfExists bool
}

// NewRenderer creates a new Renderer
func NewRenderer() *Renderer {
	return &Renderer{
		sql:        make([]string, 0),
		format:     "sql",
		useIfExists: true,
	}
}

// SetFormat sets the output format
func (r *Renderer) SetFormat(format string) {
	r.format = format
}

// RenderAll converts all operations to SQL
func (r *Renderer) RenderAll(ops []diff.Operation) string {
	// Sort operations by dependency order (TODO: implement proper sorting)
	
	for _, op := range ops {
		sql := r.Render(op)
		if sql != "" {
			r.sql = append(r.sql, sql)
		}
	}

	return r.String()
}

// Render converts a single operation to SQL
func (r *Renderer) Render(op diff.Operation) string {
	switch v := op.(type) {
	case *diff.AddTableOp:
		return r.renderAddTable(v)
	case *diff.DropTableOp:
		return r.renderDropTable(v)
	case *diff.AddColumnOp:
		return r.renderAddColumn(v)
	case *diff.AlterColumnTypeOp:
		return r.renderAlterColumnType(v)
	case *diff.SetNotNullOp:
		return r.renderSetNotNull(v)
	case *diff.DropNotNullOp:
		return r.renderDropNotNull(v)
	case *diff.CreateIndexOp:
		return r.renderCreateIndex(v)
	case *diff.DropIndexOp:
		return r.renderDropIndex(v)
	case *diff.AddEnumTypeOp:
		return r.renderAddEnumType(v)
	case *diff.DropEnumTypeOp:
		return r.renderDropEnumType(v)
	default:
		return fmt.Sprintf("-- Unknown operation: %T", op)
	}
}

func (r *Renderer) renderAddTable(op *diff.AddTableOp) string {
	table := op.Table
	columns := make([]string, 0)
	
	for _, col := range table.Columns {
		colDef := fmt.Sprintf("    %s %s", quoteIdentifier(col.Name), col.DataType)
		if !col.IsNullable {
			colDef += " NOT NULL"
		}
		if col.DefaultExpr != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
		}
		columns = append(columns, colDef)
	}

	// TODO: Add primary key, constraints
	
	sql := fmt.Sprintf("-- op: add_table risk:low\nCREATE TABLE %s (\n%s\n);",
		quoteIdentifier(table.Name),
		strings.Join(columns, ",\n"))
	return sql
}

func (r *Renderer) renderDropTable(op *diff.DropTableOp) string {
	ifExists := ""
	if r.useIfExists {
		ifExists = "IF EXISTS "
	}
	return fmt.Sprintf("-- op: drop_table risk:high\nDROP TABLE %s%s;", ifExists, quoteIdentifier(op.Name))
}

func (r *Renderer) renderAddColumn(op *diff.AddColumnOp) string {
	col := op.Column
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		quoteIdentifier(op.Table), quoteIdentifier(col.Name), col.DataType)
	
	if !col.IsNullable {
		sql += " NOT NULL"
	}
	if col.DefaultExpr != nil {
		sql += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
	}
	
	return fmt.Sprintf("-- op: add_column risk:low\n%s;", sql)
}

func (r *Renderer) renderAlterColumnType(op *diff.AlterColumnTypeOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
		quoteIdentifier(op.Table), quoteIdentifier(op.Column), op.ToType)
	return fmt.Sprintf("-- op: alter_column_type risk:high\n%s;", sql)
}

func (r *Renderer) renderSetNotNull(op *diff.SetNotNullOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL",
		quoteIdentifier(op.Table), quoteIdentifier(op.Column))
	return fmt.Sprintf("-- op: set_not_null risk:low\n%s;", sql)
}

func (r *Renderer) renderDropNotNull(op *diff.DropNotNullOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL",
		quoteIdentifier(op.Table), quoteIdentifier(op.Column))
	return fmt.Sprintf("-- op: drop_not_null risk:low\n%s;", sql)
}

func (r *Renderer) renderCreateIndex(op *diff.CreateIndexOp) string {
	idx := op.Index
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	columns := strings.Join(idx.Columns, ", ")
	return fmt.Sprintf("-- op: add_index risk:low\nCREATE %sINDEX %s ON %s (%s);",
		unique, quoteIdentifier(idx.Name), quoteIdentifier(idx.Table), columns)
}

func (r *Renderer) renderDropIndex(op *diff.DropIndexOp) string {
	return fmt.Sprintf("-- op: drop_index risk:medium\nDROP INDEX %s%s;", 
		ifExistsPrefix(r.useIfExists), quoteIdentifier(op.Name))
}

func (r *Renderer) renderAddEnumType(op *diff.AddEnumTypeOp) string {
	labels := make([]string, len(op.Type.Labels))
	for i, label := range op.Type.Labels {
		labels[i] = quoteString(label)
	}
	sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		quoteIdentifier(op.Type.Name), strings.Join(labels, ", "))
	return fmt.Sprintf("-- op: add_enum_type risk:low\n%s;", sql)
}

func (r *Renderer) renderDropEnumType(op *diff.DropEnumTypeOp) string {
	return fmt.Sprintf("-- op: drop_enum_type risk:high\nDROP TYPE %s%s;",
		ifExistsPrefix(r.useIfExists), quoteIdentifier(op.Name))
}

// String returns the rendered SQL as a string
func (r *Renderer) String() string {
	if len(r.sql) == 0 {
		return "-- No changes detected"
	}

	return "-- Begin Diff\n" + strings.Join(r.sql, "\n") + "\n-- End Diff"
}

// Helper functions

func quoteIdentifier(id string) string {
	// Always quote to be safe with reserved words
	return fmt.Sprintf(`"%s"`, id)
}

func quoteString(s string) string {
	return fmt.Sprintf("'%s'", s)
}

func ifExistsPrefix(use bool) string {
	if use {
		return "IF EXISTS "
	}
	return ""
}

// RenderJSON renders operations as JSON (for programmatic use)
func RenderJSON(ops []diff.Operation) (string, error) {
	// Convert operations to a JSON-friendly format
	type OpInfo struct {
		Kind    string             `json:"kind"`
		Object  model.ObjectKey   `json:"object"`
		Destructive bool          `json:"destructive"`
	}
	
	infos := make([]OpInfo, len(ops))
	for i, op := range ops {
		infos[i] = OpInfo{
			Kind:       string(op.Kind()),
			Object:     op.ObjectKey(),
			Destructive: op.IsDestructive(),
		}
	}
	
	// TODO: implement JSON marshaling
	_ = infos
	return "", fmt.Errorf("JSON rendering not yet implemented")
}
