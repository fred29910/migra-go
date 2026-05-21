package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/indexdef"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateIndexHandler handles CREATE INDEX statements.
type CreateIndexHandler struct{}

// Handle converts a pg_query IndexStmt into schema mutations.
func (h *CreateIndexHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetIndexStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateIndexHandler: expected IndexStmt, got %T", node)
	}

	// Parse table name
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	// Parse index name (generate default if empty)
	indexName := stmt.Idxname
	if indexName == "" {
		indexName = generateDefaultIndexName(tableName, stmt.IndexParams)
	}

	// Parse index elements
	elements := make([]model.IndexElem, 0, len(stmt.IndexParams))
	for _, item := range stmt.IndexParams {
		indexElem, err := parseIndexElem(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse index element: %w", err)
		}
		elements = append(elements, indexElem)
	}

	// Parse WHERE clause (partial index)
	whereClause := ""
	if stmt.WhereClause != nil {
		whereClause = safeDeparse(stmt.WhereClause)
	}

	// Parse access method
	method := "btree" // default
	if stmt.AccessMethod != "" {
		method = stmt.AccessMethod
	}

	index := model.Index{
		Name:         indexName,
		Table:        tableName,
		Elements:     elements,
		Unique:       stmt.Unique,
		Method:       method,
		Primary:      stmt.Primary,
		IsConstraint: stmt.Isconstraint,
		WhereClause:  whereClause,
		Concurrent:   stmt.Concurrent,
		IfNotExists:  stmt.IfNotExists,
	}

	return []SchemaMutation{CreateIndexMutation{
		Schema: schemaName,
		Index:  index,
	}}, nil
}

// parseIndexElem parses a single IndexElem from pg_query node
func parseIndexElem(node *pg_query.Node) (model.IndexElem, error) {
	return indexdef.FromNode(node)
}

// safeDeparse safely converts a pg_query node to its string representation
func safeDeparse(node *pg_query.Node) string {
	return parserutil.DeparseNode(node)
}

// generateDefaultIndexName generates a default index name
func generateDefaultIndexName(tableName string, indexParams []*pg_query.Node) string {
	// Simple heuristic: use first column name or "expr" for expression indexes
	for _, item := range indexParams {
		if elem := item.GetIndexElem(); elem != nil {
			if elem.Name != "" {
				return fmt.Sprintf("%s_%s_idx", tableName, elem.Name)
			}
			if elem.Expr != nil {
				return fmt.Sprintf("%s_expr_idx", tableName)
			}
		}
	}
	return fmt.Sprintf("%s_idx", tableName)
}
