package model

import (
	"encoding/json"
	"testing"
)

// TestSchemaSerialization tests that Schema can be serialized and deserialized
func TestSchemaSerialization(t *testing.T) {
	// Create a sample schema
	schema := NewSchema()
	ns := schema.GetOrCreateNamespace("public")

	// Add a table
	table := NewTable("public", "users")
	table.AddColumn(&Column{
		Name:       "id",
		DataType:   "integer",
		IsNullable: false,
	})
	table.AddColumn(&Column{
		Name:        "name",
		DataType:    "varchar",
		IsNullable:  true,
		DefaultExpr: nil,
	})
	ns.Tables["users"] = table

	// Add primary key
	table.PrimaryKey = &PrimaryKey{
		Name:    "users_pkey",
		Columns: []string{"id"},
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	// Deserialize from JSON
	deserialized := &Schema{}
	err = json.Unmarshal(data, deserialized)
	if err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	// Verify
	if len(deserialized.Schemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(deserialized.Schemas))
	}

	deserializedNs := deserialized.Schemas["public"]
	if len(deserializedNs.Tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(deserializedNs.Tables))
	}

	deserializedTable := deserializedNs.Tables["users"]
	if len(deserializedTable.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(deserializedTable.Columns))
	}

	if deserializedTable.Columns[0].Name != "id" {
		t.Errorf("expected column name 'id', got '%s'", deserializedTable.Columns[0].Name)
	}
}

func TestTablePlaceholderSerialization(t *testing.T) {
	table := NewTable("public", "test")
	table.IsPlaceholder = true

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("failed to marshal table: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "IsPlaceholder") || contains(jsonStr, "is_placeholder") {
		t.Errorf("JSON should not contain IsPlaceholder field, got: %s", jsonStr)
	}
}

func TestNewNamespace_InitializesObjectMaps(t *testing.T) {
	ns := NewNamespace("public")
	if ns.Tables == nil || ns.Types == nil || ns.Views == nil || ns.Sequences == nil || ns.Extensions == nil {
		t.Fatalf("namespace maps must be initialized: %#v", ns)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}
