package source

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

type Registry struct {
	loaders []Loader
}

func NewRegistry() *Registry {
	return &Registry{
		loaders: make([]Loader, 0),
	}
}

func (r *Registry) Register(loader Loader) {
	r.loaders = append(r.loaders, loader)
}

func (r *Registry) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	for _, loader := range r.loaders {
		if loader.Match(source) {
			return loader.Load(ctx, source, opt)
		}
	}
	return nil, nil, fmt.Errorf("no loader found for source: %s", source)
}
