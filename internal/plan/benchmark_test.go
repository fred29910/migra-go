package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func BenchmarkTopoSort(b *testing.B) {
	sizes := []int{10, 50, 200}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("ops_%d", size), func(b *testing.B) {
			ops := generateOpsWithDeps(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := TopoSort(context.Background(), ops)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func generateOpsWithDeps(n int) []diff.Operation {
	ops := make([]diff.Operation, n)
	for i := 0; i < n; i++ {
		tbl := &model.Table{Name: fmt.Sprintf("table_%d", i), Schema: "public"}
		ops[i] = diff.NewAddTableOp("public", tbl.Name, tbl)
	}
	return ops
}
