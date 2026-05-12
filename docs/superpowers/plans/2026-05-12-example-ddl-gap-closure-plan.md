# 示例 DDL 缺口补齐实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让 `migra diff testdata/example_source.sql testdata/example_target.sql` 输出覆盖示例差异的合理 SQL，并消除示例 enum unsupported warning。

**架构：** 解析层只提取结构化语义，模型层保存 enum、constraint、default 的可比较字段，diff 层生成强类型 operation，render 层统一组装 SQL。实现保持现有 HandlerRegistry + MutationApplier 结构，不重构主流程。

**技术栈：** Go 1.26、pg_query_go、Cobra/Viper、testify、RTK 命令包装。

---

## 文件结构

- 修改：`internal/model/table.go`，扩展 `Constraint` 结构化字段。
- 修改：`internal/parser/parserutil/util.go`，增强默认值表达式解析，供 create/alter 共用。
- 修改：`internal/parser/mutation.go`，让 `CreateTableMutation` 携带并应用主键和约束，让 `CreateEnumTypeMutation` 写入模型。
- 修改：`internal/parser/create_table_handler.go`，解析 enum 之外的建表约束语义。
- 修改：`internal/parser/enum_handler.go`，实现 `CREATE TYPE ... AS ENUM` handler。
- 修改：`internal/diff/operation.go`，新增 enum label、default、drop column operation。
- 修改：`internal/diff/diff_columns.go`、`internal/diff/diff_tables.go`、`internal/diff/differ.go`，生成新增 operation 与 warning。
- 修改：`internal/plan/plan.go`，为新增 operation 分配阶段。
- 修改：`internal/render/render.go`，渲染结构化约束、新 operation 和建表约束。
- 修改测试：`internal/parser/parser_test.go`、`internal/parser/mutation_test.go`、`internal/diff/differ_test.go`、`internal/diff/operation_test.go`、`internal/render/render_test.go`、`cmd/migra/integration_test.go`。

## 任务 1：模型扩展与 mutation 写入约束

**文件：**
- 修改：`internal/model/table.go`
- 修改：`internal/parser/mutation.go`
- 测试：`internal/parser/mutation_test.go`

- [ ] **步骤 1：编写失败的 mutation 测试**

在 `internal/parser/mutation_test.go` 增加：

```go
func TestCreateTableMutation_Apply_PrimaryKeyAndConstraints(t *testing.T) {
	m := CreateTableMutation{
		Schema: "public",
		Name:   "comments",
		Columns: []model.Column{
			{Name: "id", DataType: "serial", IsNullable: false},
			{Name: "post_id", DataType: "integer", IsNullable: false},
		},
		PrimaryKey: &model.PrimaryKey{Name: "comments_pkey", Columns: []string{"id"}},
		Constraints: []model.Constraint{
			{
				Name:       "comments_post_id_fkey",
				Type:       "foreign_key",
				Table:      "comments",
				Columns:    []string{"post_id"},
				RefSchema:  "public",
				RefTable:   "posts",
				RefColumns: []string{"id"},
			},
		},
	}

	schema := model.NewSchema()
	if err := m.Apply(schema); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["comments"]
	if table.PrimaryKey == nil || table.PrimaryKey.Name != "comments_pkey" {
		t.Fatalf("expected primary key to be applied, got %#v", table.PrimaryKey)
	}
	fk := table.Constraints["comments_post_id_fkey"]
	if fk == nil {
		t.Fatal("expected foreign key constraint")
	}
	if fk.RefSchema != "public" || fk.RefTable != "posts" || len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Fatalf("unexpected foreign key target: %#v", fk)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/parser -run TestCreateTableMutation_Apply_PrimaryKeyAndConstraints -count=1`

预期：FAIL，编译错误包含 `unknown field PrimaryKey` 或 `fk.RefSchema undefined`。

- [ ] **步骤 3：实现模型字段和 mutation 应用**

在 `internal/model/table.go` 将 `Constraint` 改为：

```go
type Constraint struct {
	Name       string
	Type       string
	Definition string
	Table      string
	Columns    []string
	RefSchema  string
	RefTable   string
	RefColumns []string
	Expression string
}
```

在 `internal/parser/mutation.go` 将 `CreateTableMutation` 改为：

```go
type CreateTableMutation struct {
	Schema      string
	Name        string
	Columns     []model.Column
	PrimaryKey  *model.PrimaryKey
	Constraints []model.Constraint
}
```

