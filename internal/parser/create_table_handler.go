package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
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
	if constraint.Type == "unique" && len(constraint.Columns) > 0 {
		return table + "_" + strings.Join(constraint.Columns, "_") + "_key"
	}
	if constraint.Type == "check" {
		return table + "_check"
	}
	if constraint.Type == "foreign_key" && len(constraint.Columns) > 0 {
		return table + "_" + strings.Join(constraint.Columns, "_") + "_fkey"
	}
	return table + "_constraint"
}

// fkActionCode delegates to the shared FK action code mapping in the model package.
// The same single-character codes are used by pg_query (for CREATE TABLE parsing)
// and pg_constraint (for database introspection).
func fkActionCode(code string) string {
	return model.FKActionCode(code)
}

// Handle converts a pg_query CreateStmt into schema mutations.
func (h *CreateTableHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateTableHandler: expected CreateStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	var primaryKey *model.PrimaryKey
	var constraints []model.Constraint

	for _, item := range stmt.TableElts {
		switch elt := item.GetNode().(type) {
		case *pg_query.Node_ColumnDef:
			colDef := elt.ColumnDef
			col := parserutil.ParseColumnDef(colDef)
			for _, conItem := range colDef.Constraints {
				if c := conItem.GetConstraint(); c != nil {
					switch c.Contype {
					case pg_query.ConstrType_CONSTR_PRIMARY:
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
					case pg_query.ConstrType_CONSTR_FOREIGN:
						var refCols []string
						for _, attr := range c.PkAttrs {
							if s := attr.GetString_(); s != nil {
								refCols = append(refCols, s.Sval)
							}
						}
						refSchema := "public"
						if c.Pktable != nil && c.Pktable.Schemaname != "" {
							refSchema = c.Pktable.Schemaname
						}
						refTable := ""
						if c.Pktable != nil {
							refTable = c.Pktable.Relname
						}
						con := model.Constraint{
							Name:       "",
							Type:       "foreign_key",
							Columns:    []string{col.Name},
							RefSchema:  refSchema,
							RefTable:   refTable,
							RefColumns: refCols,
							OnDelete:   fkActionCode(c.FkDelAction),
							OnUpdate:   fkActionCode(c.FkUpdAction),
						}
						con.Name = defaultConstraintName(tableName, con)
						constraints = append(constraints, con)
					case pg_query.ConstrType_CONSTR_UNIQUE:
						con := model.Constraint{
							Name:    c.Conname,
							Type:    "unique",
							Columns: []string{col.Name},
						}
						con.Name = defaultConstraintName(tableName, con)
						constraints = append(constraints, con)
					case pg_query.ConstrType_CONSTR_CHECK:
						con := model.Constraint{
							Name:       c.Conname,
							Type:       "check",
							Columns:    []string{col.Name},
							Expression: parserutil.DeparseNode(c.RawExpr),
						}
						con.Name = defaultConstraintName(tableName, con)
						constraints = append(constraints, con)
					}
				}
			}
			columns = append(columns, *col)
		case *pg_query.Node_Constraint:
			constraint := elt.Constraint
			switch constraint.Contype {
			case pg_query.ConstrType_CONSTR_PRIMARY:
				var cols []string
				for _, key := range constraint.Keys {
					if s := key.GetString_(); s != nil {
						cols = append(cols, s.Sval)
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
			case pg_query.ConstrType_CONSTR_FOREIGN:
				var fkCols []string
				for _, attr := range constraint.FkAttrs {
					if s := attr.GetString_(); s != nil {
						fkCols = append(fkCols, s.Sval)
					}
				}
				var refCols []string
				for _, attr := range constraint.PkAttrs {
					if s := attr.GetString_(); s != nil {
						refCols = append(refCols, s.Sval)
					}
				}
				refSchema := "public"
				if constraint.Pktable != nil && constraint.Pktable.Schemaname != "" {
					refSchema = constraint.Pktable.Schemaname
				}
				refTable := ""
				if constraint.Pktable != nil {
					refTable = constraint.Pktable.Relname
				}
				con := model.Constraint{
					Name:       "",
					Type:       "foreign_key",
					Columns:    fkCols,
					RefSchema:  refSchema,
					RefTable:   refTable,
					RefColumns: refCols,
					OnDelete:   fkActionCode(constraint.FkDelAction),
					OnUpdate:   fkActionCode(constraint.FkUpdAction),
				}
				con.Name = defaultConstraintName(tableName, con)
				constraints = append(constraints, con)
			case pg_query.ConstrType_CONSTR_UNIQUE:
				cols := parserutil.ParseConstraintColumns(constraint.Keys)
				con := model.Constraint{
					Name:    constraint.Conname,
					Type:    "unique",
					Columns: cols,
				}
				con.Name = defaultConstraintName(tableName, con)
				constraints = append(constraints, con)
			case pg_query.ConstrType_CONSTR_CHECK:
				con := model.Constraint{
					Name:       constraint.Conname,
					Type:       "check",
					Expression: parserutil.DeparseNode(constraint.RawExpr),
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
