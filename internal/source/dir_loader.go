package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// Load recursively scans all .sql files in the directory, parses each file,
// and merges them into a single Schema.
// If any file fails to parse, the entire load fails (even in non-strict mode).
// If duplicate table/enum names are found across files, the load fails.
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

	merged := model.NewSchema()
	allErrs := make([]error, 0)
	seenTables := make(map[string]string)
	seenEnums := make(map[string]string)

	fl := l.fileLoader
	if fl == nil {
		fl = &SQLFileLoader{}
	}

	for _, file := range files {
		// Always use strict mode for individual files so that parse errors
		// are returned as fatal errors rather than silently swallowed.
		strictOpt := LoadOptions{Strict: true}
		schema, errs, loadErr := fl.Load(ctx, file, strictOpt)
		if loadErr != nil {
			return nil, nil, fmt.Errorf("failed to parse %s: %w", file, loadErr)
		}
		allErrs = append(allErrs, errs...)

		if schema == nil {
			continue
		}

		for nsName, ns := range schema.Schemas {
			mergedNs := merged.GetOrCreateNamespace(nsName)

			for tableName, table := range ns.Tables {
				key := nsName + "." + tableName
				if firstFile, exists := seenTables[key]; exists {
					return nil, nil, fmt.Errorf(
						"duplicate table '%s' found in %s (first defined in %s)",
						tableName, file, firstFile,
					)
				}
				seenTables[key] = file
				mergedNs.Tables[tableName] = table
			}

			for typeName, enumType := range ns.Types {
				key := nsName + "." + typeName
				if firstFile, exists := seenEnums[key]; exists {
					return nil, nil, fmt.Errorf(
						"duplicate enum '%s' found in %s (first defined in %s)",
						typeName, file, firstFile,
					)
				}
				seenEnums[key] = file
				mergedNs.Types[typeName] = enumType
			}
		}
	}

	return merged, allErrs, nil
}

// stripFileScheme removes the "file://" prefix from s if present.
func stripFileScheme(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "file://") {
		return s[len("file://"):]
	}
	return s
}
