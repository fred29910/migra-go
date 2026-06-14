package diff

import (
	"reflect"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

// --- diffTables ---

func TestDiffTables_NilSource_AllTablesAdded(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	target := model.NewNamespace("public")
	target.Tables["users"] = model.NewTable("public", "users")
	target.Tables["posts"] = model.NewTable("public", "posts")

	ctx.diffTables(nil, target)

	if len(ctx.ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ctx.ops))
	}
	kinds := map[Kind]bool{}
	for _, op := range ctx.ops {
		kinds[op.Kind()] = true
	}
	if !kinds[KindAddTable] {
		t.Fatal("expected KindAddTable ops")
	}
}

func TestDiffTables_NilTarget_NoTableOps(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewNamespace("public")
	source.Tables["users"] = model.NewTable("public", "users")

	ctx.diffTables(source, nil)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops, got %d", len(ctx.ops))
	}
}

func TestDiffTables_AddTable(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewNamespace("public")
	source.Tables["users"] = model.NewTable("public", "users")

	target := model.NewNamespace("public")
	target.Tables["users"] = model.NewTable("public", "users")
	target.Tables["posts"] = model.NewTable("public", "posts")

	ctx.diffTables(source, target)

	var addCount int
	for _, op := range ctx.ops {
		if op.Kind() == KindAddTable {
			addCount++
		}
	}
	if addCount != 1 {
		t.Fatalf("expected 1 AddTable op, got %d", addCount)
	}
}

func TestDiffTables_DropTable(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewNamespace("public")
	source.Tables["users"] = model.NewTable("public", "users")
	source.Tables["old_table"] = model.NewTable("public", "old_table")

	target := model.NewNamespace("public")
	target.Tables["users"] = model.NewTable("public", "users")

	ctx.diffTables(source, target)

	var dropCount int
	for _, op := range ctx.ops {
		if op.Kind() == KindDropTable {
			dropCount++
		}
	}
	if dropCount != 1 {
		t.Fatalf("expected 1 DropTable op, got %d", dropCount)
	}
}

func TestDiffTables_IdenticalTables_NoOps(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewNamespace("public")
	source.Tables["users"] = model.NewTable("public", "users")

	target := model.NewNamespace("public")
	target.Tables["users"] = model.NewTable("public", "users")

	ctx.diffTables(source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for identical tables, got %d", len(ctx.ops))
	}
}

// --- sameConstraintContent ---

func TestSameConstraintContent_BothNil(t *testing.T) {
	if !sameConstraintContent(nil, nil) {
		t.Error("expected sameConstraintContent(nil, nil) to be true")
	}
}

func TestSameConstraintContent_OneNil(t *testing.T) {
	a := &model.Constraint{Name: "pk", Type: "primary_key"}
	if sameConstraintContent(a, nil) {
		t.Error("expected sameConstraintContent(a, nil) to be false")
	}
	if sameConstraintContent(nil, a) {
		t.Error("expected sameConstraintContent(nil, a) to be false")
	}
}

func TestSameConstraintContent_Identical(t *testing.T) {
	a := &model.Constraint{
		Name:       "pk_users",
		Type:       "primary_key",
		Columns:    []string{"id"},
		RefSchema:  "public",
		RefTable:   "users",
		RefColumns: []string{"id"},
		Expression: "",
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}
	b := &model.Constraint{
		Name:       "pk_users",
		Type:       "primary_key",
		Columns:    []string{"id"},
		RefSchema:  "public",
		RefTable:   "users",
		RefColumns: []string{"id"},
		Expression: "",
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}
	if !sameConstraintContent(a, b) {
		t.Error("expected identical constraints to be equal")
	}
}

func TestSameConstraintContent_DifferentType(t *testing.T) {
	a := &model.Constraint{Type: "primary_key"}
	b := &model.Constraint{Type: "unique"}
	if sameConstraintContent(a, b) {
		t.Error("expected different types to be unequal")
	}
}

func TestSameConstraintContent_DifferentColumns(t *testing.T) {
	a := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	b := &model.Constraint{Type: "primary_key", Columns: []string{"id", "name"}}
	if sameConstraintContent(a, b) {
		t.Error("expected different columns to be unequal")
	}
}

