package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

func loadViews(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
SELECT viewname, definition, false AS materialized
FROM pg_views
WHERE schemaname = $1
UNION ALL
SELECT matviewname, definition, true AS materialized
FROM pg_matviews
WHERE schemaname = $1`
	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query views: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, definition string
		var materialized bool
		if err := rows.Scan(&name, &definition, &materialized); err != nil {
			return fmt.Errorf("scan view row: %w", err)
		}
		ns.Views[name] = &model.View{Name: name, Definition: strings.TrimSuffix(strings.TrimSpace(definition), ";"), Materialized: materialized}
	}
	return rows.Err()
}
