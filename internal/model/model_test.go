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

// ---------------------------------------------------------------------------
// Clone tests
// ---------------------------------------------------------------------------

func TestView_Clone(t *testing.T) {
	v := &View{Name: "active_users", Definition: "SELECT * FROM users", Materialized: true}
	c := v.Clone()
	if c == v {
		t.Error("Clone should return a different pointer")
	}
	if *c != *v {
		t.Errorf("Clone = %+v, want %+v", c, v)
	}
	// Mutate original
	v.Name = "changed"
	if c.Name != "active_users" {
		t.Error("Mutating original should not affect clone")
	}
}

func TestSequence_Clone(t *testing.T) {
	s := &Sequence{
		Name: "my_seq", DataType: "integer", StartValue: 1, IncrementBy: 2,
		MinValue: 1, MaxValue: 999, CacheSize: 10, Cycle: true,
		OwnedByTable: "t", OwnedByColumn: "c",
	}
	c := s.Clone()
	if c == s {
		t.Error("Clone should return a different pointer")
	}
	if *c != *s {
		t.Errorf("Clone = %+v, want %+v", c, s)
	}
	s.Name = "changed"
	if c.Name != "my_seq" {
		t.Error("Mutating original should not affect clone")
	}
}

func TestExtension_Clone(t *testing.T) {
	e := &Extension{Name: "pgcrypto", Version: "1.3"}
	c := e.Clone()
	if c == e {
		t.Error("Clone should return a different pointer")
	}
	if *c != *e {
		t.Errorf("Clone = %+v, want %+v", c, e)
	}
}

func TestIndexElem_Clone(t *testing.T) {
	elem := IndexElem{Name: "col1", Expr: "(a+b)", Ordering: "DESC"}
	c := elem.Clone()
	if c != elem {
		t.Errorf("Clone = %+v, want %+v", c, elem)
	}
	// Mutate original should not affect clone (value type)
	elem.Name = "changed"
	if c.Name != "col1" {
		t.Error("Mutating original should not affect clone")
	}
}

func TestPrimaryKey_Clone(t *testing.T) {
	pk := &PrimaryKey{Name: "pk_users", Columns: []string{"id", "uuid"}}
	c := pk.Clone()
	if c == pk {
		t.Error("Clone should return a different pointer")
	}
	if len(c.Columns) != 2 || c.Columns[0] != "id" {
		t.Errorf("Columns = %v, want [id uuid]", c.Columns)
	}
	// Mutate original slice
	pk.Columns[0] = "changed"
	if c.Columns[0] != "id" {
		t.Error("Mutating original Columns should not affect clone")
	}
}

func TestPrimaryKey_Clone_Nil(t *testing.T) {
	var pk *PrimaryKey = nil
	c := pk.Clone()
	if c != nil {
		t.Error("Clone of nil should return nil")
	}
}

func TestEnumType_Clone(t *testing.T) {
	et := &EnumType{Name: "mood", Labels: []string{"happy", "sad"}}
	c := et.Clone()
	if c == et {
		t.Error("Clone should return a different pointer")
	}
	if len(c.Labels) != 2 || c.Labels[0] != "happy" {
		t.Errorf("Labels = %v, want [happy sad]", c.Labels)
	}
	// Mutate original
	et.Labels[0] = "angry"
	if c.Labels[0] != "happy" {
		t.Error("Mutating original Labels should not affect clone")
	}
}

func TestColumn_Clone(t *testing.T) {
	def := "now()"
	col := &Column{
		Name: "created_at", DataType: "timestamp", IsNullable: true,
		DefaultExpr: &def, IsIdentity: true, IdentityKind: "ALWAYS", Collation: "en_US",
	}
	c := col.Clone()
	if c == col {
		t.Error("Clone should return a different pointer")
	}
	if *c.DefaultExpr != "now()" {
		t.Errorf("DefaultExpr = %q, want %q", *c.DefaultExpr, "now()")
	}
	// Mutate original DefaultExpr
	*col.DefaultExpr = "CURRENT_TIMESTAMP"
	if *c.DefaultExpr != "now()" {
		t.Error("Mutating original DefaultExpr should not affect clone")
	}
}

func TestColumn_Clone_NilDefaultExpr(t *testing.T) {
	col := &Column{Name: "id", DataType: "integer", IsNullable: false, DefaultExpr: nil}
	c := col.Clone()
	if c.DefaultExpr != nil {
		t.Error("Clone of nil DefaultExpr should be nil")
	}
}

