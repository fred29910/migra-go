package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// RenameStmtHandler handles RENAME statements, specifically RENAME COLUMN.
// In pg_query_go v6, ALTER TABLE ... RENAME COLUMN is parsed as a RenameStmt
// (not as AlterTableStmt), with rename_type=OBJECT_COLUMN.
type RenameStmtHandler struct{}

// Handle converts a pg_query RenameStmt into schema mutations.
// Only handles RENAME COLUMN (rename_type == OBJECT_COLUMN).
func (h *RenameStmtHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetRenameStmt()
	if stmt == nil {
		return nil, fmt.Errorf("RenameStmtHandler: expected RenameStmt, got %T", node)
	}
	if stmt.RenameType != pg_query.ObjectType_OBJECT_COLUMN {
		return nil, nil
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)
	if stmt.Subname == "" {
		return nil, fmt.Errorf("RENAME COLUMN missing old column name")
	}
	if stmt.Newname == "" {
		return nil, fmt.Errorf("RENAME COLUMN missing new column name")
	}
	return []SchemaMutation{
		RenameColumnMutation{
			Schema:  schemaName,
			Table:   tableName,
			OldName: stmt.Subname,
			NewName: stmt.Newname,
		},
	}, nil
}
