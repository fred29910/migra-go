package source

import (
	"context"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

type SQLFileLoader struct{}

func (l *SQLFileLoader) Match(source string) bool {
	return strings.HasPrefix(source, "file://") || strings.HasSuffix(source, ".sql")
}

func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: 从 cmd/migra/diff.go 的 loadFromSQLFile 迁移逻辑
	return nil, nil, nil
}