func TestSameConstraintContent_DifferentRefTable(t *testing.T) {
	a := &model.Constraint{Type: "foreign_key", RefTable: "posts"}
	b := &model.Constraint{Type: "foreign_key", RefTable: "articles"}
	if sameConstraintContent(a, b) {
		t.Error("expected different ref tables to be unequal")
	}
}

func TestSameConstraintContent_DifferentOnDelete(t *testing.T) {
	a := &model.Constraint{Type: "foreign_key", OnDelete: "CASCADE"}
	b := &model.Constraint{Type: "foreign_key", OnDelete: "SET NULL"}
	if sameConstraintContent(a, b) {
		t.Error("expected different OnDelete to be unequal")
	}
}

func TestSameConstraintContent_DefinitionExcluded(t *testing.T) {
	a := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age > 0", Definition: "CHECK (age > 0)"}
	b := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age > 0", Definition: "CHECK ((age > 0))"}
	if !sameConstraintContent(a, b) {
		t.Error("expected constraints with different Definition but same structured fields to be equal")
	}
}

// --- sameConstraintSemantics ---

func TestSameConstraintSemantics_BothNil(t *testing.T) {
	if !sameConstraintSemantics(nil, nil) {
		t.Error("expected sameConstraintSemantics(nil, nil) to be true")
	}
}

func TestSameConstraintSemantics_OneNil(t *testing.T) {
	a := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	if sameConstraintSemantics(a, nil) {
		t.Error("expected sameConstraintSemantics(a, nil) to be false")
	}
	if sameConstraintSemantics(nil, a) {
		t.Error("expected sameConstraintSemantics(nil, a) to be false")
	}
}

func TestSameConstraintSemantics_SamePrimaryKey(t *testing.T) {
	a := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	b := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	if !sameConstraintSemantics(a, b) {
		t.Error("expected same PK semantics to be equal")
	}
}

func TestSameConstraintSemantics_DifferentType(t *testing.T) {
	a := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	b := &model.Constraint{Type: "unique", Columns: []string{"id"}}
	if sameConstraintSemantics(a, b) {
		t.Error("expected different types to be unequal")
	}
}

func TestSameConstraintSemantics_DifferentColumns(t *testing.T) {
	a := &model.Constraint{Type: "primary_key", Columns: []string{"id"}}
	b := &model.Constraint{Type: "primary_key", Columns: []string{"name"}}
	if sameConstraintSemantics(a, b) {
		t.Error("expected different columns to be unequal")
	}
}

func TestSameConstraintSemantics_FKSameRef(t *testing.T) {
	a := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}
	b := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}
	if !sameConstraintSemantics(a, b) {
		t.Error("expected same FK semantics to be equal")
	}
}

func TestSameConstraintSemantics_FKDifferentRefTable(t *testing.T) {
	a := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	}
	b := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "articles",
		RefColumns: []string{"id"},
	}
	if sameConstraintSemantics(a, b) {
		t.Error("expected different FK ref tables to be unequal")
	}
}

func TestSameConstraintSemantics_FKDifferentOnDelete(t *testing.T) {
	a := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
		OnDelete:   "CASCADE",
	}
	b := &model.Constraint{
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
		OnDelete:   "SET NULL",
	}
	if sameConstraintSemantics(a, b) {
		t.Error("expected different FK OnDelete to be unequal")
	}
}

func TestSameConstraintSemantics_CheckSameExpression(t *testing.T) {
	a := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age > 0"}
	b := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age > 0"}
	if !sameConstraintSemantics(a, b) {
		t.Error("expected same check expression to be equal")
	}
}

func TestSameConstraintSemantics_CheckDifferentExpression(t *testing.T) {
	a := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age > 0"}
	b := &model.Constraint{Type: "check", Columns: []string{"age"}, Expression: "age >= 18"}
	if sameConstraintSemantics(a, b) {
		t.Error("expected different check expressions to be unequal")
	}
}

// --- isColumnRenameCandidate ---

