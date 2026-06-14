package diff

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

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

func (op *CreateSequenceOp) RenderString(ctx RenderContext) string {
	seq := op.Sequence
	parts := []string{"CREATE SEQUENCE " + util.QuoteQualifiedIdentifier(op.Schema, seq.Name)}
	if seq.DataType != "" {
		parts = append(parts, "AS "+seq.DataType)
	}
	if seq.StartValue != 0 {
		parts = append(parts, fmt.Sprintf("START WITH %d", seq.StartValue))
	}
	if seq.IncrementBy != 0 {
		parts = append(parts, fmt.Sprintf("INCREMENT BY %d", seq.IncrementBy))
	}
	if seq.MinValue != 0 {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", seq.MinValue))
	}
	if seq.MaxValue != 0 {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", seq.MaxValue))
	}
	if seq.CacheSize != 0 && seq.CacheSize != 1 {
		parts = append(parts, fmt.Sprintf("CACHE %d", seq.CacheSize))
	}
	if seq.Cycle {
		parts = append(parts, "CYCLE")
	}
	return "-- op: create_sequence risk:low\n" + strings.Join(parts, " ") + ";"
}

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

func (op *DropSequenceOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_sequence risk:high\nDROP SEQUENCE %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

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

func (op *AlterSequenceOp) RenderString(ctx RenderContext) string {
	seq := op.To
	parts := []string{"ALTER SEQUENCE " + util.QuoteQualifiedIdentifier(op.Schema, seq.Name)}
	if seq.DataType != "" && seq.DataType != op.From.DataType {
		parts = append(parts, "AS "+seq.DataType)
	}
	if seq.StartValue != 0 && seq.StartValue != op.From.StartValue {
		parts = append(parts, fmt.Sprintf("START WITH %d", seq.StartValue))
	}
	if seq.IncrementBy != 0 && seq.IncrementBy != op.From.IncrementBy {
		parts = append(parts, fmt.Sprintf("INCREMENT BY %d", seq.IncrementBy))
	}
	if seq.MinValue != 0 && seq.MinValue != op.From.MinValue {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", seq.MinValue))
	}
	if seq.MaxValue != 0 && seq.MaxValue != op.From.MaxValue {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", seq.MaxValue))
	}
	if seq.CacheSize != 0 && seq.CacheSize != op.From.CacheSize {
		parts = append(parts, fmt.Sprintf("CACHE %d", seq.CacheSize))
	}
	if seq.Cycle != op.From.Cycle {
		if seq.Cycle {
			parts = append(parts, "CYCLE")
		} else {
			parts = append(parts, "NO CYCLE")
		}
	}
	return "-- op: alter_sequence risk:low\n" + strings.Join(parts, " ") + ";"
}
