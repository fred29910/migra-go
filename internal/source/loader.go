package source

import (
	"context"

	"github.com/fred29910/migra-go/internal/model"
)

// LoadOptions contains extensible options for schema loading.
type LoadOptions struct{}

// Loader defines the interface for loading schema sources.
type Loader interface {
	// Match returns true if this loader can handle the given source.
	Match(source string) bool
	// Load loads schema from the given source with options.
	Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
}
