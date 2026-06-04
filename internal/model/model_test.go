package model

import (
	"testing"
)

func TestFKActionCode_AllCodes(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"a", "NO ACTION"},
		{"r", "RESTRICT"},
		{"c", "CASCADE"},
		{"n", "SET NULL"},
		{"d", "SET DEFAULT"},
		{"x", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := FKActionCode(tt.code)
			if got != tt.want {
				t.Errorf("FKActionCode(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestNewObjectKey(t *testing.T) {
	key := NewObjectKey("public", "users", KindTable)
	if key.Schema != "public" {
		t.Errorf("Schema = %q, want %q", key.Schema, "public")
	}
	if key.Name != "users" {
		t.Errorf("Name = %q, want %q", key.Name, "users")
	}
	if key.Kind != KindTable {
		t.Errorf("Kind = %q, want %q", key.Kind, KindTable)
	}
	if key.Signature != "" {
		t.Errorf("Signature = %q, want empty", key.Signature)
	}
}

func TestNewFunctionKey(t *testing.T) {
	key := NewFunctionKey("public", "my_func", "integer, text")
	if key.Schema != "public" {
		t.Errorf("Schema = %q, want %q", key.Schema, "public")
	}
	if key.Name != "my_func" {
		t.Errorf("Name = %q, want %q", key.Name, "my_func")
	}
	if key.Kind != KindFunction {
		t.Errorf("Kind = %q, want %q", key.Kind, KindFunction)
	}
	if key.Signature != "integer, text" {
		t.Errorf("Signature = %q, want %q", key.Signature, "integer, text")
	}
}

func TestObjectKindConstants(t *testing.T) {
	expected := map[ObjectKind]string{
		KindTable:      "table",
		KindColumn:     "column",
		KindIndex:      "index",
		KindConstraint: "constraint",
		KindType:       "type",
		KindView:       "view",
		KindFunction:   "function",
		KindSchema:     "schema",
		KindSequence:   "sequence",
		KindExtension:  "extension",
	}
	for kind, want := range expected {
		if string(kind) != want {
			t.Errorf("ObjectKind %v = %q, want %q", kind, string(kind), want)
		}
	}
}

func TestGetNamespace(t *testing.T) {
	s := NewSchema()
	s.GetOrCreateNamespace("public")

	ns := s.GetNamespace("public")
	if ns == nil {
		t.Fatal("GetNamespace(public) returned nil")
	}
	if ns.Name != "public" {
		t.Errorf("ns.Name = %q, want %q", ns.Name, "public")
	}

	if s.GetNamespace("nonexistent") != nil {
		t.Error("GetNamespace(nonexistent) should return nil")
	}
}

func TestGetOrCreateNamespace_CreatesOnce(t *testing.T) {
	s := NewSchema()
	ns1 := s.GetOrCreateNamespace("public")
	ns2 := s.GetOrCreateNamespace("public")
	if ns1 != ns2 {
		t.Error("GetOrCreateNamespace should return same instance on second call")
	}
}

func TestEnumType(t *testing.T) {
	et := &EnumType{
		Name:   "status",
		Labels: []string{"active", "inactive"},
	}
	if et.Name != "status" {
		t.Errorf("Name = %q, want %q", et.Name, "status")
	}
	if len(et.Labels) != 2 {
		t.Errorf("Labels len = %d, want 2", len(et.Labels))
	}
}

func TestNewTable_InitialState(t *testing.T) {
	tbl := NewTable("public", "orders")
	if tbl.Schema != "public" {
		t.Errorf("Schema = %q, want %q", tbl.Schema, "public")
	}
	if tbl.Name != "orders" {
		t.Errorf("Name = %q, want %q", tbl.Name, "orders")
	}
	if tbl.Columns == nil {
		t.Error("Columns should be initialized")
	}
	if tbl.ColumnByName == nil {
		t.Error("ColumnByName should be initialized")
	}
	if tbl.Constraints == nil {
		t.Error("Constraints should be initialized")
	}
	if tbl.Indexes == nil {
		t.Error("Indexes should be initialized")
	}
	if tbl.IsPlaceholder {
		t.Error("IsPlaceholder should be false")
	}
}

func TestGetColumn(t *testing.T) {
	tbl := NewTable("public", "users")
	col := &Column{Name: "id", DataType: "integer"}
	tbl.AddColumn(col)

	got := tbl.GetColumn("id")
	if got != col {
		t.Error("GetColumn should return the added column")
	}

	if tbl.GetColumn("nonexistent") != nil {
		t.Error("GetColumn(nonexistent) should return nil")
	}
}

func TestRemoveColumn(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.AddColumn(&Column{Name: "id", DataType: "integer"})
	tbl.AddColumn(&Column{Name: "name", DataType: "varchar"})

	tbl.RemoveColumn("id")

	if len(tbl.Columns) != 1 {
		t.Errorf("Columns len = %d, want 1", len(tbl.Columns))
	}
	if tbl.Columns[0].Name != "name" {
		t.Errorf("Remaining column = %q, want %q", tbl.Columns[0].Name, "name")
	}
	if _, ok := tbl.ColumnByName["id"]; ok {
		t.Error("ColumnByName should not contain 'id' after removal")
	}
}

func TestRemoveColumn_NotFound(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.AddColumn(&Column{Name: "id", DataType: "integer"})

	tbl.RemoveColumn("nonexistent")
	if len(tbl.Columns) != 1 {
		t.Errorf("Columns len = %d, want 1", len(tbl.Columns))
	}
}

func TestRemoveColumn_LastElement(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.AddColumn(&Column{Name: "a", DataType: "integer"})
	tbl.AddColumn(&Column{Name: "b", DataType: "varchar"})
	tbl.RemoveColumn("b")

	if len(tbl.Columns) != 1 {
		t.Errorf("Columns len = %d, want 1", len(tbl.Columns))
	}
	if tbl.Columns[0].Name != "a" {
		t.Errorf("Remaining column = %q, want %q", tbl.Columns[0].Name, "a")
	}
}

func TestColumn_Fields(t *testing.T) {
	defExpr := "now()"
	col := &Column{
		Name:         "created_at",
		DataType:     "timestamp",
		IsNullable:   true,
		DefaultExpr:  &defExpr,
		IsIdentity:   true,
		IdentityKind: "ALWAYS",
		Collation:    "en_US",
	}
	if col.Name != "created_at" {
		t.Errorf("Name = %q, want %q", col.Name, "created_at")
	}
	if col.DataType != "timestamp" {
		t.Errorf("DataType = %q, want %q", col.DataType, "timestamp")
	}
	if !col.IsNullable {
		t.Error("IsNullable should be true")
	}
	if *col.DefaultExpr != "now()" {
		t.Errorf("DefaultExpr = %q, want %q", *col.DefaultExpr, "now()")
	}
	if !col.IsIdentity {
		t.Error("IsIdentity should be true")
	}
	if col.IdentityKind != "ALWAYS" {
		t.Errorf("IdentityKind = %q, want %q", col.IdentityKind, "ALWAYS")
	}
	if col.Collation != "en_US" {
		t.Errorf("Collation = %q, want %q", col.Collation, "en_US")
	}
}

func TestIndex_Fields(t *testing.T) {
	idx := &Index{
		Name:         "idx_users_email",
		Table:        "users",
		Columns:      []string{"email"},
		Elements:     []IndexElem{{Name: "email", Ordering: "ASC"}},
		Unique:       true,
		Method:       "btree",
		Primary:      false,
		IsConstraint: false,
		WhereClause:  "active = true",
		Definition:   "CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE active = true",
		Concurrent:   true,
		IfNotExists:  true,
	}
	if idx.Name != "idx_users_email" {
		t.Errorf("Name = %q, want %q", idx.Name, "idx_users_email")
	}
	if !idx.Unique {
		t.Error("Unique should be true")
	}
	if !idx.Concurrent {
		t.Error("Concurrent should be true")
	}
	if !idx.IfNotExists {
		t.Error("IfNotExists should be true")
	}
	if idx.WhereClause != "active = true" {
		t.Errorf("WhereClause = %q, want %q", idx.WhereClause, "active = true")
	}
	if len(idx.Elements) != 1 {
		t.Errorf("Elements len = %d, want 1", len(idx.Elements))
	}
}

func TestIndexElem_Fields(t *testing.T) {
	elem := IndexElem{
		Name:          "col1",
		Expr:          "(col1 + col2)",
		IndexColName:  "sum_col",
		Collation:     "en_US",
		Opclass:       "text_ops",
		Ordering:      "DESC",
		NullsOrdering: "FIRST",
	}
	if elem.Name != "col1" {
		t.Errorf("Name = %q, want %q", elem.Name, "col1")
	}
	if elem.Expr != "(col1 + col2)" {
		t.Errorf("Expr = %q, want %q", elem.Expr, "(col1 + col2)")
	}
	if elem.IndexColName != "sum_col" {
		t.Errorf("IndexColName = %q, want %q", elem.IndexColName, "sum_col")
	}
	if elem.Opclass != "text_ops" {
		t.Errorf("Opclass = %q, want %q", elem.Opclass, "text_ops")
	}
	if elem.NullsOrdering != "FIRST" {
		t.Errorf("NullsOrdering = %q, want %q", elem.NullsOrdering, "FIRST")
	}
}

func TestConstraint_Fields(t *testing.T) {
	c := &Constraint{
		Name:       "fk_user",
		Type:       "foreign_key",
		Definition: "FOREIGN KEY (user_id) REFERENCES users(id)",
		Table:      "orders",
		Columns:    []string{"user_id"},
		RefSchema:  "public",
		RefTable:   "users",
		RefColumns: []string{"id"},
		Expression: "",
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}
	if c.Name != "fk_user" {
		t.Errorf("Name = %q, want %q", c.Name, "fk_user")
	}
	if c.RefSchema != "public" {
		t.Errorf("RefSchema = %q, want %q", c.RefSchema, "public")
	}
	if c.OnDelete != "CASCADE" {
		t.Errorf("OnDelete = %q, want %q", c.OnDelete, "CASCADE")
	}
}

func TestView_Fields(t *testing.T) {
	v := &View{
		Name:         "active_users",
		Definition:   "SELECT * FROM users WHERE active = true",
		Materialized: true,
	}
	if v.Name != "active_users" {
		t.Errorf("Name = %q, want %q", v.Name, "active_users")
	}
	if !v.Materialized {
		t.Error("Materialized should be true")
	}
}

func TestSequence_Fields(t *testing.T) {
	seq := &Sequence{
		Name:          "order_seq",
		DataType:      "integer",
		StartValue:    1,
		IncrementBy:   1,
		MinValue:      1,
		MaxValue:      999999,
		CacheSize:     10,
		Cycle:         true,
		OwnedByTable:  "orders",
		OwnedByColumn: "id",
	}
	if seq.Name != "order_seq" {
		t.Errorf("Name = %q, want %q", seq.Name, "order_seq")
	}
	if seq.StartValue != 1 {
		t.Errorf("StartValue = %d, want 1", seq.StartValue)
	}
	if !seq.Cycle {
		t.Error("Cycle should be true")
	}
	if seq.OwnedByTable != "orders" {
		t.Errorf("OwnedByTable = %q, want %q", seq.OwnedByTable, "orders")
	}
}

func TestExtension_Fields(t *testing.T) {
	ext := &Extension{
		Name:    "pgcrypto",
		Version: "1.3",
	}
	if ext.Name != "pgcrypto" {
		t.Errorf("Name = %q, want %q", ext.Name, "pgcrypto")
	}
	if ext.Version != "1.3" {
		t.Errorf("Version = %q, want %q", ext.Version, "1.3")
	}
}