func TestIsColumnRenameCandidate_Matching(t *testing.T) {
	defaultExpr := "now()"
	src := &model.Column{Name: "username", DataType: "text", IsNullable: true, DefaultExpr: &defaultExpr, Collation: "en_US.UTF-8"}
	tgt := &model.Column{Name: "login_name", DataType: "text", IsNullable: true, DefaultExpr: &defaultExpr, Collation: "en_US.UTF-8"}
	if !isColumnRenameCandidate(src, tgt) {
		t.Error("expected matching columns to be rename candidates")
	}
}

func TestIsColumnRenameCandidate_TypeMismatch(t *testing.T) {
	src := &model.Column{DataType: "text", IsNullable: true}
	tgt := &model.Column{DataType: "integer", IsNullable: true}
	if isColumnRenameCandidate(src, tgt) {
		t.Error("expected type mismatch to not be rename candidate")
	}
}

func TestIsColumnRenameCandidate_NullableMismatch(t *testing.T) {
	src := &model.Column{DataType: "text", IsNullable: true}
	tgt := &model.Column{DataType: "text", IsNullable: false}
	if isColumnRenameCandidate(src, tgt) {
		t.Error("expected nullable mismatch to not be rename candidate")
	}
}

func TestIsColumnRenameCandidate_DefaultMismatch(t *testing.T) {
	a := "now()"
	b := "'2024-01-01'"
	src := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: &a}
	tgt := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: &b}
	if isColumnRenameCandidate(src, tgt) {
		t.Error("expected default mismatch to not be rename candidate")
	}
}

func TestIsColumnRenameCandidate_CollationMismatch(t *testing.T) {
	src := &model.Column{DataType: "text", IsNullable: true, Collation: "C"}
	tgt := &model.Column{DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"}
	if isColumnRenameCandidate(src, tgt) {
		t.Error("expected collation mismatch to not be rename candidate")
	}
}

func TestIsColumnRenameCandidate_NilDefaults(t *testing.T) {
	src := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: nil}
	tgt := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: nil}
	if !isColumnRenameCandidate(src, tgt) {
		t.Error("expected nil defaults to match")
	}
}

func TestIsColumnRenameCandidate_OneNilDefault(t *testing.T) {
	a := "now()"
	src := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: &a}
	tgt := &model.Column{DataType: "text", IsNullable: true, DefaultExpr: nil}
	if isColumnRenameCandidate(src, tgt) {
		t.Error("expected one nil default to not match")
	}
}

// --- sameIndexContent ---

func TestSameIndexContent_Identical(t *testing.T) {
	a := &model.Index{
		Name:        "idx_users_email",
		Unique:      true,
		Method:      "btree",
		WhereClause: "",
		Elements: []model.IndexElem{
			{Name: "email", IndexColName: "email", Collation: "C", Opclass: "text_ops", Ordering: "ASC", NullsOrdering: ""},
		},
	}
	b := &model.Index{
		Name:        "idx_users_email",
		Unique:      true,
		Method:      "btree",
		WhereClause: "",
		Elements: []model.IndexElem{
			{Name: "email", IndexColName: "email", Collation: "C", Opclass: "text_ops", Ordering: "ASC", NullsOrdering: ""},
		},
	}
	if !sameIndexContent(a, b) {
		t.Error("expected identical indexes to be equal")
	}
}

func TestSameIndexContent_DifferentUnique(t *testing.T) {
	a := &model.Index{Unique: true}
	b := &model.Index{Unique: false}
	if sameIndexContent(a, b) {
		t.Error("expected different unique to be unequal")
	}
}

func TestSameIndexContent_DifferentMethod(t *testing.T) {
	a := &model.Index{Method: "btree"}
	b := &model.Index{Method: "hash"}
	if sameIndexContent(a, b) {
		t.Error("expected different methods to be unequal")
	}
}

func TestSameIndexContent_DifferentWhereClause(t *testing.T) {
	a := &model.Index{WhereClause: "active = true"}
	b := &model.Index{WhereClause: "deleted = false"}
	if sameIndexContent(a, b) {
		t.Error("expected different where clauses to be unequal")
	}
}

func TestSameIndexContent_DifferentElementCount(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a"}, {Name: "b"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different element counts to be unequal")
	}
}

func TestSameIndexContent_DifferentElementName(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "b"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different element names to be unequal")
	}
}

func TestSameIndexContent_DifferentCollation(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a", Collation: "C"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a", Collation: "en_US.UTF-8"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different collations to be unequal")
	}
}