在 `CreateTableMutation.Apply` 创建或合并表后写入：

```go
func applyTableMetadata(table *model.Table, primaryKey *model.PrimaryKey, constraints []model.Constraint) {
	if primaryKey != nil {
		table.PrimaryKey = primaryKey
	}
	for i := range constraints {
		c := constraints[i]
		if c.Table == "" {
			c.Table = table.Name
		}
		table.Constraints[c.Name] = &c
	}
}
```

在 placeholder merge 分支和新建表分支都调用 `applyTableMetadata(table, m.PrimaryKey, m.Constraints)`。

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/parser -run TestCreateTableMutation_Apply_PrimaryKeyAndConstraints -count=1`

预期：PASS。

- [ ] **步骤 5：Commit**

运行：

```bash
rtk git add internal/model/table.go internal/parser/mutation.go internal/parser/mutation_test.go
rtk git commit -m "feat(model): 支持结构化表约束"
```

## 任务 2：默认值表达式解析一致性

**文件：**
- 修改：`internal/parser/parserutil/util.go`
- 测试：`internal/parser/parser_test.go`

- [ ] **步骤 1：编写失败的 parser 测试**

在 `internal/parser/parser_test.go` 增加：

```go
func TestParseDefaultFunctionExpressionConsistentForCreateAndAlter(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE events (
			id integer,
			created_at timestamp DEFAULT now()
		);
		ALTER TABLE events ADD COLUMN updated_at timestamp DEFAULT now();
	`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["events"]
	created := table.ColumnByName["created_at"]
	updated := table.ColumnByName["updated_at"]
	if created.DefaultExpr == nil || *created.DefaultExpr != "now()" {
		t.Fatalf("expected created_at default now(), got %#v", created.DefaultExpr)
	}
	if updated.DefaultExpr == nil || *updated.DefaultExpr != "now()" {
		t.Fatalf("expected updated_at default now(), got %#v", updated.DefaultExpr)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/parser -run TestParseDefaultFunctionExpressionConsistentForCreateAndAlter -count=1`

预期：FAIL，`DefaultExpr` 为 nil 或空字符串。

- [ ] **步骤 3：增强共享表达式解析**

在 `internal/parser/parserutil/util.go` 修改 `ParseExpression`，常量分支保留，新增 deparse fallback：

```go
func ParseExpression(expr pg_nodes.Node) (string, bool) {
	switch e := expr.(type) {
	case pg_nodes.A_Const:
		if e.Val != nil {
			switch v := e.Val.(type) {
			case pg_nodes.String:
				return "'" + v.Str + "'", true
			case pg_nodes.Integer:
				return fmt.Sprintf("%d", v.Ival), true
			case pg_nodes.Float:
				return v.Str, true
			}
		}
	}
	if d, ok := expr.(interface{ Deparse() string }); ok {
		var out string
		func() {
			defer func() {
				if recover() != nil {
					out = ""
				}
			}()
			out = d.Deparse()
		}()
		out = strings.TrimSpace(out)
		if out != "" {
			return out, true
		}
	}
	return "", false
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/parser -run TestParseDefaultFunctionExpressionConsistentForCreateAndAlter -count=1`

预期：PASS。

- [ ] **步骤 5：Commit**

运行：

```bash
rtk git add internal/parser/parserutil/util.go internal/parser/parser_test.go
rtk git commit -m "feat(parser): 解析函数默认值"
```

## 任务 3：解析 enum 与建表约束

**文件：**
- 修改：`internal/parser/enum_handler.go`
- 修改：`internal/parser/create_table_handler.go`
- 修改：`internal/parser/mutation.go`
- 测试：`internal/parser/parser_test.go`

- [ ] **步骤 1：编写失败的 enum 和 constraint 测试**

在 `internal/parser/parser_test.go` 替换跳过的 `TestParseCreateEnumType`，并新增建表约束测试：

```go
func TestParseCreateEnumType(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`CREATE TYPE user_role AS ENUM ('admin', 'user');`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	enumType := schema.Schemas["public"].Types["user_role"]
	if enumType == nil {
		t.Fatal("expected enum type")
	}
	if strings.Join(enumType.Labels, ",") != "admin,user" {
		t.Fatalf("unexpected enum labels: %#v", enumType.Labels)
	}
}

func TestParseCreateTablePrimaryKeyAndForeignKey(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE posts (id integer PRIMARY KEY);
		CREATE TABLE comments (
			id integer PRIMARY KEY,
			post_id integer NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id)
		);
	`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	comments := schema.Schemas["public"].Tables["comments"]
	if comments.PrimaryKey == nil || strings.Join(comments.PrimaryKey.Columns, ",") != "id" {
		t.Fatalf("expected comments primary key, got %#v", comments.PrimaryKey)
	}
	fk := comments.Constraints["comments_post_id_fkey"]
	if fk == nil {
		t.Fatal("expected generated foreign key constraint")
	}
	if fk.Type != "foreign_key" || fk.RefSchema != "public" || fk.RefTable != "posts" || strings.Join(fk.RefColumns, ",") != "id" {
		t.Fatalf("unexpected foreign key: %#v", fk)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/parser -run 'TestParseCreateEnumType|TestParseCreateTablePrimaryKeyAndForeignKey' -count=1`

预期：FAIL，enum unsupported 或缺失 primary/foreign key。

- [ ] **步骤 3：实现 enum handler 和 mutation**

在 `internal/parser/enum_handler.go` 中解析 `pg_nodes.CreateEnumStmt`：

```go
func (h *CreateEnumHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt, ok := node.(pg_nodes.CreateEnumStmt)
	if !ok {
		return nil, fmt.Errorf("CreateEnumHandler: expected pg_nodes.CreateEnumStmt, got %T", node)
	}
	schemaName := "public"
	typeName := ""
	if len(stmt.TypeName.Items) > 0 {
		parts := make([]string, 0, len(stmt.TypeName.Items))
		for _, item := range stmt.TypeName.Items {
			if s, ok := item.(pg_nodes.String); ok {
				parts = append(parts, s.Str)
			}
		}
		if len(parts) == 1 {
			typeName = parts[0]
		} else if len(parts) >= 2 {
			schemaName = parts[len(parts)-2]
			typeName = parts[len(parts)-1]
		}
	}
	labels := make([]string, 0, len(stmt.Vals.Items))
	for _, item := range stmt.Vals.Items {
		if s, ok := item.(pg_nodes.String); ok {
			labels = append(labels, s.Str)
		}
	}
	return []SchemaMutation{CreateEnumTypeMutation{Schema: schemaName, Name: typeName, Labels: labels}}, nil
}
```

在 `CreateEnumTypeMutation.Apply` 写入 `ns.Types[m.Name] = &model.EnumType{Name: m.Name, Labels: m.Labels}`，重复 type 返回错误。

- [ ] **步骤 4：实现 create table 约束解析**

在 `internal/parser/create_table_handler.go` 解析 `pg_nodes.Constraint` table element 和 column constraints。新增小函数：

```go
func defaultConstraintName(table string, constraint model.Constraint) string {
	if constraint.Name != "" {
		return constraint.Name
	}
	if constraint.Type == "primary_key" {
		return table + "_pkey"
	}
	if constraint.Type == "foreign_key" && len(constraint.Columns) > 0 {
		return table + "_" + strings.Join(constraint.Columns, "_") + "_fkey"
	}
	return table + "_constraint"
}
```

在 handler 中构造 `CreateTableMutation{Schema: schemaName, Name: tableName, Columns: columns, PrimaryKey: primaryKey, Constraints: constraints}`。实现时从 AST 字段读取本表列、引用表和引用列；如果 `RangeVar.Schemaname` 为空，`RefSchema` 使用 `"public"`。

- [ ] **步骤 5：运行测试验证通过**

运行：`rtk go test ./internal/parser -run 'TestParseCreateEnumType|TestParseCreateTablePrimaryKeyAndForeignKey' -count=1`

预期：PASS。

- [ ] **步骤 6：Commit**

运行：

```bash
rtk git add internal/parser/enum_handler.go internal/parser/create_table_handler.go internal/parser/mutation.go internal/parser/parser_test.go
rtk git commit -m "feat(parser): 解析 enum 和表约束"
```

## 任务 4：新增 diff operation 与比较逻辑

**文件：**
- 修改：`internal/diff/operation.go`
- 修改：`internal/diff/diff_columns.go`
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/diff/differ.go`
- 测试：`internal/diff/operation_test.go`
- 测试：`internal/diff/differ_test.go`

- [ ] **步骤 1：编写失败的 operation 和 diff 测试**

在 `internal/diff/operation_test.go` 增加：

```go
func TestNewOperationsMetadata(t *testing.T) {
	enumOp := NewAddEnumLabelOp("public", "user_role", "guest")
	if enumOp.Kind() != KindAddEnumLabel || enumOp.IsDestructive() {
		t.Fatalf("unexpected enum label op metadata")
	}
	if len(enumOp.DependsOn()) != 1 || enumOp.DependsOn()[0].Kind != model.KindType {
		t.Fatalf("expected enum type dependency, got %#v", enumOp.DependsOn())
	}

	dropCol := NewDropColumnOp("public", "users", "old_col")
	if !dropCol.IsDestructive() {
		t.Fatal("drop column must be destructive")
	}
}
```

在 `internal/diff/differ_test.go` 增加 enum/default/drop column 测试：

```go
func TestDiffer_EnumLabelAppendDefaultChangeAndDropColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceNs.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user"}}
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "old_col", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user", "guest"}}
	targetTable := model.NewTable("public", "users")
	defaultExpr := "now()"
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false, DefaultExpr: &defaultExpr})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	kinds := map[Kind]bool{}
	for _, op := range ops {
		kinds[op.Kind()] = true
	}
	if !kinds[KindAddEnumLabel] || !kinds[KindSetDefault] || !kinds[KindDropColumn] {
		t.Fatalf("missing expected operations, got %#v", kinds)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/diff -run 'TestNewOperationsMetadata|TestDiffer_EnumLabelAppendDefaultChangeAndDropColumn' -count=1`

预期：FAIL，缺少 `KindAddEnumLabel`、`NewDropColumnOp` 或 default operation。

- [ ] **步骤 3：实现 operation 类型**

在 `internal/diff/operation.go` 增加 kind：

```go
KindAddEnumLabel Kind = "add_enum_label"
KindSetDefault   Kind = "set_default"
KindDropDefault  Kind = "drop_default"
```

保留现有 `KindDropColumn`，补齐 `DropColumnOp`。新增：

```go
type AddEnumLabelOp struct {
	baseOperation
	Schema string
	Type   string
	Label  string
}
```

`DependsOn` 返回 `model.NewObjectKey(op.Schema, op.Type, model.KindType)`。新增 `SetDefaultOp`、`DropDefaultOp`，字段包含 `Schema`、`Table`、`Column`、`DefaultExpr`。

- [ ] **步骤 4：实现 diff 逻辑**

在 `diffColumn` 中将 default warning 替换为：

```go
if !sameDefault(source.DefaultExpr, target.DefaultExpr) {
	if target.DefaultExpr == nil {
		c.addOp(NewDropDefaultOp(schema, table, source.Name))
	} else {
		c.addOp(NewSetDefaultOp(schema, table, source.Name, *target.DefaultExpr))
	}
}
```

在 `diffTableColumns` 中将 drop column warning 替换为 `c.addOp(NewDropColumnOp(schema, source.Name, name))`。

在 `diffTypes` 中同名 enum 比较时只处理尾部追加：

```go
if isEnumAppend(sourceType.Labels, targetType.Labels) {
	for _, label := range targetType.Labels[len(sourceType.Labels):] {
		c.addOp(NewAddEnumLabelOp(target.Name, name, label))
	}
} else if !sameStringSlice(sourceType.Labels, targetType.Labels) {
	c.warnf("enum %s.%s change is not append-only and is not implemented", target.Name, name)
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`rtk go test ./internal/diff -run 'TestNewOperationsMetadata|TestDiffer_EnumLabelAppendDefaultChangeAndDropColumn' -count=1`

预期：PASS。

- [ ] **步骤 6：Commit**

运行：

```bash
rtk git add internal/diff/operation.go internal/diff/diff_columns.go internal/diff/diff_tables.go internal/diff/differ.go internal/diff/operation_test.go internal/diff/differ_test.go
rtk git commit -m "feat(diff): 生成 enum 默认值和列删除操作"
```

## 任务 5：渲染结构化约束和新增 operation

**文件：**
- 修改：`internal/render/render.go`
- 测试：`internal/render/render_test.go`

- [ ] **步骤 1：编写失败的 render 测试**

在 `internal/render/render_test.go` 增加：

```go
func TestRenderAddTable_WithDefaultsAndConstraints(t *testing.T) {
	defaultExpr := "now()"
	table := model.NewTable("public", "comments")
	table.AddColumn(&model.Column{Name: "id", DataType: "serial", IsNullable: false})
	table.AddColumn(&model.Column{Name: "post_id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: &defaultExpr})
	table.PrimaryKey = &model.PrimaryKey{Name: "comments_pkey", Columns: []string{"id"}}
	table.Constraints["comments_post_id_fkey"] = &model.Constraint{
		Name:       "comments_post_id_fkey",
		Type:       "foreign_key",
		Table:      "comments",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "comments", table))
	for _, want := range []string{
		`"created_at" timestamp DEFAULT now()`,
		`CONSTRAINT "comments_pkey" PRIMARY KEY ("id")`,
		`CONSTRAINT "comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."posts" ("id")`,
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderNewOperations(t *testing.T) {
	r := NewRenderer()
	cases := map[string]diff.Operation{
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest';`: diff.NewAddEnumLabelOp("public", "user_role", "guest"),
		`ALTER TABLE "public"."users" ALTER COLUMN "created_at" SET DEFAULT now();`: diff.NewSetDefaultOp("public", "users", "created_at", "now()"),
		`ALTER TABLE "public"."users" ALTER COLUMN "created_at" DROP DEFAULT;`: diff.NewDropDefaultOp("public", "users", "created_at"),
		`ALTER TABLE "public"."users" DROP COLUMN IF EXISTS "old_col";`: diff.NewDropColumnOp("public", "users", "old_col"),
	}
	for want, op := range cases {
		if got := r.Render(op); !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/render -run 'TestRenderAddTable_WithDefaultsAndConstraints|TestRenderNewOperations' -count=1`

预期：FAIL，缺少约束或新增 operation 渲染。

- [ ] **步骤 3：实现 render**

在 `Render` switch 中加入 `AddEnumLabelOp`、`SetDefaultOp`、`DropDefaultOp`、`DropColumnOp`。新增 helper：

```go
func renderConstraint(c *model.Constraint) string {
	switch c.Type {
	case "primary_key":
		return fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)", quoteIdentifier(c.Name), quoteIdentifierList(c.Columns))
	case "foreign_key":
		return fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			quoteIdentifier(c.Name),
			quoteIdentifierList(c.Columns),
			quoteQualifiedIdentifier(c.RefSchema, c.RefTable),
			quoteIdentifierList(c.RefColumns),
		)
	case "check":
		if c.Expression != "" {
			return fmt.Sprintf("CONSTRAINT %s CHECK (%s)", quoteIdentifier(c.Name), c.Expression)
		}
	}
	if c.Definition != "" {
		return fmt.Sprintf("CONSTRAINT %s %s", quoteIdentifier(c.Name), c.Definition)
	}
	return ""
}
```

`renderAddTable` 追加 `table.PrimaryKey` 和 sorted `table.Constraints`，避免重复渲染同名 primary key。

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/render -run 'TestRenderAddTable_WithDefaultsAndConstraints|TestRenderNewOperations' -count=1`

预期：PASS。

- [ ] **步骤 5：Commit**

运行：

```bash
rtk git add internal/render/render.go internal/render/render_test.go
rtk git commit -m "feat(render): 渲染约束和新增操作"
```

## 任务 6：planner 安全策略接入

**文件：**
- 修改：`internal/plan/plan.go`
- 测试：`internal/plan/plan_test.go`

- [ ] **步骤 1：编写失败的 planner 测试**

在 `internal/plan/plan_test.go` 增加：

```go
func TestPlanner_NewOperationStages(t *testing.T) {
	ops := []diff.Operation{
		diff.NewAddEnumLabelOp("public", "user_role", "guest"),
		diff.NewSetDefaultOp("public", "users", "created_at", "now()"),
		diff.NewDropDefaultOp("public", "users", "created_at"),
		diff.NewDropColumnOp("public", "users", "old_col"),
	}

	safeStages := NewPlanner(false).Plan(ops)
	if len(safeStages[StageDeploy]) != 3 {
		t.Fatalf("expected three deploy ops, got %#v", safeStages[StageDeploy])
	}
	if len(safeStages[StagePostDeploy]) != 0 {
		t.Fatalf("drop column must be skipped without unsafe-drop")
	}

	unsafeStages := NewPlanner(true).Plan(ops)
	if len(unsafeStages[StagePostDeploy]) != 1 {
		t.Fatalf("expected drop column in post-deploy with unsafe-drop")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/plan -run TestPlanner_NewOperationStages -count=1`

预期：FAIL，新增 kind 未分配到预期阶段。

- [ ] **步骤 3：实现 planner 分支**

在 `assignStage` 中将 `KindAddEnumLabel`、`KindSetDefault`、`KindDropDefault` 归入 deploy；将 `KindDropColumn` 归入 post-deploy 且受 `unsafeDrops` 控制。

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/plan -run TestPlanner_NewOperationStages -count=1`

预期：PASS。

- [ ] **步骤 5：Commit**

运行：

```bash
rtk git add internal/plan/plan.go internal/plan/plan_test.go
rtk git commit -m "feat(plan): 安排新增操作阶段"
```

## 任务 7：示例 CLI 端到端验收

**文件：**
- 修改：`cmd/migra/integration_test.go`

- [ ] **步骤 1：编写失败的端到端测试**

在 `cmd/migra/integration_test.go` 增加 imports：`context` 和 `github.com/fred29910/migra-go/internal/app`。然后增加测试：

```go
func TestExampleSQLFilesDiffIncludesEnumAndConstraints(t *testing.T) {
	out, warns, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/example_source.sql",
		Target:     "../../testdata/example_target.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`CREATE TABLE "public"."comments"`,
		`ALTER TABLE "public"."users" ADD COLUMN "age" integer`,
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`,
		`CONSTRAINT "comments_pkey" PRIMARY KEY ("id")`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	for _, warning := range warns {
		if strings.Contains(warning, "CREATE TYPE ENUM is not yet supported") {
			t.Fatalf("unexpected enum unsupported warning: %s", warning)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败或暴露输出注入问题**

运行：`rtk go test ./cmd/migra -run TestExampleSQLFilesDiffIncludesEnumAndConstraints -count=1`

预期：FAIL，缺少 enum label 或 constraint。

- [ ] **步骤 3：完成端到端测试接入**

如果测试失败是因为 parser、diff 或 render 尚未串联完整，回到对应任务修正实现后重新运行本测试。该测试必须通过 `app.NewDiffService(newDefaultDeps()).Run` 获取输出，不读取全局 stdout。

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./cmd/migra -run TestExampleSQLFilesDiffIncludesEnumAndConstraints -count=1`

预期：PASS。

- [ ] **步骤 5：Commit**

运行：

```bash
rtk git add cmd/migra/integration_test.go
rtk git commit -m "test(cli): 覆盖示例 SQL 差异输出"
```

## 任务 8：全量验证与文档一致性

**文件：**
- 可修改：`README.md`
- 可修改：`docs/superpowers/specs/2026-05-12-example-ddl-gap-closure-design.md`

- [ ] **步骤 1：运行全量测试**

运行：`rtk go test ./...`

预期：全部 Go 测试通过。

- [ ] **步骤 2：运行静态检查**

运行：`rtk go vet ./...`

预期：无 vet 问题。

- [ ] **步骤 3：构建 CLI**

运行：`rtk make build`

预期：生成 `./migra`，命令退出码为 0。

- [ ] **步骤 4：手动验收示例输出**

运行：`rtk ./migra diff testdata/example_source.sql testdata/example_target.sql`

预期输出包含：

```sql
CREATE TABLE "public"."comments"
ALTER TABLE "public"."users" ADD COLUMN "age" integer
ALTER TYPE "public"."user_role" ADD VALUE 'guest';
```

预期 stderr 不包含：

```text
CREATE TYPE ENUM is not yet supported
```

- [ ] **步骤 5：检查文档是否仍准确**

运行：`rtk grep 'CREATE TYPE ENUM is not yet supported|column drop is not implemented|default change is not implemented' internal README.md docs/superpowers/specs/2026-05-12-example-ddl-gap-closure-design.md`

预期：生产代码中不再出现 default/drop column 的 not implemented 文案；spec 中可保留历史背景说明但不能描述当前行为为 unsupported。

- [ ] **步骤 6：Commit 验证修正**

如果步骤 5 需要调整 README 或 spec，运行：

```bash
rtk git add README.md docs/superpowers/specs/2026-05-12-example-ddl-gap-closure-design.md
rtk git commit -m "docs: 同步示例 DDL 支持范围"
```

如果无文档修改，跳过 commit。
