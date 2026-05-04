package render

import (
	"fmt"
	"strings"

	"github.com/migra-go/migra-go/internal/model"
)

// Renderer converts diff operations into SQL statements
type Renderer struct {
	sql []string
}

// NewRenderer creates a new Renderer
func NewRenderer() *Renderer {
	return &Renderer{
		sql: make([]string, 0),
	}
}

// RenderAll converts all diff operations to SQL
func (r *Renderer) RenderAll(ops []*model.DiffOp) string {
	// Sort operations by dependency order
	sorted := sortOps(ops)

	for _, op := range sorted {
		sql := r.Render(op)
		if sql != "" {
			r.sql = append(r.sql, sql)
		}
	}

	return r.String()
}

// Render converts a single diff operation to SQL
func (r *Renderer) Render(op *model.DiffOp) string {
	switch op.Kind {
	case model.AddTable:
		return r.renderAddTable(op)
	case model.DropTable:
		return r.renderDropTable(op)
	case model.AddColumn:
		return r.renderAddColumn(op)
	case model.DropColumn:
		return r.renderDropColumn(op)
	case model.AlterColumn:
		return r.renderAlterColumn(op)
	case model.AddIndex:
		return r.renderAddIndex(op)
	case model.DropIndex:
		return r.renderDropIndex(op)
	case model.AddConstraint:
		return r.renderAddConstraint(op)
	case model.DropConstraint:
		return r.renderDropConstraint(op)
	default:
		return ""
	}
}

func (r *Renderer) renderAddTable(op *model.DiffOp) string {
	// TODO: Generate CREATE TABLE statement
	return fmt.Sprintf("-- CREATE TABLE %s (...)", op.Obj)
}

func (r *Renderer) renderDropTable(op *model.DiffOp) string {
	return fmt.Sprintf("DROP TABLE %s;", op.Obj)
}

func (r *Renderer) renderAddColumn(op *model.DiffOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		op.Obj, op.Obj, op.Details["data_type"])

	if op.Details["nullable"] == "false" {
		sql += " NOT NULL"
	}

	if d, ok := op.Details["default"]; ok {
		sql += " DEFAULT " + d
	}

	return sql + ";"
}

func (r *Renderer) renderDropColumn(op *model.DiffOp) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", op.Obj, op.Obj)
}

func (r *Renderer) renderAlterColumn(op *model.DiffOp) string {
	// TODO: Handle different types of column alterations
	return fmt.Sprintf("-- ALTER TABLE %s ALTER COLUMN ...", op.Obj)
}

func (r *Renderer) renderAddIndex(op *model.DiffOp) string {
	unique := ""
	if op.Details["unique"] == "true" {
		unique = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);",
		unique, op.Obj, op.Obj, op.Details["columns"])
}

func (r *Renderer) renderDropIndex(op *model.DiffOp) string {
	return fmt.Sprintf("DROP INDEX %s;", op.Obj)
}

func (r *Renderer) renderAddConstraint(op *model.DiffOp) string {
	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s %s;",
		op.Obj, op.Obj, op.Details["definition"])
}

func (r *Renderer) renderDropConstraint(op *model.DiffOp) string {
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", op.Obj, op.Obj)
}

// String returns the rendered SQL as a string
func (r *Renderer) String() string {
	if len(r.sql) == 0 {
		return "-- No changes detected"
	}

	return "-- Begin Diff\n" + strings.Join(r.sql, "\n") + "\n-- End Diff"
}

// sortOps sorts operations by dependency order
func sortOps(ops []*model.DiffOp) []*model.DiffOp {
	// TODO: Implement proper topological sorting
	return ops
}
