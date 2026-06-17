package diff

import (
	"context"
	"fmt"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

// BenchmarkDiffSmall benchmarks diff with a small schema (1 table, 10 columns)
func BenchmarkDiffSmall(b *testing.B) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	for i := 0; i < 10; i++ {
		sourceTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("col_%d", i),
			DataType: "text",
		})
	}
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	for i := 0; i < 10; i++ {
		targetTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("col_%d", i),
			DataType: "text",
		})
	}
	targetNs.Tables["users"] = targetTable

	d := NewDiffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Diff(context.Background(), source, target)
	}
}

// BenchmarkDiffLarge benchmarks diff with a large schema (100 tables × 20 columns)
func BenchmarkDiffLarge(b *testing.B) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	for t := 0; t < 100; t++ {
		tableName := fmt.Sprintf("table_%d", t)
		table := model.NewTable("public", tableName)
		for c := 0; c < 20; c++ {
			table.AddColumn(&model.Column{
				Name:     fmt.Sprintf("col_%d", c),
				DataType: "text",
			})
		}
		sourceNs.Tables[tableName] = table
	}

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	for t := 0; t < 100; t++ {
		tableName := fmt.Sprintf("table_%d", t)
		table := model.NewTable("public", tableName)
		for c := 0; c < 20; c++ {
			table.AddColumn(&model.Column{
				Name:     fmt.Sprintf("col_%d", c),
				DataType: "text",
			})
		}
		targetNs.Tables[tableName] = table
	}

	d := NewDiffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Diff(context.Background(), source, target)
	}
}

// BenchmarkDiffWithChanges benchmarks diff with schema changes (add/drop columns)
func BenchmarkDiffWithChanges(b *testing.B) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	// 30 common columns
	for i := 0; i < 30; i++ {
		sourceTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("common_%d", i),
			DataType: "text",
		})
	}
	// 10 source-only columns
	for i := 0; i < 10; i++ {
		sourceTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("src_%d", i),
			DataType: "text",
		})
	}
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	// 30 common columns
	for i := 0; i < 30; i++ {
		targetTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("common_%d", i),
			DataType: "text",
		})
	}
	// 10 target-only columns
	for i := 0; i < 10; i++ {
		targetTable.AddColumn(&model.Column{
			Name:     fmt.Sprintf("tgt_%d", i),
			DataType: "text",
		})
	}
	targetNs.Tables["users"] = targetTable

	d := NewDiffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Diff(context.Background(), source, target)
	}
}

// BenchmarkRenameDetection benchmarks the O(n²) rename detection algorithm
func BenchmarkRenameDetection(b *testing.B) {
	sizes := []int{5, 10, 20, 50}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			source := model.NewSchema()
			sourceNs := source.GetOrCreateNamespace("public")
			sourceTable := model.NewTable("public", "users")
			// Common columns
			for i := 0; i < 10; i++ {
				sourceTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("common_%d", i),
					DataType: "text",
				})
			}
			// Source-only columns (to be "dropped")
			for i := 0; i < size; i++ {
				sourceTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("src_%d", i),
					DataType: "text",
				})
			}
			sourceNs.Tables["users"] = sourceTable

			target := model.NewSchema()
			targetNs := target.GetOrCreateNamespace("public")
			targetTable := model.NewTable("public", "users")
			// Common columns
			for i := 0; i < 10; i++ {
				targetTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("common_%d", i),
					DataType: "text",
				})
			}
			// Target-only columns (to be "added")
			for i := 0; i < size; i++ {
				targetTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("tgt_%d", i),
					DataType: "text",
				})
			}
			targetNs.Tables["users"] = targetTable

			d := NewDiffer()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d.Diff(context.Background(), source, target)
			}
		})
	}
}

// BenchmarkConstraintComparison benchmarks constraint comparison with reflect.DeepEqual
func BenchmarkConstraintComparison(b *testing.B) {
	sizes := []int{10, 20, 50}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			source := model.NewSchema()
			sourceNs := source.GetOrCreateNamespace("public")
			sourceTable := model.NewTable("public", "users")
			// Add columns for foreign key references
			sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer"})
			for i := 0; i < size; i++ {
				sourceTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("ref_%d", i),
					DataType: "integer",
				})
			}
			// Add constraints
			for i := 0; i < size; i++ {
				sourceTable.Constraints[fmt.Sprintf("fk_%d", i)] = &model.Constraint{
					Name:       fmt.Sprintf("fk_%d", i),
					Type:       "foreign_key",
					Table:      "users",
					Columns:    []string{fmt.Sprintf("ref_%d", i)},
					RefSchema:  "public",
					RefTable:   "other_table",
					RefColumns: []string{"id"},
				}
			}
			sourceNs.Tables["users"] = sourceTable

			target := model.NewSchema()
			targetNs := target.GetOrCreateNamespace("public")
			targetTable := model.NewTable("public", "users")
			// Add columns for foreign key references
			targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer"})
			for i := 0; i < size; i++ {
				targetTable.AddColumn(&model.Column{
					Name:     fmt.Sprintf("ref_%d", i),
					DataType: "integer",
				})
			}
			// Add same constraints
			for i := 0; i < size; i++ {
				targetTable.Constraints[fmt.Sprintf("fk_%d", i)] = &model.Constraint{
					Name:       fmt.Sprintf("fk_%d", i),
					Type:       "foreign_key",
					Table:      "users",
					Columns:    []string{fmt.Sprintf("ref_%d", i)},
					RefSchema:  "public",
					RefTable:   "other_table",
					RefColumns: []string{"id"},
				}
			}
			targetNs.Tables["users"] = targetTable

			d := NewDiffer()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d.Diff(context.Background(), source, target)
			}
		})
	}
}

// BenchmarkSameConstraintContent benchmarks the sameConstraintContent function directly
func BenchmarkSameConstraintContent(b *testing.B) {
	sizes := []int{1, 5, 10, 20}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			columns := make([]string, size)
			for i := 0; i < size; i++ {
				columns[i] = fmt.Sprintf("col_%d", i)
			}
			constraintA := &model.Constraint{
				Type:       "foreign_key",
				Columns:    columns,
				RefSchema:  "public",
				RefTable:   "other",
				RefColumns: columns,
			}
			constraintB := &model.Constraint{
				Type:       "foreign_key",
				Columns:    columns,
				RefSchema:  "public",
				RefTable:   "other",
				RefColumns: columns,
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sameConstraintContent(constraintA, constraintB)
			}
		})
	}
}

// BenchmarkManyTables benchmarks diff with many tables
func BenchmarkManyTables(b *testing.B) {
	sizes := []int{50, 100, 200}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("tables_%d", size), func(b *testing.B) {
			source := model.NewSchema()
			sourceNs := source.GetOrCreateNamespace("public")
			for t := 0; t < size; t++ {
				tableName := fmt.Sprintf("table_%d", t)
				table := model.NewTable("public", tableName)
				table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
				sourceNs.Tables[tableName] = table
			}

			target := model.NewSchema()
			targetNs := target.GetOrCreateNamespace("public")
			for t := 0; t < size; t++ {
				tableName := fmt.Sprintf("table_%d", t)
				table := model.NewTable("public", tableName)
				table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
				targetNs.Tables[tableName] = table
			}

			d := NewDiffer()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d.Diff(context.Background(), source, target)
			}
		})
	}
}
