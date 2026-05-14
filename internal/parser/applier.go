package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// MutationApplier applies SchemaMutation values to a Schema.
// It validates consistency and collects errors without halting on the first one.
type MutationApplier struct {
	errors []error
}

// Apply applies mutations to the given schema in order.
// Returns nil if all mutations succeeded, or an aggregate error.
func (a *MutationApplier) Apply(schema *model.Schema, mutations []SchemaMutation) error {
	if a.errors == nil {
		a.errors = make([]error, 0)
	} else {
		a.errors = a.errors[:0]
	}
	for _, mut := range mutations {
		if err := mut.Apply(schema); err != nil {
			a.errors = append(a.errors, &MutationError{Mutation: mut, Cause: err})
		}
	}
	if len(a.errors) > 0 {
		first := a.errors[0]
		return fmt.Errorf("applied %d mutations with %d errors, first: %w",
			len(mutations), len(a.errors), first)
	}
	return nil
}

// MutationError wraps an error with mutation context.
type MutationError struct {
	Mutation SchemaMutation
	Cause    error
}

func (e *MutationError) Error() string {
	t := e.Mutation.Target()
	return fmt.Sprintf("mutation %s on %s/%s: %v", e.Mutation.Kind(), t.Schema, t.Name, e.Cause)
}

func (e *MutationError) Unwrap() error {
	return e.Cause
}
