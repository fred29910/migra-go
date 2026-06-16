package parser

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkParseSQL benchmarks SQL parsing with varying schema sizes.
func BenchmarkParseSQL(b *testing.B) {
	sizes := []int{10, 50, 200}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("tables_%d", size), func(b *testing.B) {
			ddl := generateCreateTableDDL(size, 5)
			p := NewParser()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := p.ParseSQL(ddl)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkParseSQLComplex benchmarks parsing of complex DDL with constraints, indexes, etc.
func BenchmarkParseSQLComplex(b *testing.B) {
	// Use a real complex DDL string
	ddl := `CREATE TABLE orders (
        id SERIAL PRIMARY KEY,
        user_id INTEGER NOT NULL REFERENCES users(id),
        total NUMERIC(10,2) DEFAULT 0.00,
        status VARCHAR(20) DEFAULT 'pending'::character varying,
        created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now(),
        CONSTRAINT chk_total CHECK (total >= 0)
    );
    CREATE INDEX idx_orders_user_id ON orders(user_id);
    CREATE INDEX idx_orders_created_at ON orders(created_at DESC);`

	p := NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseSQL(ddl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateCreateTableDDL generates a DDL string with N tables, each with M columns.
func generateCreateTableDDL(numTables, numCols int) string {
	var b strings.Builder
	for t := 0; t < numTables; t++ {
		b.WriteString(fmt.Sprintf("CREATE TABLE table_%d (\n", t))
		b.WriteString("    id SERIAL PRIMARY KEY,\n")
		for c := 0; c < numCols; c++ {
			b.WriteString(fmt.Sprintf("    col_%d INTEGER DEFAULT 0", c))
			if c < numCols-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(");\n\n")
	}
	return b.String()
}
