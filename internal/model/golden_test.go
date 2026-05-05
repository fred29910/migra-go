package model_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/testutil"
)

// TestGoldenSchema demonstrates golden file testing
// To update golden file: go test -run TestGoldenSchema -update
func TestGoldenSchema(t *testing.T) {
	// Create a sample schema
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")

	// Add a table with various column types
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{
		Name:       "id",
		DataType:   "integer",
		IsNullable: false,
	})
	table.AddColumn(&model.Column{
		Name:        "username",
		DataType:    "varchar",
		IsNullable:  false,
		DefaultExpr: nil,
	})
	table.AddColumn(&model.Column{
		Name:        "created_at",
		DataType:    "timestamp",
		IsNullable:  true,
		DefaultExpr: stringPtr("now()"),
	})
	ns.Tables["users"] = table

	// Add primary key
	table.PrimaryKey = &model.PrimaryKey{
		Name:    "users_pkey",
		Columns: []string{"id"},
	}

	// Add an enum type
	ns.Types["user_role"] = &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user", "guest"},
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	// Compare with golden file (or update it)
	goldenPath := "testdata/schema_golden.json"
	testutil.GoldenFile(t, goldenPath, string(data), isUpdateMode())
}

// stringPtr returns a pointer to the string
func stringPtr(s string) *string {
	return &s
}

// isUpdateMode checks if UPDATE_GOLDEN env var is set
func isUpdateMode() bool {
	return os.Getenv("UPDATE_GOLDEN") == "1"
}
