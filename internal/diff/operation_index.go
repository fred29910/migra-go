package diff

import "github.com/fred29910/migra-go/internal/model"

type CreateIndexOp struct {
	baseOperation
	Schema string
	Index  *model.Index
}

func NewCreateIndexOp(schema string, index *model.Index) *CreateIndexOp {
	return &CreateIndexOp{
		baseOperation: baseOperation{
			kind:      KindAddIndex,
			objectKey: model.NewObjectKey(schema, index.Name, model.KindIndex),
		},
		Schema: schema,
		Index:  index,
	}
}

func (op *CreateIndexOp) IsDestructive() bool {
	return false
}

func (op *CreateIndexOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Index.Table, model.KindTable),
	}
}

type DropIndexOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropIndexOp(schema, name string) *DropIndexOp {
	return &DropIndexOp{
		baseOperation: baseOperation{
			kind:      KindDropIndex,
			objectKey: model.NewObjectKey(schema, name, model.KindIndex),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropIndexOp) IsDestructive() bool {
	return false
}
