package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateIndexHandler handles CREATE INDEX statements.
type CreateIndexHandler struct{}

// Handle converts a pg_query IndexStmt into schema mutations.
func (h *CreateIndexHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.IndexStmt)
	if !ok {
		return nil, fmt.Errorf("CreateIndexHandler: expected pg_nodes.IndexStmt, got %T", node)
	}

	// Parse table name
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	// Parse index name (generate default if empty)
	indexName := ""
	if stmt.Idxname != nil {
		indexName = *stmt.Idxname
	}
	if indexName == "" {
		indexName = generateDefaultIndexName(tableName, stmt.IndexParams)
	}

	// Parse index elements
	elements := make([]model.IndexElem, 0, len(stmt.IndexParams.Items))
	for _, item := range stmt.IndexParams.Items {
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
	if stmt.AccessMethod != nil {
		method = *stmt.AccessMethod
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
func parseIndexElem(node pg_nodes.Node) (model.IndexElem, error) {
	elem, ok := node.(pg_nodes.IndexElem)
	if !ok {
		return model.IndexElem{}, fmt.Errorf("expected IndexElem, got %T", node)
	}

	result := model.IndexElem{}

	// Column name (simple column index)
	if elem.Name != nil {
		result.Name = *elem.Name
	}

	// Expression (expression index)
	if elem.Expr != nil {
		result.Expr = safeDeparse(elem.Expr)
	}

	// Index column name
	if elem.Indexcolname != nil {
		result.IndexColName = *elem.Indexcolname
	}

	// Ordering
	switch elem.Ordering {
	case pg_nodes.SORTBY_DEFAULT:
		result.Ordering = "default"
	case pg_nodes.SORTBY_ASC:
		result.Ordering = "ASC"
	case pg_nodes.SORTBY_DESC:
		result.Ordering = "DESC"
	}

	// Nulls ordering
	switch elem.NullsOrdering {
	case pg_nodes.SORTBY_NULLS_DEFAULT:
		result.NullsOrdering = "default"
	case pg_nodes.SORTBY_NULLS_FIRST:
		result.NullsOrdering = "FIRST"
	case pg_nodes.SORTBY_NULLS_LAST:
		result.NullsOrdering = "LAST"
	}

	return result, nil
}

// safeDeparse safely calls Deparse() method, recovering from panic
func safeDeparse(node interface{}) string {
	if d, ok := node.(interface{ Deparse() string }); ok {
		var result string
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Deparse not implemented, use fmt as fallback
					result = fmt.Sprintf("%v", node)
				}
			}()
			result = d.Deparse()
		}()
		return result
	}
	return fmt.Sprintf("%v", node)
}

// generateDefaultIndexName generates a default index name
func generateDefaultIndexName(tableName string, indexParams pg_nodes.List) string {
	// Simple heuristic: use first column name or "expr" for expression indexes
	for _, item := range indexParams.Items {
		if elem, ok := item.(pg_nodes.IndexElem); ok {
			if elem.Name != nil {
				return fmt.Sprintf("%s_%s_idx", tableName, *elem.Name)
			}
			if elem.Expr != nil {
				return fmt.Sprintf("%s_expr_idx", tableName)
			}
		}
	}
	return fmt.Sprintf("%s_idx", tableName)
}
