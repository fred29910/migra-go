package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestCreateIndexMutation_Apply(t *testing.T) {
	mut := CreateIndexMutation{
		Schema: "public",
		Index: model.Index{
			Name: "idx_users_email",
			Table: "users",
			Elements: []model.IndexElem{
				{Name: "email"},
			},
			Unique: false,
			Method: "btree",
		},
	}

	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	err := mut.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	table := ns.Tables["users"]
	if len(table.Indexes) != 1 {
		t.Errorf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes["idx_users_email"]
	if idx == nil {
		t.Fatal("expected index 'idx_users_email'")
	}
	if idx.Method != "btree" {
		t.Errorf("expected method 'btree', got '%s'", idx.Method)
	}
}

func TestCreateIndexMutation_IfNotExists(t *testing.T) {
	mut := CreateIndexMutation{
		Schema: "public",
		Index: model.Index{
			Name:     "idx_users_email",
			Table:    "users",
			Elements: []model.IndexElem{{Name: "email"}},
			IfNotExists: true,
		},
	}

	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	// 第一次创建
	err := mut.Apply(schema)
	if err != nil {
		t.Fatalf("First Apply failed: %v", err)
	}

	// 第二次创建（IfNotExists=true，应该跳过）
	err = mut.Apply(schema)
	if err != nil {
		t.Fatalf("Second Apply with IfNotExists should not fail: %v", err)
	}
}

func TestCreateIndexMutation_PrimaryKey(t *testing.T) {
	mut := CreateIndexMutation{
		Schema: "public",
		Index: model.Index{
			Name:     "pk_users",
			Table:    "users",
			Elements: []model.IndexElem{{Name: "id"}},
			Primary:  true,
			Unique:   true,
		},
	}

	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	err := mut.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if table.PrimaryKey == nil {
		t.Fatal("expected PrimaryKey to be set")
	}
	if len(table.PrimaryKey.Columns) != 1 || table.PrimaryKey.Columns[0] != "id" {
		t.Errorf("expected primary key on 'id' column")
	}
}