func TestSameIndexContent_DifferentExpr(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a", Expr: "lower(a)"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a", Expr: "upper(a)"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different expressions to be unequal")
	}
}

func TestSameIndexContent_DifferentOpclass(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a", Opclass: "text_ops"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a", Opclass: "varchar_ops"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different opclasses to be unequal")
	}
}

func TestSameIndexContent_DifferentOrdering(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a", Ordering: "ASC"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a", Ordering: "DESC"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different ordering to be unequal")
	}
}

func TestSameIndexContent_DifferentNullsOrdering(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{{Name: "a", NullsOrdering: "FIRST"}}}
	b := &model.Index{Elements: []model.IndexElem{{Name: "a", NullsOrdering: "LAST"}}}
	if sameIndexContent(a, b) {
		t.Error("expected different nulls ordering to be unequal")
	}
}

func TestSameIndexContent_EmptyElements(t *testing.T) {
	a := &model.Index{Elements: []model.IndexElem{}}
	b := &model.Index{Elements: []model.IndexElem{}}
	if !sameIndexContent(a, b) {
		t.Error("expected empty elements to be equal")
	}
}

// --- diffTableConstraints ---

func TestDiffTableConstraints_AddConstraint(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	target := model.NewTable("public", "users")
	target.Constraints["pk_users"] = &model.Constraint{Name: "pk_users", Type: "primary_key", Columns: []string{"id"}}

	ctx.diffTableConstraints("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddConstraint {
		t.Fatalf("expected KindAddConstraint, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableConstraints_DropConstraint(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Constraints["old_pk"] = &model.Constraint{Name: "old_pk", Type: "primary_key", Columns: []string{"id"}}
	target := model.NewTable("public", "users")

	ctx.diffTableConstraints("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropConstraint {
		t.Fatalf("expected KindDropConstraint, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableConstraints_ChangeConstraint(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Constraints["fk_posts"] = &model.Constraint{
		Name:       "fk_posts",
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	}
	target := model.NewTable("public", "users")
	target.Constraints["fk_posts"] = &model.Constraint{
		Name:       "fk_posts",
		Type:       "foreign_key",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "articles",
		RefColumns: []string{"id"},
	}

	ctx.diffTableConstraints("public", source, target)

	if len(ctx.ops) != 2 {
		t.Fatalf("expected 2 ops (drop+add), got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropConstraint || ctx.ops[1].Kind() != KindAddConstraint {
		t.Fatalf("expected DropConstraint+AddConstraint, got %v+%v", ctx.ops[0].Kind(), ctx.ops[1].Kind())
	}
}

func TestDiffTableConstraints_SemanticsSameSkipsChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Constraints["pk_users"] = &model.Constraint{
		Name:       "pk_users",
		Type:       "primary_key",
		Columns:    []string{"id"},
		Definition: "PRIMARY KEY (id)",
	}
	target := model.NewTable("public", "users")
	target.Constraints["pk_users"] = &model.Constraint{
		Name:       "pk_users",
		Type:       "primary_key",
		Columns:    []string{"id"},
		Definition: "PRIMARY KEY ((id))",
	}

	ctx.diffTableConstraints("public", source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops when semantics are same, got %d", len(ctx.ops))
	}
}

// --- diffTableIndexes ---

func TestDiffTableIndexes_AddIndex(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	target := model.NewTable("public", "users")
	target.Indexes["idx_users_email"] = &model.Index{Name: "idx_users_email", Table: "users"}

	ctx.diffTableIndexes("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddIndex {
		t.Fatalf("expected KindAddIndex, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableIndexes_DropIndex(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Indexes["idx_old"] = &model.Index{Name: "idx_old", Table: "users"}
	target := model.NewTable("public", "users")

	ctx.diffTableIndexes("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropIndex {
		t.Fatalf("expected KindDropIndex, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableIndexes_ChangeIndex(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Indexes["idx_users"] = &model.Index{Name: "idx_users", Unique: false}
	target := model.NewTable("public", "users")
	target.Indexes["idx_users"] = &model.Index{Name: "idx_users", Unique: true}

	ctx.diffTableIndexes("public", source, target)

	if len(ctx.ops) != 2 {
		t.Fatalf("expected 2 ops (drop+create), got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropIndex || ctx.ops[1].Kind() != KindAddIndex {
		t.Fatalf("expected DropIndex+AddIndex, got %v+%v", ctx.ops[0].Kind(), ctx.ops[1].Kind())
	}
}

func TestDiffTableIndexes_IdenticalIndexes(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.Indexes["idx_users"] = &model.Index{Name: "idx_users", Unique: true}
	target := model.NewTable("public", "users")
	target.Indexes["idx_users"] = &model.Index{Name: "idx_users", Unique: true}

	ctx.diffTableIndexes("public", source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for identical indexes, got %d", len(ctx.ops))
	}
}

// --- diffTableColumns ---

func TestDiffTableColumns_RenameColumn(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	source.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})

	target := model.NewTable("public", "users")
	target.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	target.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})

	ctx.diffTableColumns("public", source, target)

	var hasRename, hasDrop, hasAdd bool
	for _, op := range ctx.ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindDropColumn:
			hasDrop = true
		case KindAddColumn:
			hasAdd = true
		}
	}

	if !hasRename {
		t.Error("expected RenameColumn op")
	}
	if hasDrop || hasAdd {
		t.Error("expected no DropColumn/AddColumn when rename detected")
	}
}

func TestDiffTableColumns_AddColumn(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})

	target := model.NewTable("public", "users")
	target.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	target.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: true})

	ctx.diffTableColumns("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddColumn {
		t.Fatalf("expected KindAddColumn, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableColumns_DropColumn(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	source.AddColumn(&model.Column{Name: "old_col", DataType: "text", IsNullable: true})

	target := model.NewTable("public", "users")
	target.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})

	ctx.diffTableColumns("public", source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropColumn {
		t.Fatalf("expected KindDropColumn, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTableColumns_NoChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	source := model.NewTable("public", "users")
	source.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})

	target := model.NewTable("public", "users")
	target.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})

	ctx.diffTableColumns("public", source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for identical columns, got %d", len(ctx.ops))
	}
}

