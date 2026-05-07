package source

import (
	"context"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

type DBLoader struct{}

func (l *DBLoader) Match(source string) bool {
	return strings.HasPrefix(source, "postgres://") || strings.HasPrefix(source, "mysql://")
}

func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: 从 cmd/migra/diff.go 的 loadFromDB 迁移逻辑
	return nil, nil, nil
}
