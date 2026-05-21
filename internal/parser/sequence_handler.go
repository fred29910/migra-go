package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CreateSequenceHandler handles CREATE SEQUENCE statements.
type CreateSequenceHandler struct{}

func (h *CreateSequenceHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateSeqStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateSequenceHandler: expected CreateSeqStmt, got %T", node)
	}
	name, schemaName := parserutil.ParseRelation(stmt.Sequence)
	seq := model.Sequence{
		Name:        name,
		DataType:    "bigint",
		IncrementBy: 1,
		CacheSize:   1,
	}
	applySequenceOptions(&seq, stmt.Options)
	return []SchemaMutation{CreateSequenceMutation{Schema: schemaName, Sequence: seq}}, nil
}

func applySequenceOptions(seq *model.Sequence, options []*pg_query.Node) {
	for _, node := range options {
		def := node.GetDefElem()
		if def == nil {
			continue
		}
		switch strings.ToLower(def.Defname) {
		case "as":
			seq.DataType = sequenceStringValue(def.Arg)
		case "increment":
			seq.IncrementBy = sequenceIntValue(def.Arg)
		case "start":
			seq.StartValue = sequenceIntValue(def.Arg)
		case "minvalue":
			seq.MinValue = sequenceIntValue(def.Arg)
		case "maxvalue":
			seq.MaxValue = sequenceIntValue(def.Arg)
		case "cache":
			seq.CacheSize = sequenceIntValue(def.Arg)
		case "cycle":
			seq.Cycle = true
		}
	}
}

func sequenceIntValue(node *pg_query.Node) int64 {
	if node == nil {
		return 0
	}
	if a := node.GetAConst(); a != nil {
		if i := a.GetIval(); i != nil {
			return int64(i.Ival)
		}
	}
	return 0
}

func sequenceStringValue(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	if tn := node.GetTypeName(); tn != nil {
		return parserutil.ParseTypeName(tn)
	}
	if s := node.GetString_(); s != nil {
		return s.Sval
	}
	return ""
}

// CreateSequenceMutation describes creating a sequence.
type CreateSequenceMutation struct {
	Schema   string
	Sequence model.Sequence
}

func (m CreateSequenceMutation) Kind() MutationKind {
	return MutationKind("create_sequence")
}

func (m CreateSequenceMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Sequence.Name, model.KindSequence)
}

func (m CreateSequenceMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	seq := m.Sequence
	ns.Sequences[seq.Name] = &seq
	return nil
}
