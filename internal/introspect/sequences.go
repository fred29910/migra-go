package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

func loadSequences(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
SELECT sequencename, data_type, start_value, min_value, max_value, increment_by, cycle, cache_size
FROM pg_sequences
WHERE schemaname = $1`
	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query sequences: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		seq := &model.Sequence{}
		if err := rows.Scan(&seq.Name, &seq.DataType, &seq.StartValue, &seq.MinValue, &seq.MaxValue, &seq.IncrementBy, &seq.Cycle, &seq.CacheSize); err != nil {
			return fmt.Errorf("scan sequence row: %w", err)
		}
		ns.Sequences[seq.Name] = seq
	}
	return rows.Err()
}