func TestConstraint_Clone(t *testing.T) {
	c := &Constraint{
		Name: "fk_user", Type: "foreign_key", Definition: "FOREIGN KEY (user_id) REFERENCES users(id)",
		Table: "orders", Columns: []string{"user_id"}, RefSchema: "public",
		RefTable: "users", RefColumns: []string{"id"}, Expression: "",
		OnDelete: "CASCADE", OnUpdate: "NO ACTION",
	}
	cc := c.Clone()
	if cc == c {
		t.Error("Clone should return a different pointer")
	}
	if len(cc.Columns) != 1 || cc.Columns[0] != "user_id" {
		t.Errorf("Columns = %v, want [user_id]", cc.Columns)
	}
	// Mutate original slices
	c.Columns[0] = "changed"
	c.RefColumns[0] = "changed"
	if cc.Columns[0] != "user_id" {
		t.Error("Mutating original Columns should not affect clone")
	}
	if cc.RefColumns[0] != "id" {
		t.Error("Mutating original RefColumns should not affect clone")
	}
}

func TestIndex_Clone(t *testing.T) {
	idx := &Index{
		Name: "idx_users_email", Table: "users", Columns: []string{"email"},
		Elements: []IndexElem{{Name: "email", Ordering: "ASC"}},
		Unique: true, Method: "btree", WhereClause: "active = true",
		Definition: "CREATE UNIQUE INDEX ...", Concurrent: true, IfNotExists: true,
	}
	c := idx.Clone()
	if c == idx {
		t.Error("Clone should return a different pointer")
	}
	// Verify slices are independent
	idx.Columns[0] = "changed"
	if c.Columns[0] != "email" {
		t.Error("Mutating original Columns should not affect clone")
	}
	idx.Elements[0].Name = "changed"
	if c.Elements[0].Name != "email" {
		t.Error("Mutating original Elements should not affect clone")
	}
}

func TestTable_Clone(t *testing.T) {
	tbl := NewTable("public", "users")
	col1 := &Column{Name: "id", DataType: "integer", DefaultExpr: nil}
	col2 := &Column{Name: "name", DataType: "text", IsNullable: true}
	tbl.AddColumn(col1)
	tbl.AddColumn(col2)
	tbl.PrimaryKey = &PrimaryKey{Name: "pk_users", Columns: []string{"id"}}
	tbl.Constraints["fk_role"] = &Constraint{
		Name: "fk_role", Type: "foreign_key", Columns: []string{"role_id"},
		RefTable: "roles", RefColumns: []string{"id"},
	}
	tbl.Indexes["idx_name"] = &Index{
		Name: "idx_name", Columns: []string{"name"}, Elements: []IndexElem{{Name: "name"}},
	}

	ct := tbl.Clone()
	if ct == tbl {
		t.Fatal("Clone should return a different pointer")
	}

	// 1. Columns slice is independent
	if len(ct.Columns) != 2 {
		t.Fatalf("Columns len = %d, want 2", len(ct.Columns))
	}
	tbl.Columns[0].Name = "changed"
	if ct.Columns[0].Name != "id" {
		t.Error("Mutating original Column should not affect clone's column")
	}

	// 2. ColumnByName has same *Column objects as Columns slice
	if ct.ColumnByName["id"] != ct.Columns[0] {
		t.Error("ColumnByName should reference same *Column as Columns slice")
	}
	if ct.ColumnByName["name"] != ct.Columns[1] {
		t.Error("ColumnByName should reference same *Column as Columns slice")
	}

	// 3. ColumnIndex is independent
	if ct.ColumnIndex["id"] != 0 || ct.ColumnIndex["name"] != 1 {
		t.Error("ColumnIndex should be cloned correctly")
	}

	// 4. PrimaryKey clone
	if ct.PrimaryKey == tbl.PrimaryKey {
		t.Error("PrimaryKey should be cloned")
	}
	tbl.PrimaryKey.Columns[0] = "changed"
	if ct.PrimaryKey.Columns[0] != "id" {
		t.Error("Mutating original PrimaryKey should not affect clone")
	}

	// 5. Constraints map independent
	if len(ct.Constraints) != 1 {
		t.Fatalf("Constraints len = %d, want 1", len(ct.Constraints))
	}
	tbl.Constraints["fk_role"].Columns[0] = "changed"
	if ct.Constraints["fk_role"].Columns[0] != "role_id" {
		t.Error("Mutating original Constraint should not affect clone")
	}

	// 6. Indexes map independent
	if len(ct.Indexes) != 1 {
		t.Fatalf("Indexes len = %d, want 1", len(ct.Indexes))
	}
	tbl.Indexes["idx_name"].Columns[0] = "changed"
	if ct.Indexes["idx_name"].Columns[0] != "name" {
		t.Error("Mutating original Index should not affect clone")
	}
}

