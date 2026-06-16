package source

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser"
)

// SQLFileLoader implements Loader for SQL file sources.
type SQLFileLoader struct{}

// Priority returns the lowest priority level so SQL files are matched last
// (only after DB and directory loaders have been ruled out).
func (l *SQLFileLoader) Priority() LoaderPriority { return LoaderPriorityLowest }

// Match returns true if the source is a SQL file path or URL.
func (l *SQLFileLoader) Match(source string) bool {
	lowerSource := strings.ToLower(source)
	return strings.HasPrefix(lowerSource, "file://") || strings.HasSuffix(lowerSource, ".sql")
}

// Load loads schema from a SQL file.
// Returns the loaded schema, any parsing errors, and any fatal error.
func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// Remove file:// prefix if present (case-insensitive)
	path := source
	if strings.HasPrefix(strings.ToLower(source), "file://") {
		path = source[len("file://"):]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	p := parser.NewParser()
	schema, parseErr := p.ParseSQL(string(data))
	if parseErr != nil && opt.Strict {
		return nil, nil, parseErr
	}
	return schema, nil, nil
}