// --- isEnumAppend ---

func TestIsEnumAppend_True(t *testing.T) {
	src := []string{"a", "b"}
	tgt := []string{"a", "b", "c"}
	if !isEnumAppend(src, tgt) {
		t.Error("expected isEnumAppend to be true")
	}
}

func TestIsEnumAppend_FalseShorter(t *testing.T) {
	src := []string{"a", "b", "c"}
	tgt := []string{"a", "b"}
	if isEnumAppend(src, tgt) {
		t.Error("expected isEnumAppend to be false when target is shorter")
	}
}

func TestIsEnumAppend_FalseDifferentOrder(t *testing.T) {
	src := []string{"a", "b"}
	tgt := []string{"a", "c", "b"}
	if isEnumAppend(src, tgt) {
		t.Error("expected isEnumAppend to be false when order differs")
	}
}

func TestIsEnumAppend_EmptySource(t *testing.T) {
	var src []string
	tgt := []string{"a"}
	if !isEnumAppend(src, tgt) {
		t.Error("expected isEnumAppend to be true for empty source")
	}
}

// --- sameStringSlice ---

func TestSliceComparison_Equal(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c"}
	if len(a) != len(b) {
		t.Fatal("expected equal length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("element %d: got %q, want %q", i, a[i], b[i])
		}
	}
}

func TestSliceComparison_DifferentLength(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"a", "b", "c"}
	if len(a) == len(b) {
		t.Error("expected different lengths")
	}
}

