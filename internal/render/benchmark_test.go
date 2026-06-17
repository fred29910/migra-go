package render

import (
	"context"
	"fmt"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func BenchmarkRenderAll(b *testing.B) {
	sizes := []int{10, 50, 200}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("ops_%d", size), func(b *testing.B) {
			ops := generateOps(size)
			r := NewRenderer()
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := r.RenderAll(ctx, ops)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func generateOps(n int) []diff.Operation {
	ops := make([]diff.Operation, n)
	for i := 0; i < n; i++ {
		tbl := &model.Table{Name: fmt.Sprintf("table_%d", i), Schema: "public"}
		ops[i] = diff.NewAddTableOp("public", tbl.Name, tbl)
	}
	return ops
}
