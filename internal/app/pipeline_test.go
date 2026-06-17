package app

import (
	"context"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeDiff_Immutability(t *testing.T) {
	source := &model.Schema{
		Schemas: map[string]*model.Namespace{
			"public": {
				Name: "public",
				Tables: map[string]*model.Table{
					"users": {
						Name:         "users",
						Columns:      []*model.Column{{Name: "id", DataType: "integer"}},
						ColumnByName: map[string]*model.Column{"id": {Name: "id", DataType: "integer"}},
						ColumnIndex:  map[string]int{"id": 0},
						Constraints:  map[string]*model.Constraint{},
						Indexes:      map[string]*model.Index{},
					},
				},
				Types:      map[string]*model.EnumType{},
				Views:      map[string]*model.View{},
				Sequences:  map[string]*model.Sequence{},
				Extensions: map[string]*model.Extension{},
			},
		},
	}
	target := &model.Schema{
		Schemas: map[string]*model.Namespace{
			"public": {
				Name: "public",
				Tables: map[string]*model.Table{
					"orders": {
						Name:         "orders",
						Columns:      []*model.Column{{Name: "id", DataType: "integer"}},
						ColumnByName: map[string]*model.Column{"id": {Name: "id", DataType: "integer"}},
						ColumnIndex:  map[string]int{"id": 0},
						Constraints:  map[string]*model.Constraint{},
						Indexes:      map[string]*model.Index{},
					},
				},
				Types:      map[string]*model.EnumType{},
				Views:      map[string]*model.View{},
				Sequences:  map[string]*model.Sequence{},
				Extensions: map[string]*model.Extension{},
			},
		},
	}

	sourceCopy := source.Clone()
	targetCopy := target.Clone()

	_, _, err := ComputeDiff(context.Background(), source, target, DiffConfig{})
	require.NoError(t, err)

	assert.Equal(t, sourceCopy, source, "source should not be modified")
	assert.Equal(t, targetCopy, target, "target should not be modified")
}
