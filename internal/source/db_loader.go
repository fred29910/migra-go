package source

import (
	"context"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// DBLoader implements Loader for database connections.
type DBLoader struct{}

func (l *DBLoader) Match(source string) bool {
	lowerSource := strings.ToLower(source)
	return strings.HasPrefix(lowerSource, "postgres://") || strings.HasPrefix(lowerSource, "mysql://")
}

func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: 从 cmd/migra/diff.go 的 loadFromDB 迁移逻辑
	return nil, nil, nil
}
