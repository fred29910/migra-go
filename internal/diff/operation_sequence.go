package diff

import "github.com/fred29910/migra-go/internal/model"

type CreateSequenceOp struct {
	baseOperation
	Schema   string
	Sequence *model.Sequence
}

func NewCreateSequenceOp(schema string, seq *model.Sequence) *CreateSequenceOp {
	return &CreateSequenceOp{
		baseOperation: baseOperation{
			kind:      KindCreateSequence,
			objectKey: model.NewObjectKey(schema, seq.Name, model.KindSequence),
		},
		Schema:   schema,
		Sequence: seq,
	}
}

func (op *CreateSequenceOp) IsDestructive() bool { return false }

type DropSequenceOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropSequenceOp(schema, name string) *DropSequenceOp {
	return &DropSequenceOp{
		baseOperation: baseOperation{
			kind:      KindDropSequence,
			objectKey: model.NewObjectKey(schema, name, model.KindSequence),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropSequenceOp) IsDestructive() bool { return true }

type AlterSequenceOp struct {
	baseOperation
	Schema string
	From   *model.Sequence
	To     *model.Sequence
}

func NewAlterSequenceOp(schema string, from, to *model.Sequence) *AlterSequenceOp {
	return &AlterSequenceOp{
		baseOperation: baseOperation{
			kind:      KindAlterSequence,
			objectKey: model.NewObjectKey(schema, to.Name, model.KindSequence),
		},
		Schema: schema,
		From:   from,
		To:     to,
	}
}

func (op *AlterSequenceOp) IsDestructive() bool { return false }