func TestTable_Clone_NilPrimaryKey(t *testing.T) {
	tbl := NewTable("public", "users")
	ct := tbl.Clone()
	if ct.PrimaryKey != nil {
		t.Error("Clone of nil PrimaryKey should be nil")
	}
}

func TestTable_Clone_EmptyMaps(t *testing.T) {
	tbl := &Table{
		Schema: "public", Name: "empty",
		Columns: make([]*Column, 0), ColumnByName: make(map[string]*Column),
		ColumnIndex: make(map[string]int), Constraints: make(map[string]*Constraint),
		Indexes: make(map[string]*Index),
	}
	ct := tbl.Clone()
	if ct == nil {
		t.Fatal("Clone should not return nil")
	}
	if ct.PrimaryKey != nil {
		t.Error("PrimaryKey should be nil")
	}
}

func TestNamespace_Clone(t *testing.T) {
	ns := NewNamespace("public")
	ns.Tables["users"] = &Table{
		Schema: "public", Name: "users",
		Columns: []*Column{{Name: "id", DataType: "integer"}},
	}
	ns.Types["mood"] = &EnumType{Name: "mood", Labels: []string{"happy"}}
	ns.Views["v"] = &View{Name: "v", Definition: "SELECT 1"}
	ns.Sequences["seq"] = &Sequence{Name: "seq", StartValue: 1}
	ns.Extensions["ext"] = &Extension{Name: "ext", Version: "1.0"}

	cn := ns.Clone()
	if cn == ns {
		t.Fatal("Clone should return a different pointer")
	}

	// Verify all maps are independent
	ns.Tables["users"].Name = "changed"
	if cn.Tables["users"].Name != "users" {
		t.Error("Mutating original Table should not affect clone")
	}
	ns.Types["mood"].Labels[0] = "sad"
	if cn.Types["mood"].Labels[0] != "happy" {
		t.Error("Mutating original EnumType should not affect clone")
	}
	ns.Views["v"].Definition = "SELECT 2"
	if cn.Views["v"].Definition != "SELECT 1" {
		t.Error("Mutating original View should not affect clone")
	}
	ns.Sequences["seq"].StartValue = 99
	if cn.Sequences["seq"].StartValue != 1 {
		t.Error("Mutating original Sequence should not affect clone")
	}
	ns.Extensions["ext"].Version = "2.0"
	if cn.Extensions["ext"].Version != "1.0" {
		t.Error("Mutating original Extension should not affect clone")
	}
}

func TestNamespace_Clone_EmptyMaps(t *testing.T) {
	ns := NewNamespace("empty")
	cn := ns.Clone()
	if cn == nil {
		t.Fatal("Clone should not return nil")
	}
	if cn.Name != "empty" {
		t.Errorf("Name = %q, want %q", cn.Name, "empty")
	}
}

func TestSchema_Clone(t *testing.T) {
	s := NewSchema()
	s.GetOrCreateNamespace("public")
	s.GetOrCreateNamespace("app")

	cs := s.Clone()
	if cs == s {
		t.Fatal("Clone should return a different pointer")
	}
	if len(cs.Schemas) != 2 {
		t.Fatalf("Schemas len = %d, want 2", len(cs.Schemas))
	}

	// Verify map independence
	s.Schemas["public"].Name = "changed"
	if cs.Schemas["public"].Name != "public" {
		t.Error("Mutating original Namespace should not affect clone")
	}
}

func TestSchema_Clone_Empty(t *testing.T) {
	s := NewSchema()
	cs := s.Clone()
	if cs == nil {
		t.Fatal("Clone should not return nil")
	}
	if len(cs.Schemas) != 0 {
		t.Errorf("Schemas len = %d, want 0", len(cs.Schemas))
	}
}

// ---------------------------------------------------------------------------
// Model accessor convenience method tests
// ---------------------------------------------------------------------------

func TestSchema_HasNamespace(t *testing.T) {
	schema := NewSchema()
	if schema.HasNamespace("public") {
		t.Error("HasNamespace(public) should be false initially")
	}
	schema.GetOrCreateNamespace("public")
	if !schema.HasNamespace("public") {
		t.Error("HasNamespace(public) should be true after creation")
	}
}

func TestSchema_NamespaceNames(t *testing.T) {
	schema := NewSchema()
	schema.GetOrCreateNamespace("z_last")
	schema.GetOrCreateNamespace("a_first")
	got := schema.NamespaceNames()
	if len(got) != 2 || got[0] != "a_first" || got[1] != "z_last" {
		t.Errorf("NamespaceNames = %v, want [a_first z_last]", got)
	}
}

