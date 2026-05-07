package source

import (
	"context"

	"github.com/fred29910/migra-go/internal/model"
)

type LoadOptions struct{}

type Loader interface {
	Match(source string) bool
	Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
}
