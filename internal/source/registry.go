package source

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// Registry manages a collection of loaders and dispatches load requests.
type Registry struct {
	loaders []Loader
}

// NewRegistry creates a new loader registry.
func NewRegistry() *Registry {
	return &Registry{
		loaders: make([]Loader, 0),
	}
}

// Register adds a loader to the registry.
func (r *Registry) Register(loader Loader) {
	r.loaders = append(r.loaders, loader)
}

// Load finds a matching loader and delegates the load operation.
func (r *Registry) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	for _, loader := range r.loaders {
		if loader.Match(source) {
			return loader.Load(ctx, source, opt)
		}
	}
	return nil, nil, fmt.Errorf("no loader found for source: %s", source)
}
