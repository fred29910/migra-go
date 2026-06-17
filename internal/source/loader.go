package source

import (
	"context"

	"github.com/fred29910/migra-go/internal/model"
)

// LoadOptions contains extensible options for schema loading.
type LoadOptions struct {
	// Schemas to load (for database sources)
	Schemas []string
	// Strict mode (fail on parsing errors)
	Strict bool
}

// LoaderPriority defines priority levels for loader ordering.
// Higher priority loaders are matched first.
type LoaderPriority int

const (
	LoaderPriorityLowest  LoaderPriority = 10
	LoaderPriorityDefault LoaderPriority = 50
	LoaderPriorityHighest LoaderPriority = 100
)

// Loader defines the interface for loading schema sources.
type Loader interface {
	// Match returns true if this loader can handle the given source.
	Match(source string) bool
	// Load loads schema from the given source with options.
	Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
	// Priority returns the loader's priority level.
	// Higher priority loaders are matched first during registry dispatch.
	Priority() LoaderPriority
}
