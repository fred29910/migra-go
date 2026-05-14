package source

import (
	"context"
	"os"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// DirectoryLoader implements Loader for directory sources.
// It recursively scans all .sql files in a directory, parses each file
// using SQLFileLoader, and merges them into a single Schema.
type DirectoryLoader struct {
	fileLoader *SQLFileLoader
}

// Match returns true if source is an existing directory.
func (l *DirectoryLoader) Match(source string) bool {
	// Skip sources that other loaders handle
	lower := strings.ToLower(source)
	if strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.HasPrefix(lower, "pg://") ||
		strings.HasPrefix(lower, "file://") ||
		strings.HasSuffix(lower, ".sql") {
		return false
	}
	info, err := os.Stat(source)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Load recursively scans all .sql files in the directory, parses each file,
// and merges them into a single Schema.
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: implement in next task
	_ = ctx
	_ = source
	_ = opt
	return nil, nil, nil
}