func TestSliceComparison_DifferentContent(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"a", "c"}
	same := len(a) == len(b)
	if same {
		for i := range a {
			if a[i] != b[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("expected different content")
	}
}

// --- diffSchemas ---

func TestDiffSchemas_CreateAndDropSchema(t *testing.T) {
	source := model.NewSchema()
	source.Schemas["old_schema"] = model.NewNamespace("old_schema")

	target := model.NewSchema()
	target.Schemas["new_schema"] = model.NewNamespace("new_schema")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSchemas(source, target)

	kinds := map[Kind]bool{}
	for _, op := range ctx.ops {
		kinds[op.Kind()] = true
	}

	if !kinds[KindCreateSchema] {
		t.Error("expected CreateSchema op")
	}
	if !kinds[KindDropSchema] {
		t.Error("expected DropSchema op")
	}
}

func TestDiffSchemas_PublicSchemaNeverDropped(t *testing.T) {
	source := model.NewSchema()
	source.Schemas["public"] = model.NewNamespace("public")

	target := model.NewSchema()

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSchemas(source, target)

	for _, op := range ctx.ops {
		if op.Kind() == KindDropSchema {
			dropOp := op.(*DropSchemaOp)
			if dropOp.Schema == "public" {
				t.Error("public schema should never be dropped")
			}
		}
	}

	if len(ctx.warnings) == 0 {
		t.Error("expected warning about public schema drop")
	}
}

func TestDiffSchemas_IdenticalSchemas(t *testing.T) {
	source := model.NewSchema()
	source.Schemas["public"] = model.NewNamespace("public")

	target := model.NewSchema()
	target.Schemas["public"] = model.NewNamespace("public")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSchemas(source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for identical schemas, got %d", len(ctx.ops))
	}
}

// --- diffNamespace ---

func TestDiffNamespace_NilTarget(t *testing.T) {
	source := model.NewNamespace("public")
	source.Tables["users"] = model.NewTable("public", "users")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffNamespace(source, nil)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for nil target, got %d", len(ctx.ops))
	}
}

