package indexdef

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ParseElement parses a single index element SQL string (e.g., "email COLLATE \"C\" text_pattern_ops DESC NULLS LAST")
// into a model.IndexElem struct.
func ParseElement(elemSQL string) (model.IndexElem, error) {
	sql := "CREATE INDEX __migra_idx ON __migra_table USING btree (" + elemSQL + ")"
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: %w", elemSQL, err)
	}
	if len(tree.Stmts) != 1 {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: expected one statement", elemSQL)
	}
	stmt := tree.Stmts[0].Stmt.GetIndexStmt()
	if stmt == nil || len(stmt.IndexParams) != 1 {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: expected one index element", elemSQL)
	}
	return FromNode(stmt.IndexParams[0])
}

// FromNode converts a pg_query IndexElem node into a model.IndexElem.
func FromNode(node *pg_query.Node) (model.IndexElem, error) {
	elem := node.GetIndexElem()
	if elem == nil {
		return model.IndexElem{}, fmt.Errorf("expected IndexElem, got %T", node)
	}

	result := model.IndexElem{}

	if elem.Name != "" {
		result.Name = elem.Name
	}

	if elem.Expr != nil {
		result.Expr = parserutil.DeparseNode(elem.Expr)
	}

	if elem.Indexcolname != "" {
		result.IndexColName = elem.Indexcolname
	}

	switch elem.Ordering {
	case pg_query.SortByDir_SORTBY_DEFAULT:
		result.Ordering = "default"
	case pg_query.SortByDir_SORTBY_ASC:
		result.Ordering = "ASC"
	case pg_query.SortByDir_SORTBY_DESC:
		result.Ordering = "DESC"
	}

	switch elem.NullsOrdering {
	case pg_query.SortByNulls_SORTBY_NULLS_DEFAULT:
		result.NullsOrdering = "default"
	case pg_query.SortByNulls_SORTBY_NULLS_FIRST:
		result.NullsOrdering = "FIRST"
	case pg_query.SortByNulls_SORTBY_NULLS_LAST:
		result.NullsOrdering = "LAST"
	}

	if len(elem.Opclass) > 0 {
		result.Opclass = joinStringNodes(elem.Opclass)
	}

	if len(elem.Collation) > 0 {
		result.Collation = joinStringNodes(elem.Collation)
	}

	return result, nil
}

func joinStringNodes(nodes []*pg_query.Node) string {
	parts := make([]string, 0, len(nodes))
	for _, item := range nodes {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	return strings.Join(parts, ".")
}
