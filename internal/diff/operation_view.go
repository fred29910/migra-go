package diff

import "github.com/fred29910/migra-go/internal/model"

type CreateViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

func NewCreateViewOp(schema string, view *model.View) *CreateViewOp {
	return &CreateViewOp{
		baseOperation: baseOperation{
			kind:      KindCreateView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema: schema,
		View:   view,
	}
}

func (op *CreateViewOp) IsDestructive() bool { return false }

type DropViewOp struct {
	baseOperation
	Schema     string
	Name       string
	IsRecreate bool
}

func NewDropViewOp(schema, name string) *DropViewOp {
	return &DropViewOp{
		baseOperation: baseOperation{
			kind:      KindDropView,
			objectKey: model.NewObjectKey(schema, name, model.KindView),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropViewOp) IsDestructive() bool { return !op.IsRecreate }

type ReplaceViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

func NewReplaceViewOp(schema string, view *model.View) *ReplaceViewOp {
	return &ReplaceViewOp{
		baseOperation: baseOperation{
			kind:      KindReplaceView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema: schema,
		View:   view,
	}
}

func (op *ReplaceViewOp) IsDestructive() bool { return false }

type CreateMaterializedViewOp struct {
	baseOperation
	Schema           string
	MaterializedView *model.View
}

func NewCreateMaterializedViewOp(schema string, view *model.View) *CreateMaterializedViewOp {
	return &CreateMaterializedViewOp{
		baseOperation: baseOperation{
			kind:      KindCreateMaterializedView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema:           schema,
		MaterializedView: view,
	}
}

func (op *CreateMaterializedViewOp) IsDestructive() bool { return false }

type DropMaterializedViewOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropMaterializedViewOp(schema, name string) *DropMaterializedViewOp {
	return &DropMaterializedViewOp{
		baseOperation: baseOperation{
			kind:      KindDropMaterializedView,
			objectKey: model.NewObjectKey(schema, name, model.KindView),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropMaterializedViewOp) IsDestructive() bool { return true }
