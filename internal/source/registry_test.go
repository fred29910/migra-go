package source

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/fred29910/migra-go/internal/errors"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLoader is a test double that records whether Match/Load were called.
type mockLoader struct {
	matchCalled bool
	loadCalled  bool
	matchResult bool
	loadSchema  *model.Schema
	loadErrs    []error
	loadErr     error
}

func (m *mockLoader) Match(source string) bool {
	m.matchCalled = true
	return m.matchResult
}

func (m *mockLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	m.loadCalled = true
	return m.loadSchema, m.loadErrs, m.loadErr
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)
	assert.NotNil(t, r.loaders)
	assert.Empty(t, r.loaders)
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	ml1 := &mockLoader{}
	r.Register(ml1)
	assert.Len(t, r.loaders, 1)

	ml2 := &mockLoader{}
	r.Register(ml2)
	assert.Len(t, r.loaders, 2)
}

func TestRegistry_Load_FirstMatch(t *testing.T) {
	r := NewRegistry()

	expectedSchema := model.NewSchema()
	expectedErrs := []error{errors.New("parse warning")}

	ml1 := &mockLoader{
		matchResult: true,
		loadSchema:  expectedSchema,
		loadErrs:    expectedErrs,
	}
	ml2 := &mockLoader{
		matchResult: true,
		loadSchema:  model.NewSchema(),
	}

	r.Register(ml1)
	r.Register(ml2)

	schema, errs, err := r.Load(context.TODO(), "some-source", LoadOptions{})
	require.NoError(t, err)
	assert.True(t, ml1.matchCalled)
	assert.True(t, ml1.loadCalled)
	assert.False(t, ml2.matchCalled, "second loader should not be consulted after first match")
	assert.Equal(t, expectedSchema, schema)
	assert.Equal(t, expectedErrs, errs)
}

func TestRegistry_Load_SecondMatch(t *testing.T) {
	r := NewRegistry()

	expectedSchema := model.NewSchema()

	ml1 := &mockLoader{matchResult: false}
	ml2 := &mockLoader{
		matchResult: true,
		loadSchema:  expectedSchema,
	}

	r.Register(ml1)
	r.Register(ml2)

	schema, errs, err := r.Load(context.TODO(), "some-source", LoadOptions{})
	require.NoError(t, err)
	assert.True(t, ml1.matchCalled)
	assert.False(t, ml1.loadCalled, "first loader should not be called when Match returns false")
	assert.True(t, ml2.matchCalled)
	assert.True(t, ml2.loadCalled)
	assert.Equal(t, expectedSchema, schema)
	assert.Nil(t, errs)
}

func TestRegistry_Load_NoMatch(t *testing.T) {
	r := NewRegistry()

	ml := &mockLoader{matchResult: false}
	r.Register(ml)

	schema, errs, err := r.Load(context.TODO(), "unknown-source", LoadOptions{})
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Nil(t, errs)
	assert.True(t, errors.Is(err, appErrors.ErrNotFound))
}

func TestRegistry_Load_EmptyRegistry(t *testing.T) {
	r := NewRegistry()

	schema, errs, err := r.Load(context.TODO(), "anything", LoadOptions{})
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Nil(t, errs)
	assert.True(t, errors.Is(err, appErrors.ErrNotFound))
}

func TestRegistry_Load_LoaderReturnsError(t *testing.T) {
	r := NewRegistry()

	loadErr := errors.New("connection refused")
	ml := &mockLoader{
		matchResult: true,
		loadErr:     loadErr,
	}
	r.Register(ml)

	schema, errs, err := r.Load(context.TODO(), "pg://localhost/db", LoadOptions{})
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Nil(t, errs)
	assert.Equal(t, loadErr, err)
}

func TestRegistry_Load_PassesOptions(t *testing.T) {
	r := NewRegistry()

	// We can't directly inspect the options passed through Load without
	// instrumenting the mock further, but we can verify the happy path
	// with non-default options doesn't error.
	ml := &mockLoader{
		matchResult: true,
		loadSchema:  model.NewSchema(),
	}
	r.Register(ml)

	schema, _, err := r.Load(context.TODO(), "test", LoadOptions{
		Schemas: []string{"public", "app"},
		Strict:  true,
	})
	require.NoError(t, err)
	assert.NotNil(t, schema)
}
