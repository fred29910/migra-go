package normalize

import (
	"fmt"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

func BenchmarkCanonicalizeSchema(b *testing.B) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	for t := 0; t < 100; t++ {
		table := model.NewTable("public", fmt.Sprintf("table_%d", t))
		for c := 0; c < 20; c++ {
			table.AddColumn(&model.Column{
				Name:        fmt.Sprintf("col_%d", c),
				DataType:    "integer",
				DefaultExpr: strPtr("0"),
			})
		}
		ns.Tables[table.Name] = table
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CanonicalizeSchema(schema)
	}
}

func BenchmarkNormalizeDataType(b *testing.B) {
	types := []string{
		"int4", "int8", "int2", "bool",
		"character varying(255)", "character(10)",
		"timestamp without time zone", "timestamp with time zone",
		"integer", "text", "boolean", "varchar(100)",
		"numeric(10,2)", "int4[]", "text[]",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, typ := range types {
			util.NormalizeDataType(typ)
		}
	}
}

func strPtr(s string) *string { return &s }
