package parser

import (
	"testing"
)

func TestCreateIndexHandler_Basic(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (id integer, email varchar(100)); CREATE INDEX idx_users_email ON users (email);`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["users"]
	if table == nil {
		t.Fatal("expected users table")
	}

	if len(table.Indexes) != 1 {
		t.Errorf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes["idx_users_email"]
	if idx == nil {
		t.Fatal("expected index 'idx_users_email'")
	}
}

func TestCreateIndexHandler_Unique(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (id integer, email varchar(100)); CREATE UNIQUE INDEX idx_users_email ON users (email);`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["users"]
	idx := table.Indexes["idx_users_email"]
	if idx == nil {
		t.Fatal("expected index")
	}
	if !idx.Unique {
		t.Error("expected index to be unique")
	}
}

func TestCreateIndexHandler_ExpressionIndex(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (id integer, email varchar(100)); CREATE INDEX idx_users_lower_email ON users ((lower(email)));`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["users"]
	idx := table.Indexes["idx_users_lower_email"]
	if idx == nil {
		t.Fatal("expected index")
	}
	if len(idx.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(idx.Elements))
	}
	if idx.Elements[0].Expr == "" {
		t.Error("expected expression index")
	}
}

func TestCreateIndexHandler_PartialIndex(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (id integer, email varchar(100)); CREATE INDEX idx_users_active ON users (email) WHERE email IS NOT NULL;`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["users"]
	idx := table.Indexes["idx_users_active"]
	if idx == nil {
		t.Fatal("expected index")
	}
	if idx.WhereClause == "" {
		t.Error("expected WHERE clause")
	}
}

func TestCreateIndexHandler_PrimaryKey(t *testing.T) {
	// 注意：这里需要设置 Primary=true，但 SQL 语法中不好直接设置
	// 这个测试暂时跳过，等完整实现后再调整
	t.Skip("Primary key index test needs special handling")
}

func TestCreateIndexHandler_WithMethod(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (id integer, email varchar(100)); CREATE INDEX idx_users_email ON users USING hash (email);`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["users"]
	idx := table.Indexes["idx_users_email"]
	if idx == nil {
		t.Fatal("expected index")
	}
	if idx.Method != "hash" {
		t.Errorf("expected method 'hash', got '%s'", idx.Method)
	}
}
