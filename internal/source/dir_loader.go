package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser"
)

// DirectoryLoader implements Loader for directory sources.
// It recursively scans all .sql files in a directory, merges their SQL
// content, and parses everything in a single pass. This ensures that
// cross-file DDL dependencies (e.g., CREATE TABLE in one file and
// CREATE INDEX referencing that table in another) are resolved correctly.
type DirectoryLoader struct{}

// Match returns true if source is an existing directory.
func (l *DirectoryLoader) Match(source string) bool {
	lower := strings.ToLower(source)
	// DB URLs are handled by DBLoader (registered before us in the
	// registry), but we defensively reject them here in case
	// registration order changes in the future.
	if strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.HasPrefix(lower, "pg://") {
		return false
	}
	path := stripFileScheme(source)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Load recursively scans all .sql files in the directory, merges their
// contents, then parses the combined SQL in a single pass.
// This approach (方案 B) resolves cross-file DDL dependencies that would
// fail when parsing each file in isolation.
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	sourcePath := stripFileScheme(source)

	var files []string
	err := filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan directory %s: %w", sourcePath, err)
	}

	sort.Strings(files)

	if len(files) == 0 {
		return model.NewSchema(), []error{fmt.Errorf("no .sql files found in directory: %s", sourcePath)}, nil
	}

	// Merge all SQL file contents into a single string.
	// This way, a single Parser sees all DDL statements in order, so
	// cross-file dependencies (tables referenced by indexes, foreign keys, etc.)
	// are resolved within a single parse session.
	var combinedSQL strings.Builder
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, fmt.Errorf("failed to read %s: %w", file, readErr)
		}
		if combinedSQL.Len() > 0 {
			combinedSQL.WriteString("\n")
		}
		combinedSQL.Write(data)
	}

	// Always treat parse errors as fatal for merged parsing: any file
	// with invalid SQL corrupts the entire combined input, so partial
	// recovery is not meaningful.
	p := parser.NewParser()
	schema, parseErr := p.ParseSQL(combinedSQL.String())
	errs := p.Errors()
	if parseErr != nil {
		return nil, errs, fmt.Errorf("failed to parse directory contents: %w", parseErr)
	}

	return schema, errs, nil
}

// stripFileScheme removes the "file://" prefix from s if present.
func stripFileScheme(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "file://") {
		return s[len("file://"):]
	}
	return s
}