func TestSchema_GetTable(t *testing.T) {
	schema := NewSchema()
	if schema.GetTable("public", "users") != nil {
		t.Error("GetTable should return nil for non-existent namespace")
	}
	ns := schema.GetOrCreateNamespace("public")
	tbl := NewTable("public", "users")
	ns.Tables["users"] = tbl
	if schema.GetTable("public", "users") != tbl {
		t.Error("GetTable should return the correct table")
	}
	if schema.GetTable("public", "nonexistent") != nil {
		t.Error("GetTable should return nil for non-existent table")
	}
}

func TestNamespace_GetTable(t *testing.T) {
	ns := NewNamespace("public")
	if ns.GetTable("users") != nil {
		t.Error("GetTable should return nil for non-existent table")
	}
	tbl := NewTable("public", "users")
	ns.Tables["users"] = tbl
	if ns.GetTable("users") != tbl {
		t.Error("GetTable should return the added table")
	}
}

func TestNamespace_HasTable(t *testing.T) {
	ns := NewNamespace("public")
	if ns.HasTable("users") {
		t.Error("HasTable should be false initially")
	}
	ns.Tables["users"] = NewTable("public", "users")
	if !ns.HasTable("users") {
		t.Error("HasTable should be true after adding")
	}
}

func TestNamespace_TableNames(t *testing.T) {
	ns := NewNamespace("public")
	ns.Tables["z"] = NewTable("public", "z")
	ns.Tables["a"] = NewTable("public", "a")
	got := ns.TableNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "z" {
		t.Errorf("TableNames = %v, want [a z]", got)
	}
}

func TestNamespace_GetType(t *testing.T) {
	ns := NewNamespace("public")
	if ns.GetType("mood") != nil {
		t.Error("GetType should return nil for non-existent type")
	}
	et := &EnumType{Name: "mood"}
	ns.Types["mood"] = et
	if ns.GetType("mood") != et {
		t.Error("GetType should return the added type")
	}
}

func TestNamespace_HasType(t *testing.T) {
	ns := NewNamespace("public")
	if ns.HasType("mood") {
		t.Error("HasType should be false initially")
	}
	ns.Types["mood"] = &EnumType{Name: "mood"}
	if !ns.HasType("mood") {
		t.Error("HasType should be true after adding")
	}
}

func TestNamespace_TypeNames(t *testing.T) {
	ns := NewNamespace("public")
	ns.Types["b"] = &EnumType{Name: "b"}
	ns.Types["a"] = &EnumType{Name: "a"}
	got := ns.TypeNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("TypeNames = %v, want [a b]", got)
	}
}

func TestNamespace_GetView(t *testing.T) {
	ns := NewNamespace("public")
	if ns.GetView("v") != nil {
		t.Error("GetView should return nil for non-existent view")
	}
	v := &View{Name: "v"}
	ns.Views["v"] = v
	if ns.GetView("v") != v {
		t.Error("GetView should return the added view")
	}
}

func TestNamespace_HasView(t *testing.T) {
	ns := NewNamespace("public")
	if ns.HasView("v") {
		t.Error("HasView should be false initially")
	}
	ns.Views["v"] = &View{Name: "v"}
	if !ns.HasView("v") {
		t.Error("HasView should be true after adding")
	}
}

func TestNamespace_ViewNames(t *testing.T) {
	ns := NewNamespace("public")
	ns.Views["b"] = &View{Name: "b"}
	ns.Views["a"] = &View{Name: "a"}
	got := ns.ViewNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ViewNames = %v, want [a b]", got)
	}
}

func TestNamespace_GetSequence(t *testing.T) {
	ns := NewNamespace("public")
	if ns.GetSequence("seq") != nil {
		t.Error("GetSequence should return nil for non-existent sequence")
	}
	seq := &Sequence{Name: "seq"}
	ns.Sequences["seq"] = seq
	if ns.GetSequence("seq") != seq {
		t.Error("GetSequence should return the added sequence")
	}
}

func TestNamespace_HasSequence(t *testing.T) {
	ns := NewNamespace("public")
	if ns.HasSequence("seq") {
		t.Error("HasSequence should be false initially")
	}
	ns.Sequences["seq"] = &Sequence{Name: "seq"}
	if !ns.HasSequence("seq") {
		t.Error("HasSequence should be true after adding")
	}
}

func TestNamespace_SequenceNames(t *testing.T) {
	ns := NewNamespace("public")
	ns.Sequences["b"] = &Sequence{Name: "b"}
	ns.Sequences["a"] = &Sequence{Name: "a"}
	got := ns.SequenceNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("SequenceNames = %v, want [a b]", got)
	}
}