func TestDiffNamespace_NilSource(t *testing.T) {
	target := model.NewNamespace("public")
	target.Tables["users"] = model.NewTable("public", "users")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffNamespace(nil, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddTable {
		t.Fatalf("expected KindAddTable, got %v", ctx.ops[0].Kind())
	}
}

// --- diffTypes ---

func TestDiffTypes_AddType(t *testing.T) {
	source := model.NewNamespace("public")
	target := model.NewNamespace("public")
	target.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user"}}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffTypes(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddEnumType {
		t.Fatalf("expected KindAddEnumType, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTypes_DropType(t *testing.T) {
	source := model.NewNamespace("public")
	source.Types["old_type"] = &model.EnumType{Name: "old_type", Labels: []string{"a"}}
	target := model.NewNamespace("public")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffTypes(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropEnumType {
		t.Fatalf("expected KindDropEnumType, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTypes_AppendLabel(t *testing.T) {
	source := model.NewNamespace("public")
	source.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user"}}
	target := model.NewNamespace("public")
	target.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user", "guest"}}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffTypes(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddEnumLabel {
		t.Fatalf("expected KindAddEnumLabel, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffTypes_NonAppendChangeWarning(t *testing.T) {
	source := model.NewNamespace("public")
	source.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user"}}
	target := model.NewNamespace("public")
	target.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"superadmin", "user"}}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffTypes(source, target)

	if len(ctx.warnings) == 0 {
		t.Error("expected warning for non-append enum change")
	}
}

// --- diffViews ---

func TestDiffViews_AddView(t *testing.T) {
	source := model.NewNamespace("public")
	target := model.NewNamespace("public")
	target.Views["v_users"] = &model.View{Name: "v_users", Definition: "SELECT id FROM users"}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffViews(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindCreateView {
		t.Fatalf("expected KindCreateView, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffViews_DropView(t *testing.T) {
	source := model.NewNamespace("public")
	source.Views["v_old"] = &model.View{Name: "v_old", Definition: "SELECT 1"}
	target := model.NewNamespace("public")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffViews(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropView {
		t.Fatalf("expected KindDropView, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffViews_ReplaceView(t *testing.T) {
	source := model.NewNamespace("public")
	source.Views["v_users"] = &model.View{Name: "v_users", Definition: "SELECT id FROM users"}
	target := model.NewNamespace("public")
	target.Views["v_users"] = &model.View{Name: "v_users", Definition: "SELECT id, name FROM users"}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffViews(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindReplaceView {
		t.Fatalf("expected KindReplaceView, got %v", ctx.ops[0].Kind())
	}
}

// --- diffSequences ---

func TestDiffSequences_AddSequence(t *testing.T) {
	source := model.NewNamespace("public")
	target := model.NewNamespace("public")
	target.Sequences["id_seq"] = &model.Sequence{Name: "id_seq", DataType: "bigint", StartValue: 1}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSequences(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindCreateSequence {
		t.Fatalf("expected KindCreateSequence, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffSequences_DropSequence(t *testing.T) {
	source := model.NewNamespace("public")
	source.Sequences["old_seq"] = &model.Sequence{Name: "old_seq", DataType: "bigint", StartValue: 1}
	target := model.NewNamespace("public")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSequences(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropSequence {
		t.Fatalf("expected KindDropSequence, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffSequences_AlterSequence(t *testing.T) {
	source := model.NewNamespace("public")
	source.Sequences["id_seq"] = &model.Sequence{Name: "id_seq", DataType: "bigint", StartValue: 1}
	target := model.NewNamespace("public")
	target.Sequences["id_seq"] = &model.Sequence{Name: "id_seq", DataType: "bigint", StartValue: 100}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffSequences(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAlterSequence {
		t.Fatalf("expected KindAlterSequence, got %v", ctx.ops[0].Kind())
	}
}

// --- diffExtensions ---

func TestDiffExtensions_AddExtension(t *testing.T) {
	source := model.NewNamespace("public")
	target := model.NewNamespace("public")
	target.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.3"}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffExtensions(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindCreateExtension {
		t.Fatalf("expected KindCreateExtension, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffExtensions_DropExtension(t *testing.T) {
	source := model.NewNamespace("public")
	source.Extensions["old_ext"] = &model.Extension{Name: "old_ext", Version: "1.0"}
	target := model.NewNamespace("public")

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffExtensions(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropExtension {
		t.Fatalf("expected KindDropExtension, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffExtensions_UpdateExtension(t *testing.T) {
	source := model.NewNamespace("public")
	source.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.2"}
	target := model.NewNamespace("public")
	target.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.3"}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffExtensions(source, target)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAlterExtensionUpdate {
		t.Fatalf("expected KindAlterExtensionUpdate, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffExtensions_NoChangeWhenVersionEmpty(t *testing.T) {
	source := model.NewNamespace("public")
	source.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: ""}
	target := model.NewNamespace("public")
	target.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: ""}

	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	ctx.diffExtensions(source, target)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops when versions are empty, got %d", len(ctx.ops))
	}
}

// --- sameSequenceContent ---

func TestSameSequenceContent_Identical(t *testing.T) {
	a := &model.Sequence{Name: "s", DataType: "bigint", StartValue: 1, IncrementBy: 1, MinValue: 1, MaxValue: 100, CacheSize: 1, Cycle: false}
	b := &model.Sequence{Name: "s", DataType: "bigint", StartValue: 1, IncrementBy: 1, MinValue: 1, MaxValue: 100, CacheSize: 1, Cycle: false}
	if !sameSequenceContent(a, b) {
		t.Error("expected identical sequences to be equal")
	}
}

func TestSameSequenceContent_DifferentStartValue(t *testing.T) {
	a := &model.Sequence{StartValue: 1}
	b := &model.Sequence{StartValue: 100}
	if sameSequenceContent(a, b) {
		t.Error("expected different start values to be unequal")
	}
}

func TestSameSequenceContent_DifferentCycle(t *testing.T) {
	a := &model.Sequence{Cycle: true}
	b := &model.Sequence{Cycle: false}
	if sameSequenceContent(a, b) {
		t.Error("expected different cycle to be unequal")
	}
}

// --- ObjectKey equality ---

func TestObjectKey_Equality(t *testing.T) {
	a := model.NewObjectKey("public", "users", model.KindTable)
	b := model.NewObjectKey("public", "users", model.KindTable)
	if !reflect.DeepEqual(a, b) {
		t.Error("expected identical ObjectKeys to be equal")
	}

	c := model.NewObjectKey("public", "posts", model.KindTable)
	if reflect.DeepEqual(a, c) {
		t.Error("expected different ObjectKeys to be unequal")
	}
}

func TestObjectKey_FunctionKey(t *testing.T) {
	a := model.NewFunctionKey("public", "my_func", "integer, text")
	if a.Kind != model.KindFunction {
		t.Errorf("expected KindFunction, got %v", a.Kind)
	}
	if a.Signature != "integer, text" {
		t.Errorf("expected signature 'integer, text', got '%s'", a.Signature)
	}
}
