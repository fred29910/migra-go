package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateTableHandler handles CREATE TABLE statements.
type CreateTableHandler struct{}

func defaultConstraintName(table string, constraint model.Constraint) string {
	if constraint.Name != "" {
		return constraint.Name
	}
	if constraint.Type == "primary_key" {
		return table + "_pkey"
	}
	if constraint.Type == "foreign_key" && len(constraint.Columns) > 0 {
		return table + "_" + strings.Join(constraint.Columns, "_") + "_fkey"
	}
	return table + "_constraint"
}

// Handle converts a pg_query CreateStmt into schema mutations.
func (h *CreateTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.CreateStmt)
	if !ok {
		return nil, fmt.Errorf("CreateTableHandler: expected pg_nodes.CreateStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	var primaryKey *model.PrimaryKey
	var constraints []model.Constraint

	for _, item := range stmt.TableElts.Items {
		switch elt := item.(type) {
		case pg_nodes.ColumnDef:
			col := parserutil.ParseColumnDef(elt)
			for _, conItem := range elt.Constraints.Items {
				if c, ok := conItem.(pg_nodes.Constraint); ok {
					if c.Contype == pg_nodes.CONSTR_PRIMARY {
						col.IsNullable = false
						primaryKey = &model.PrimaryKey{
							Name:    defaultConstraintName(tableName, model.Constraint{Name: "", Type: "primary_key"}),
							Columns: []string{col.Name},
						}
						constraints = append(constraints, model.Constraint{
							Name:    primaryKey.Name,
							Type:    "primary_key",
							Columns: []string{col.Name},
						})
					}
				}
			}
			columns = append(columns, *col)
		case pg_nodes.Constraint:
			switch elt.Contype {
			case pg_nodes.CONSTR_PRIMARY:
				var cols []string
				for _, key := range elt.Keys.Items {
					if s, ok := key.(pg_nodes.String); ok {
						cols = append(cols, s.Str)
					}
				}
				primaryKey = &model.PrimaryKey{
					Name:    defaultConstraintName(tableName, model.Constraint{Name: "", Type: "primary_key"}),
					Columns: cols,
				}
				constraints = append(constraints, model.Constraint{
					Name:    primaryKey.Name,
					Type:    "primary_key",
					Columns: cols,
				})
			case pg_nodes.CONSTR_FOREIGN:
				var fkCols []string
				for _, attr := range elt.FkAttrs.Items {
					if s, ok := attr.(pg_nodes.String); ok {
						fkCols = append(fkCols, s.Str)
					}
				}
				var refCols []string
				for _, attr := range elt.PkAttrs.Items {
					if s, ok := attr.(pg_nodes.String); ok {
						refCols = append(refCols, s.Str)
					}
				}
				refSchema := "public"
				if elt.Pktable != nil && elt.Pktable.Schemaname != nil {
					refSchema = *elt.Pktable.Schemaname
				}
				refTable := ""
				if elt.Pktable != nil && elt.Pktable.Relname != nil {
					refTable = *elt.Pktable.Relname
				}
				con := model.Constraint{
					Name:       "",
					Type:       "foreign_key",
					Columns:    fkCols,
					RefSchema:  refSchema,
					RefTable:   refTable,
					RefColumns: refCols,
				}
				con.Name = defaultConstraintName(tableName, con)
				constraints = append(constraints, con)
			}
		}
	}

	return []SchemaMutation{
		CreateTableMutation{
			Schema:      schemaName,
			Name:        tableName,
			Columns:     columns,
			PrimaryKey:  primaryKey,
			Constraints: constraints,
		},
	}, nil
}