func TestNamespace_GetExtension(t *testing.T) {
	ns := NewNamespace("public")
	if ns.GetExtension("pgcrypto") != nil {
		t.Error("GetExtension should return nil for non-existent extension")
	}
	ext := &Extension{Name: "pgcrypto"}
	ns.Extensions["pgcrypto"] = ext
	if ns.GetExtension("pgcrypto") != ext {
		t.Error("GetExtension should return the added extension")
	}
}

func TestNamespace_HasExtension(t *testing.T) {
	ns := NewNamespace("public")
	if ns.HasExtension("pgcrypto") {
		t.Error("HasExtension should be false initially")
	}
	ns.Extensions["pgcrypto"] = &Extension{Name: "pgcrypto"}
	if !ns.HasExtension("pgcrypto") {
		t.Error("HasExtension should be true after adding")
	}
}

func TestNamespace_ExtensionNames(t *testing.T) {
	ns := NewNamespace("public")
	ns.Extensions["b"] = &Extension{Name: "b"}
	ns.Extensions["a"] = &Extension{Name: "a"}
	got := ns.ExtensionNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ExtensionNames = %v, want [a b]", got)
	}
}

func TestTable_HasColumn(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.AddColumn(&Column{Name: "id"})
	if !tbl.HasColumn("id") {
		t.Error("HasColumn(id) should be true after adding")
	}
	if tbl.HasColumn("name") {
		t.Error("HasColumn(name) should be false")
	}
}

func TestTable_ColumnNames(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.AddColumn(&Column{Name: "id"})
	tbl.AddColumn(&Column{Name: "name"})
	got := tbl.ColumnNames()
	if len(got) != 2 || got[0] != "id" || got[1] != "name" {
		t.Errorf("ColumnNames = %v, want [id name]", got)
	}
}

func TestTable_ColumnCount(t *testing.T) {
	tbl := NewTable("public", "users")
	if tbl.ColumnCount() != 0 {
		t.Errorf("ColumnCount = %d, want 0", tbl.ColumnCount())
	}
	tbl.AddColumn(&Column{Name: "id"})
	if tbl.ColumnCount() != 1 {
		t.Errorf("ColumnCount = %d, want 1", tbl.ColumnCount())
	}
}

func TestTable_GetIndex(t *testing.T) {
	tbl := NewTable("public", "users")
	if tbl.GetIndex("idx_name") != nil {
		t.Error("GetIndex should return nil for non-existent index")
	}
	idx := &Index{Name: "idx_name"}
	tbl.Indexes["idx_name"] = idx
	if tbl.GetIndex("idx_name") != idx {
		t.Error("GetIndex should return the added index")
	}
}

func TestTable_HasIndex(t *testing.T) {
	tbl := NewTable("public", "users")
	if tbl.HasIndex("idx_name") {
		t.Error("HasIndex should be false initially")
	}
	tbl.Indexes["idx_name"] = &Index{Name: "idx_name"}
	if !tbl.HasIndex("idx_name") {
		t.Error("HasIndex should be true after adding")
	}
}

func TestTable_IndexNames(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.Indexes["b"] = &Index{Name: "b"}
	tbl.Indexes["a"] = &Index{Name: "a"}
	got := tbl.IndexNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("IndexNames = %v, want [a b]", got)
	}
}

func TestTable_GetConstraint(t *testing.T) {
	tbl := NewTable("public", "users")
	if tbl.GetConstraint("pk_users") != nil {
		t.Error("GetConstraint should return nil for non-existent constraint")
	}
	c := &Constraint{Name: "pk_users"}
	tbl.Constraints["pk_users"] = c
	if tbl.GetConstraint("pk_users") != c {
		t.Error("GetConstraint should return the added constraint")
	}
}

func TestTable_HasConstraint(t *testing.T) {
	tbl := NewTable("public", "users")
	if tbl.HasConstraint("pk_users") {
		t.Error("HasConstraint should be false initially")
	}
	tbl.Constraints["pk_users"] = &Constraint{Name: "pk_users"}
	if !tbl.HasConstraint("pk_users") {
		t.Error("HasConstraint should be true after adding")
	}
}

func TestTable_ConstraintNames(t *testing.T) {
	tbl := NewTable("public", "users")
	tbl.Constraints["b"] = &Constraint{Name: "b"}
	tbl.Constraints["a"] = &Constraint{Name: "a"}
	got := tbl.ConstraintNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ConstraintNames = %v, want [a b]", got)
	}
}
