package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// loadEnumTypes loads enum types from pg_type and pg_enum
func loadEnumTypes(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT t.typname, e.enumlabel, e.enumsortorder
	FROM pg_type t
	JOIN pg_enum e ON t.oid = e.enumtypid
	JOIN pg_namespace n ON n.oid = t.typnamespace
	WHERE n.nspname = $1 AND t.typtype = 'e'
	ORDER BY t.typname, e.enumsortorder`

	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query enum types: %w", err)
	}
	defer rows.Close()

	// Group labels by type name
	type enumInfo struct {
		labels []string
	}
	enumMap := make(map[string]*enumInfo)

	var (
		typeName  string
		label     string
		sortOrder float32
	)

	for rows.Next() {
		err := rows.Scan(&typeName, &label, &sortOrder)
		if err != nil {
			return fmt.Errorf("scan enum row: %w", err)
		}

		if _, exists := enumMap[typeName]; !exists {
			enumMap[typeName] = &enumInfo{
				labels: make([]string, 0),
			}
		}
		enumMap[typeName].labels = append(enumMap[typeName].labels, label)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate enum rows: %w", err)
	}

	// Convert to model.EnumType
	for name, info := range enumMap {
		// Labels are already sorted by enumsortorder
		ns.Types[name] = &model.EnumType{
			Name:   name,
			Labels: info.labels,
		}
	}

	return nil
}
