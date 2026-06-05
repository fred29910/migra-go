package render

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/util"
)

func renderCreateSequence(r *Renderer, op *diff.CreateSequenceOp) string {
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

func renderDropSequence(r *Renderer, op *diff.DropSequenceOp) string {
	return fmt.Sprintf("-- op: drop_sequence risk:high\nDROP SEQUENCE %s%s;",
		ifExistsPrefix(r.useIfExists), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

func renderAlterSequence(r *Renderer, op *diff.AlterSequenceOp) string {
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
