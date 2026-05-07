package source

import (
	"context"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// SQLFileLoader implements Loader for SQL file sources.
type SQLFileLoader struct{}

// Match returns true if the source is a SQL file path or URL.
func (l *SQLFileLoader) Match(source string) bool {
	lowerSource := strings.ToLower(source)
	return strings.HasPrefix(lowerSource, "file://") || strings.HasSuffix(lowerSource, ".sql")
}

// Load loads schema from a SQL file.
// Returns the loaded schema, any parsing errors, and any fatal error.
func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: 从 cmd/migra/diff.go 的 loadFromSQLFile 迁移逻辑
	return nil, nil, nil
}
