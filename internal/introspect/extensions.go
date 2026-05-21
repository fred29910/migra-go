package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

func loadExtensions(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
SELECT e.extname, e.extversion
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE n.nspname = $1`
	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query extensions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		ext := &model.Extension{}
		if err := rows.Scan(&ext.Name, &ext.Version); err != nil {
			return fmt.Errorf("scan extension row: %w", err)
		}
		ns.Extensions[ext.Name] = ext
	}
	return rows.Err()
}
