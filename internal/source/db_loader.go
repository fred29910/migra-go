package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/introspect"
	"github.com/fred29910/migra-go/internal/model"
)

// DBLoader implements Loader for database connections.
type DBLoader struct{}

// Match returns true if the source is a database connection string.
func (l *DBLoader) Match(source string) bool {
	lowerSource := strings.ToLower(source)
	return strings.HasPrefix(lowerSource, "postgres://") || strings.HasPrefix(lowerSource, "mysql://")
}

// Load loads schema from a database connection string.
// Returns the loaded schema, any parsing errors, and any fatal error.
func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	introspectOpt := introspect.LoadOptions{
		Schemas: opt.Schemas,
	}
	schema, err := introspect.LoadFromDB(ctx, source, introspectOpt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load schema from database: %w", err)
	}
	return schema, nil, nil
}
