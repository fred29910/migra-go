package source

import (
	"context"
	"fmt"
	"sort"

	"github.com/fred29910/migra-go/internal/errors"
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

// Register adds a loader to the registry and re-sorts all loaders by
// priority (descending). Higher priority loaders are matched first.
func (r *Registry) Register(loader Loader) {
	r.loaders = append(r.loaders, loader)
	sort.SliceStable(r.loaders, func(i, j int) bool {
		return r.loaders[i].Priority() > r.loaders[j].Priority()
	})
}

// Load finds a matching loader and delegates the load operation.
func (r *Registry) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	for _, loader := range r.loaders {
		if loader.Match(source) {
			return loader.Load(ctx, source, opt)
		}
	}
	return nil, nil, fmt.Errorf("load schema: %w", errors.ErrNotFound)
}
