# DDL 支持矩阵修正与实现补齐计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将 `docs/DDL.md` 与当前实现对齐，并补齐 `docs/plan/feat_ext.md` 中「需要修正文档或补齐实现的特性」列出的 8 个缺口。

**架构：** 按现有五阶段流水线推进：Parser 将 DDL AST 转为 `SchemaMutation`，Mutation 落入 `model.Schema`，Diff 生成 `Operation`，Render 输出 SQL，Introspect 从数据库恢复 schema 状态。先修正文档口径，再补低风险解析与规范化能力，最后处理高级索引和 schema 删除这类跨层能力。

**技术栈：** Go、pg_query_go v6、pgx v5、PostgreSQL catalog / information_schema、现有 `rtk` 命令包装器。

---

## 范围与优先级

本计划覆盖以下 8 项：

1. 修正 `docs/DDL.md` 中偏乐观的支持状态。
2. 补齐 `UNIQUE` / `CHECK` 约束的 SQL 文件解析与渲染。
3. 支持 `ALTER TABLE ... ADD/DROP CONSTRAINT` 的 SQL 文件解析。
4. 补齐 `character(n)` / `char(n)` 类型规范化。
5. 修正 identity 变更的渲染语义。
6. 支持 `ALTER COLUMN TYPE ... USING` 的解析与渲染。
7. 补齐高级索引渲染，并规划高级索引 introspect。
8. 支持受控 `DROP SCHEMA` 与执行阶段归类。

高级索引 introspect 变更风险较高，放在独立任务中，不与渲染补齐混在同一个提交里。

## 文件结构与职责

| 文件 | 职责 | 计划内改动 |
| --- | --- | --- |
| `docs/DDL.md` | PostgreSQL DDL 支持矩阵 | 修正支持状态和已知限制描述 |
| `internal/parser/create_table_handler.go` | `CREATE TABLE` 解析 | 提取 `UNIQUE` / `CHECK` 约束 |
| `internal/parser/alter_table_handler.go` | `ALTER TABLE` 解析 | 处理 add/drop constraint、保留 `USING` |
| `internal/parser/parserutil/util.go` | 类型、表达式、列定义解析工具 | 增加约束解析辅助函数和表达式反解析辅助 |
| `internal/parser/mutation.go` | SQL 文件解析后的 schema mutation | 新增 constraint mutation，扩展 alter type mutation |
| `internal/parser/index_handler.go` | `CREATE INDEX` 解析 | 提取 opclass、collation、可读表达式、predicate |
| `internal/model/table.go` | Table / Index / Constraint 模型 | 必要时增加 index definition 字段 |
| `internal/model/index_elem.go` | Index element 模型 | 复用现有 opclass、collation、ordering 字段 |
| `internal/diff/operation.go` | Diff operation 定义 | 增加 `AddIdentityOp`，扩展 `AlterColumnTypeOp` |
| `internal/diff/diff_columns.go` | 列级差异检测 | 区分 add identity、set identity、drop identity |
| `internal/diff/differ.go` | namespace / table / type diff | 生成 `DropSchemaOp` |
| `internal/diff/diff_tables.go` | table constraint / index diff | 使用高级索引字段比较 |
| `internal/render/render.go` | SQL 渲染 | 渲染 unique、using、identity、index、高级 schema 操作 |
| `internal/introspect/indexes.go` | 数据库索引内省 | 增强高级索引定义读取 |
| `internal/normalize/normalize.go` | schema 规范化 | 增加 `character(n)` 映射 |
| `internal/plan/plan.go` | 执行计划阶段 | 明确 schema 和 identity 操作阶段 |
| `internal/app/diff_service.go` | destructive op 过滤 | 确认 `DropSchemaOp` 受 `--unsafe-drop` 控制 |

## 任务 0：修正 DDL 支持矩阵

**文件：**
- 修改：`docs/DDL.md`
- 修改：`docs/plan/feat_ext.md`

- [ ] **步骤 0.1：更新 `ALTER COLUMN TYPE ... USING` 状态**

将 `docs/DDL.md` 中表操作章节的 `ALTER TABLE ... ALTER COLUMN TYPE` 行从「含 `USING`」改为「基础类型变更支持，`USING` 部分支持」：

```markdown
| `ALTER TABLE ... ALTER COLUMN TYPE` | ⚠️ (基础类型支持，`USING` 待补齐) | ✅ | ⚠️ (`USING` 待补齐) | ✅ | ⚠️ |
```

- [ ] **步骤 0.2：更新约束解析状态**

将 `UNIQUE` / `CHECK` 相关行的 Parser 状态调整为部分支持，直到任务 1 和任务 2 完成：

```markdown
| `UNIQUE` 约束 (表级) | ⚠️ (DB introspect 支持，SQL 文件解析待补齐) | ✅ | ✅ | ✅ | ⚠️ |
| `UNIQUE` 约束 (行内) | ⚠️ (DB introspect 支持，SQL 文件解析待补齐) | ✅ | ✅ | ✅ | ⚠️ |
| `CHECK` 约束 (表级) | ⚠️ (DB introspect 支持，SQL 文件解析待补齐) | ✅ | ✅ | ✅ | ⚠️ |
| `CHECK` 约束 (行内) | ⚠️ (DB introspect 支持，SQL 文件解析待补齐) | ✅ | ✅ | ✅ | ⚠️ |
```

- [ ] **步骤 0.3：更新高级索引状态**

将索引方法、表达式索引、部分索引、opclass、排序、`CONCURRENTLY`、`IF NOT EXISTS` 的 Renderer 状态调整为部分支持：

```markdown
| 索引方法 (`USING btree/hash/gin/gist/brin`) | ✅ | ✅ | ⚠️ (待输出 `USING`) | ✅ | ⚠️ |
| 表达式索引 (`ON tbl (lower(col))`) | ⚠️ (表达式需可读反解析) | ✅ | ⚠️ | ❌ | ⚠️ |
| 部分索引 (`WHERE` 子句) | ⚠️ (predicate 需可读反解析) | ✅ | ⚠️ | ❌ | ⚠️ |
| 操作符类 (`text_pattern_ops`, 等) | ⚠️ (待提取 opclass) | ✅ | ⚠️ | ❌ | ⚠️ |
| 排序规则 (`ASC`/`DESC`, `NULLS FIRST/LAST`) | ✅ | ✅ | ⚠️ | ❌ | ⚠️ |
| `CONCURRENTLY` | ✅ | ✅ | ⚠️ | ❌ | ⚠️ |
| `IF NOT EXISTS` | ✅ | ✅ | ⚠️ | — | ⚠️ |
```

- [ ] **步骤 0.4：运行文档空白检查**

运行：

```bash
rtk git diff --check
```

预期：退出码为 0。

- [ ] **步骤 0.5：提交**

```bash
rtk git add docs/DDL.md docs/plan/feat_ext.md
rtk git commit -m "docs(DDL): 修正特性支持矩阵"
```

## 任务 1：补齐 `UNIQUE` / `CHECK` 约束解析与渲染

**文件：**
- 修改：`internal/parser/create_table_handler.go`
- 修改：`internal/parser/parserutil/util.go`
- 修改：`internal/render/render.go`
- 修改：`internal/parser/handler_test.go`
- 修改：`internal/render/render_test.go`

- [ ] **步骤 1.1：编写失败的 parser 测试**

在 `internal/parser/handler_test.go` 添加：

```go
func TestCreateTableHandler_UniqueAndCheckConstraints(t *testing.T) {
	sql := `CREATE TABLE users (
		id integer PRIMARY KEY,
		email text UNIQUE,
		age integer CHECK (age > 0),
		CONSTRAINT users_email_lower_uniq UNIQUE (email),
		CONSTRAINT users_age_check CHECK (age < 150)
	)`
	node := mustParseFirstStmt(t, sql)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut := mutations[0].(CreateTableMutation)
	var uniqueCount, checkCount int
	for _, c := range mut.Constraints {
		switch c.Type {
		case "unique":
			uniqueCount++
			assert.NotEmpty(t, c.Columns)
		case "check":
			checkCount++
			assert.NotEmpty(t, c.Expression)
		}
		assert.NotEmpty(t, c.Name)
	}
	assert.Equal(t, 2, uniqueCount)
	assert.Equal(t, 2, checkCount)
}
```

运行：

```bash
rtk go test ./internal/parser -run TestCreateTableHandler_UniqueAndCheckConstraints -v
```

预期：失败，当前 `CreateTableHandler` 不生成 `unique` / `check` 约束。

- [ ] **步骤 1.2：添加 constraint 解析辅助函数**

在 `internal/parser/parserutil/util.go` 添加可复用函数：

```go
func ParseConstraintColumns(keys []*pg_query.Node) []string {
	cols := make([]string, 0, len(keys))
	for _, key := range keys {
		if s := key.GetString_(); s != nil {
			cols = append(cols, s.Sval)
		}
	}
	return cols
}

func DeparseNode(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	sql, err := pg_query.Deparse(&pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{{Stmt: node}},
	})
	if err == nil {
		return strings.TrimSpace(strings.TrimSuffix(sql, ";"))
	}
	return fmt.Sprintf("%v", node)
}
```

如果 `pg_query.Deparse` 不接受裸 expression 节点，则将 `DeparseNode` 保持在 Parser 内部使用，并在任务执行时用最小 SQL 包装方式调整。调整后必须保证新增测试断言中的 `Expression` 是可读 SQL，而不是 protobuf dump。

- [ ] **步骤 1.3：处理 table-level `UNIQUE` / `CHECK`**

在 `internal/parser/create_table_handler.go` 的 table constraint switch 中添加：

```go
case pg_query.ConstrType_CONSTR_UNIQUE:
	cols := parserutil.ParseConstraintColumns(constraint.Keys)
	con := model.Constraint{
		Name:    constraint.Conname,
		Type:    "unique",
		Columns: cols,
	}
	con.Name = defaultConstraintName(tableName, con)
	constraints = append(constraints, con)
case pg_query.ConstrType_CONSTR_CHECK:
	con := model.Constraint{
		Name:       constraint.Conname,
		Type:       "check",
		Expression: parserutil.DeparseNode(constraint.RawExpr),
	}
	con.Name = defaultConstraintName(tableName, con)
	constraints = append(constraints, con)
```

同步扩展 `defaultConstraintName`：

```go
if constraint.Type == "unique" && len(constraint.Columns) > 0 {
	return table + "_" + strings.Join(constraint.Columns, "_") + "_key"
}
if constraint.Type == "check" {
	return table + "_check"
}
```

- [ ] **步骤 1.4：处理 inline `UNIQUE` / `CHECK`**

在 column constraint switch 中添加：

```go
case pg_query.ConstrType_CONSTR_UNIQUE:
	con := model.Constraint{
		Name:    c.Conname,
		Type:    "unique",
		Columns: []string{col.Name},
	}
	con.Name = defaultConstraintName(tableName, con)
	constraints = append(constraints, con)
case pg_query.ConstrType_CONSTR_CHECK:
	con := model.Constraint{
		Name:       c.Conname,
		Type:       "check",
		Columns:    []string{col.Name},
		Expression: parserutil.DeparseNode(c.RawExpr),
	}
	con.Name = defaultConstraintName(tableName, con)
	constraints = append(constraints, con)
```

- [ ] **步骤 1.5：补齐 renderer 的 unique 分支**

在 `internal/render/render.go` 的 `renderConstraintDefinition` 中添加：

```go
case "unique":
	return fmt.Sprintf("UNIQUE (%s)", quoteIdentifierList(c.Columns))
```

- [ ] **步骤 1.6：编写 renderer 测试**

在 `internal/render/render_test.go` 添加：

```go
func TestRenderUniqueAndCheckConstraints(t *testing.T) {
	r := NewRenderer()
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	table.Constraints["users_email_key"] = &model.Constraint{
		Name: "users_email_key", Type: "unique", Columns: []string{"email"},
	}
	table.Constraints["users_email_check"] = &model.Constraint{
		Name: "users_email_check", Type: "check", Expression: "email <> ''",
	}

	sql := r.Render(diff.NewAddTableOp("public", "users", table))
	for _, want := range []string{
		`CONSTRAINT "users_email_key" UNIQUE ("email")`,
		`CONSTRAINT "users_email_check" CHECK (email <> '')`,
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("expected %q in:\n%s", want, sql)
		}
	}
}
```

- [ ] **步骤 1.7：验证并提交**

运行：

```bash
rtk go test ./internal/parser ./internal/render ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/parser/create_table_handler.go internal/parser/parserutil/util.go internal/parser/handler_test.go internal/render/render.go internal/render/render_test.go
rtk git commit -m "feat(parser): 支持 unique 和 check 约束解析"
```

## 任务 2：支持 `ALTER TABLE ... ADD/DROP CONSTRAINT`

**文件：**
- 修改：`internal/parser/alter_table_handler.go`
- 修改：`internal/parser/mutation.go`
- 修改：`internal/parser/handler_test.go`

- [ ] **步骤 2.1：编写失败测试**

在 `internal/parser/handler_test.go` 添加：

```go
func TestAlterTableHandler_AddAndDropConstraint(t *testing.T) {
	addNode := mustParseFirstStmt(t, `ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)`)
	h := &AlterTableHandler{}

	addMutations, err := h.Handle(addNode)
	require.NoError(t, err)
	require.Len(t, addMutations, 1)
	addMut := addMutations[0].(AddConstraintMutation)
	assert.Equal(t, "public", addMut.Schema)
	assert.Equal(t, "users", addMut.Table)
	assert.Equal(t, "users_email_key", addMut.Constraint.Name)
	assert.Equal(t, "unique", addMut.Constraint.Type)
	assert.Equal(t, []string{"email"}, addMut.Constraint.Columns)

	dropNode := mustParseFirstStmt(t, `ALTER TABLE users DROP CONSTRAINT users_email_key`)
	dropMutations, err := h.Handle(dropNode)
	require.NoError(t, err)
	require.Len(t, dropMutations, 1)
	dropMut := dropMutations[0].(DropConstraintMutation)
	assert.Equal(t, "users_email_key", dropMut.Name)
}
```

运行：

```bash
rtk go test ./internal/parser -run TestAlterTableHandler_AddAndDropConstraint -v
```

预期：编译失败或测试失败，mutation 类型尚不存在。

- [ ] **步骤 2.2：增加 mutation kind 和结构体**

在 `internal/parser/mutation.go` 的常量区添加：

```go
MutKindAddConstraint  MutationKind = "add_constraint"
MutKindDropConstraint MutationKind = "drop_constraint"
```

添加结构体：

```go
type AddConstraintMutation struct {
	Schema     string
	Table      string
	Constraint model.Constraint
}

func (m AddConstraintMutation) Kind() MutationKind { return MutKindAddConstraint }
func (m AddConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Constraint.Name, model.KindConstraint)
}
func (m AddConstraintMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table := ns.Tables[m.Table]
	if table == nil {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	c := m.Constraint
	if c.Table == "" {
		c.Table = m.Table
	}
	table.Constraints[c.Name] = &c
	return nil
}

type DropConstraintMutation struct {
	Schema string
	Table  string
	Name   string
}

func (m DropConstraintMutation) Kind() MutationKind { return MutKindDropConstraint }
func (m DropConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Name, model.KindConstraint)
}
func (m DropConstraintMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table := ns.Tables[m.Table]
	if table == nil {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	delete(table.Constraints, m.Name)
	return nil
}
```

- [ ] **步骤 2.3：处理 `AT_AddConstraint` 和 `AT_DropConstraint`**

在 `internal/parser/alter_table_handler.go` 的 switch 中添加：

```go
case pg_query.AlterTableType_AT_AddConstraint:
	if cmd.Def == nil {
		continue
	}
	c := cmd.Def.GetConstraint()
	if c == nil {
		continue
	}
	constraint, ok := parseTableConstraint(tableName, c)
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: unsupported ADD CONSTRAINT type: %v\n", c.Contype)
		continue
	}
	mutations = append(mutations, AddConstraintMutation{
		Schema: schemaName, Table: tableName, Constraint: constraint,
	})
case pg_query.AlterTableType_AT_DropConstraint:
	if cmd.Name == "" {
		fmt.Fprintf(os.Stderr, "warning: DROP CONSTRAINT missing constraint name\n")
		continue
	}
	mutations = append(mutations, DropConstraintMutation{
		Schema: schemaName, Table: tableName, Name: cmd.Name,
	})
```

`parseTableConstraint` 应放在 parser 包内，供 `CreateTableHandler` 与 `AlterTableHandler` 复用。任务 1 中已有解析逻辑，执行本任务时将其抽成函数，避免重复。

- [ ] **步骤 2.4：验证并提交**

运行：

```bash
rtk go test ./internal/parser ./internal/diff ./internal/render
```

预期：全部通过。

提交：

```bash
rtk git add internal/parser/alter_table_handler.go internal/parser/mutation.go internal/parser/handler_test.go internal/parser/create_table_handler.go
rtk git commit -m "feat(parser): 支持 alter table 约束变更"
```

## 任务 3：补齐 `character(n)` / `char(n)` 规范化

**文件：**
- 修改：`internal/normalize/normalize.go`
- 修改：`internal/normalize/normalize_test.go`

- [ ] **步骤 3.1：编写失败测试**

在 `internal/normalize/normalize_test.go` 的 `TestNormalizeDataType_VarcharWithLength` 或新测试中添加：

```go
func TestNormalizeDataType_CharacterWithLength(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"character(10)", "char(10)"},
		{"CHARACTER(10)", "char(10)"},
		{"char(10)", "char(10)"},
		{"character", "char"},
		{"CHARACTER", "char"},
	}
	for _, tt := range tests {
		got := normalizeDataType(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDataType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

运行：

```bash
rtk go test ./internal/normalize -run TestNormalizeDataType_CharacterWithLength -v
```

预期：失败，`character(10)` 未规范化为 `char(10)`。

- [ ] **步骤 3.2：实现 regex 和 alias**

在 `internal/normalize/normalize.go` 中添加：

```go
"character": "char",
```

在 regex 区域添加：

```go
var characterWithLenRe = regexp.MustCompile(`(?i)^character\((\d+)\)$`)
```

在 `normalizeDataType` 中 `charVaryingWithLenRe` 判断之后添加：

```go
if m := characterWithLenRe.FindStringSubmatch(lower); m != nil {
	return "char(" + m[1] + ")"
}
```

- [ ] **步骤 3.3：验证并提交**

运行：

```bash
rtk go test ./internal/normalize ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/normalize/normalize.go internal/normalize/normalize_test.go
rtk git commit -m "fix(normalize): 规范化 character 类型别名"
```

## 任务 4：修正 identity 变更语义

**文件：**
- 修改：`internal/diff/operation.go`
- 修改：`internal/diff/diff_columns.go`
- 修改：`internal/render/render.go`
- 修改：`internal/diff/differ_test.go`
- 修改：`internal/render/render_test.go`
- 修改：`internal/plan/plan.go`

- [ ] **步骤 4.1：编写失败测试**

在 `internal/diff/differ_test.go` 添加：

```go
func TestDiffer_IdentityAddUsesAddIdentityOp(t *testing.T) {
	source := model.NewSchema()
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	source.GetOrCreateNamespace("public").Tables["users"] = sourceTable

	target := model.NewSchema()
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{
		Name: "id", DataType: "integer", IsNullable: false,
		IsIdentity: true, IdentityKind: "ALWAYS",
	})
	target.GetOrCreateNamespace("public").Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	require.Empty(t, warnings)
	require.Len(t, ops, 1)
	_, ok := ops[0].(*AddIdentityOp)
	require.True(t, ok, "non-identity to identity must use AddIdentityOp")
}
```

在 `internal/render/render_test.go` 添加：

```go
func TestRenderAddIdentity(t *testing.T) {
	op := diff.NewAddIdentityOp("public", "users", "id", "ALWAYS")
	sql := NewRenderer().Render(op)
	want := `ALTER TABLE "public"."users" ALTER COLUMN "id" ADD GENERATED ALWAYS AS IDENTITY;`
	if !strings.Contains(sql, want) {
		t.Fatalf("expected %q in:\n%s", want, sql)
	}
}
```

运行：

```bash
rtk go test ./internal/diff ./internal/render -run "TestDiffer_IdentityAddUsesAddIdentityOp|TestRenderAddIdentity" -v
```

预期：编译失败，`AddIdentityOp` 尚不存在。

- [ ] **步骤 4.2：新增 `AddIdentityOp`**

在 `internal/diff/operation.go` 添加 kind：

```go
KindAddIdentity Kind = "add_identity"
```

添加结构体：

```go
type AddIdentityOp struct {
	baseOperation
	Schema       string
	Table        string
	Column       string
	IdentityKind string
}

func NewAddIdentityOp(schema, table, column, identityKind string) *AddIdentityOp {
	return &AddIdentityOp{
		baseOperation: baseOperation{
			kind:      KindAddIdentity,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:       schema,
		Table:        table,
		Column:       column,
		IdentityKind: identityKind,
	}
}

func (op *AddIdentityOp) IsDestructive() bool { return false }
func (op *AddIdentityOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{model.NewObjectKey(op.Schema, op.Table, model.KindTable)}
}
```

- [ ] **步骤 4.3：调整 diff 逻辑**

在 `internal/diff/diff_columns.go` 中将 identity 比较替换为：

```go
if source.IsIdentity != target.IsIdentity || source.IdentityKind != target.IdentityKind {
	switch {
	case !source.IsIdentity && target.IsIdentity:
		c.addOp(NewAddIdentityOp(schema, table, source.Name, target.IdentityKind))
	case source.IsIdentity && target.IsIdentity:
		c.addOp(NewSetIdentityOp(schema, table, source.Name, target.IdentityKind))
	case source.IsIdentity && !target.IsIdentity:
		c.addOp(NewDropIdentityOp(schema, table, source.Name))
	}
}
```

- [ ] **步骤 4.4：调整 renderer**

在 `internal/render/render.go` 的 `Render` switch 添加：

```go
case *diff.AddIdentityOp:
	return fmt.Sprintf("-- op: add_identity risk:low\nALTER TABLE %s ALTER COLUMN %s ADD GENERATED %s AS IDENTITY;",
		quoteQualifiedIdentifier(v.Schema, v.Table),
		quoteIdentifier(v.Column),
		v.IdentityKind,
	)
```

保留 `SetIdentityOp` 为：

```go
ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS
```

不要给 `SET GENERATED` 添加 `AS IDENTITY`，PostgreSQL 的语法是 `SET GENERATED { ALWAYS | BY DEFAULT }`。

- [ ] **步骤 4.5：调整执行阶段**

在 `internal/plan/plan.go` 的 deploy 分支加入 `KindAddIdentity`：

```go
case diff.KindAlterColumnType, diff.KindSetNotNull, diff.KindDropNotNull,
	diff.KindAddEnumLabel, diff.KindSetDefault, diff.KindDropDefault,
	diff.KindRenameColumn, diff.KindAddIdentity, diff.KindSetIdentity,
	diff.KindAlterColumnCollation:
	return StageDeploy
```

将 `KindDropIdentity` 放入 post-deploy 删除分支。

- [ ] **步骤 4.6：验证并提交**

运行：

```bash
rtk go test ./internal/diff ./internal/render ./internal/plan
```

预期：全部通过。

提交：

```bash
rtk git add internal/diff/operation.go internal/diff/diff_columns.go internal/diff/differ_test.go internal/render/render.go internal/render/render_test.go internal/plan/plan.go
rtk git commit -m "fix(render): 区分 identity 添加和生成策略修改"
```

## 任务 5：支持 `ALTER COLUMN TYPE ... USING`

**文件：**
- 修改：`internal/parser/alter_table_handler.go`
- 修改：`internal/parser/mutation.go`
- 修改：`internal/diff/operation.go`
- 修改：`internal/render/render.go`
- 修改：`internal/parser/handler_test.go`
- 修改：`internal/render/render_test.go`

- [ ] **步骤 5.1：编写失败测试**

在 `internal/parser/handler_test.go` 添加：

```go
func TestAlterTableHandler_AlterColumnTypeUsing(t *testing.T) {
	node := mustParseFirstStmt(t, `ALTER TABLE users ALTER COLUMN id TYPE bigint USING id::bigint`)
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut := mutations[0].(AlterColumnTypeMutation)
	assert.Equal(t, "id", mut.Column)
	assert.Equal(t, "bigint", mut.ToType)
	assert.Equal(t, "id::bigint", mut.UsingExpr)
}
```

在 `internal/render/render_test.go` 添加：

```go
func TestRenderAlterColumnTypeUsing(t *testing.T) {
	op := diff.NewAlterColumnTypeOp("public", "users", "id", "integer", "bigint")
	op.UsingExpr = "id::bigint"
	sql := NewRenderer().Render(op)
	want := `ALTER TABLE "public"."users" ALTER COLUMN "id" TYPE bigint USING id::bigint;`
	if !strings.Contains(sql, want) {
		t.Fatalf("expected %q in:\n%s", want, sql)
	}
}
```

运行：

```bash
rtk go test ./internal/parser ./internal/render -run "Using" -v
```

预期：编译失败，字段尚不存在。

- [ ] **步骤 5.2：扩展 mutation 和 operation**

在 `AlterColumnTypeMutation` 添加：

```go
UsingExpr string
```

在 `AlterColumnTypeOp` 添加：

```go
UsingExpr string
```

保持构造函数签名不变，调用方可按需赋值。

- [ ] **步骤 5.3：从 pg_query AST 提取 `USING`**

在 `internal/parser/alter_table_handler.go` 的 `AT_AlterColumnType` 分支设置：

```go
usingExpr := ""
if cmd.Behavior != pg_query.DropBehavior_DROP_RESTRICT {
	_ = cmd.Behavior
}
if cmd.Transform != nil {
	usingExpr = parserutil.DeparseNode(cmd.Transform)
}
mutations = append(mutations, AlterColumnTypeMutation{
	Schema: schemaName,
	Table: tableName,
	Column: colName,
	ToType: col.DataType,
	UsingExpr: usingExpr,
})
```

执行时若 pg_query_go 字段名不是 `Transform`，先用小型探针测试确认字段名，再将字段名替换为实际 AST 字段。完成后保留测试，不保留探针代码。

- [ ] **步骤 5.4：让 mutation apply 保留 `UsingExpr`**

`Apply` 只更新 schema 的目标类型，不需要将 `UsingExpr` 落到 `model.Column`。`UsingExpr` 是迁移 SQL 的执行细节，不是 schema 终态。这里不新增模型字段。

- [ ] **步骤 5.5：renderer 输出 `USING`**

在 `renderAlterColumnType` 中添加：

```go
if op.UsingExpr != "" {
	sql += " USING " + op.UsingExpr
}
```

- [ ] **步骤 5.6：验证并提交**

运行：

```bash
rtk go test ./internal/parser ./internal/render ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/parser/alter_table_handler.go internal/parser/mutation.go internal/parser/handler_test.go internal/diff/operation.go internal/render/render.go internal/render/render_test.go
rtk git commit -m "feat(parser): 保留 alter column type using 表达式"
```

## 任务 6：补齐高级索引渲染

**文件：**
- 修改：`internal/parser/index_handler.go`
- 修改：`internal/render/render.go`
- 修改：`internal/parser/index_handler_test.go`
- 修改：`internal/render/render_test.go`

- [ ] **步骤 6.1：编写 parser 失败测试**

在 `internal/parser/index_handler_test.go` 添加：

```go
func TestCreateIndexHandler_AdvancedElements(t *testing.T) {
	p := NewParser()
	sql := `CREATE TABLE users (email text);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_pattern
ON users USING btree (email text_pattern_ops DESC NULLS LAST)
WHERE email IS NOT NULL;`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	idx := schema.Schemas["public"].Tables["users"].Indexes["idx_users_email_pattern"]
	if idx == nil {
		t.Fatal("expected index")
	}
	if !idx.Concurrent {
		t.Fatal("expected Concurrent=true")
	}
	if !idx.IfNotExists {
		t.Fatal("expected IfNotExists=true")
	}
	if idx.Method != "btree" {
		t.Fatalf("expected btree, got %q", idx.Method)
	}
	if idx.WhereClause != "email IS NOT NULL" {
		t.Fatalf("unexpected where clause: %q", idx.WhereClause)
	}
	elem := idx.Elements[0]
	if elem.Opclass != "text_pattern_ops" || elem.Ordering != "DESC" || elem.NullsOrdering != "LAST" {
		t.Fatalf("unexpected element: %#v", elem)
	}
}
```

运行：

```bash
rtk go test ./internal/parser -run TestCreateIndexHandler_AdvancedElements -v
```

预期：失败，opclass 和可读 where clause 尚未完整提取。

- [ ] **步骤 6.2：提取 opclass 和 collation**

在 `parseIndexElem` 中添加：

```go
if len(elem.Opclass) > 0 {
	parts := make([]string, 0, len(elem.Opclass))
	for _, item := range elem.Opclass {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	result.Opclass = strings.Join(parts, ".")
}
if len(elem.Collation) > 0 {
	parts := make([]string, 0, len(elem.Collation))
	for _, item := range elem.Collation {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	result.Collation = strings.Join(parts, ".")
}
```

- [ ] **步骤 6.3：替换 `safeDeparse`**

将 `safeDeparse` 改为调用 `parserutil.DeparseNode`：

```go
func safeDeparse(node *pg_query.Node) string {
	return parserutil.DeparseNode(node)
}
```

如果 expression 节点不能直接 deparse，使用任务 1 中验证后的表达式反解析方案，确保输出是 `lower(email)`、`email IS NOT NULL` 这类可读 SQL。

- [ ] **步骤 6.4：编写 renderer 测试**

在 `internal/render/render_test.go` 添加：

```go
func TestRenderCreateIndexAdvanced(t *testing.T) {
	idx := &model.Index{
		Name: "idx_users_email_pattern",
		Table: "users",
		Method: "btree",
		Concurrent: true,
		IfNotExists: true,
		WhereClause: "email IS NOT NULL",
		Elements: []model.IndexElem{{
			Name: "email",
			Opclass: "text_pattern_ops",
			Ordering: "DESC",
			NullsOrdering: "LAST",
		}},
	}
	sql := NewRenderer().Render(diff.NewCreateIndexOp("public", idx))
	want := `CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_users_email_pattern" ON "public"."users" USING btree ("email" text_pattern_ops DESC NULLS LAST) WHERE email IS NOT NULL;`
	if !strings.Contains(sql, want) {
		t.Fatalf("expected %q in:\n%s", want, sql)
	}
}
```

- [ ] **步骤 6.5：实现 renderer**

重写 `renderCreateIndex` 中 index item 构建逻辑：

```go
func renderIndexElem(elem model.IndexElem) string {
	item := ""
	if elem.Name != "" {
		item = quoteIdentifier(elem.Name)
	} else if elem.Expr != "" {
		item = "(" + elem.Expr + ")"
	}
	if elem.Collation != "" {
		item += " COLLATE " + quoteIdentifier(elem.Collation)
	}
	if elem.Opclass != "" {
		item += " " + elem.Opclass
	}
	if elem.Ordering == "ASC" || elem.Ordering == "DESC" {
		item += " " + elem.Ordering
	}
	if elem.NullsOrdering == "FIRST" || elem.NullsOrdering == "LAST" {
		item += " NULLS " + elem.NullsOrdering
	}
	return item
}
```

在 `renderCreateIndex` 中组装：

```go
concurrently := ""
if idx.Concurrent {
	concurrently = "CONCURRENTLY "
}
ifNotExists := ""
if idx.IfNotExists {
	ifNotExists = "IF NOT EXISTS "
}
method := ""
if idx.Method != "" {
	method = " USING " + idx.Method
}
sql := fmt.Sprintf("CREATE %sINDEX %s%s%s ON %s%s (%s)",
	unique, concurrently, ifNotExists, quoteIdentifier(idx.Name),
	quoteQualifiedIdentifier(op.Schema, idx.Table), method, items)
if idx.WhereClause != "" {
	sql += " WHERE " + idx.WhereClause
}
return fmt.Sprintf("-- op: add_index risk:low\n%s;", sql)
```

- [ ] **步骤 6.6：验证并提交**

运行：

```bash
rtk go test ./internal/parser ./internal/render ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/parser/index_handler.go internal/parser/index_handler_test.go internal/render/render.go internal/render/render_test.go
rtk git commit -m "feat(render): 补齐高级索引 SQL 渲染"
```

## 任务 7：增强高级索引 introspect

**文件：**
- 修改：`internal/introspect/indexes.go`
- 修改：`internal/model/table.go`
- 修改：`internal/normalize/normalize.go`
- 修改：`internal/introspect/indexes_test.go`（如不存在则创建）

- [ ] **步骤 7.1：增加 raw definition 字段**

在 `model.Index` 中添加：

```go
Definition string // pg_get_indexdef output for advanced index comparison
Predicate  string // partial index predicate, empty = no predicate
```

保留现有 `WhereClause`，执行时可将 `Predicate` 与 `WhereClause` 合并为同一字段。推荐直接复用 `WhereClause`，避免双字段分叉；如果采用复用方案，则只新增 `Definition string`。

- [ ] **步骤 7.2：修改 introspect SQL**

将 `internal/introspect/indexes.go` 查询扩展为：

```sql
SELECT
	idx.relname AS index_name,
	t.relname AS table_name,
	array_agg(a.attname ORDER BY k.ordinality) FILTER (WHERE a.attname IS NOT NULL) AS column_names,
	i.indisunique AS is_unique,
	am.amname AS method,
	pg_get_indexdef(i.indexrelid) AS definition,
	COALESCE(pg_get_expr(i.indpred, i.indrelid), '') AS predicate
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_class t ON t.oid = i.indrelid
JOIN pg_namespace n ON n.oid = idx.relnamespace
JOIN pg_am am ON am.oid = idx.relam
LEFT JOIN LATERAL unnest(i.indkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON true
LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE n.nspname = $1 AND i.indisprimary = false
GROUP BY idx.oid, idx.relname, t.relname, i.indisunique, am.amname, i.indexrelid, i.indpred, i.indrelid
```

- [ ] **步骤 7.3：scan 新字段并落模型**

在 scan 变量中加入：

```go
definition string
predicate string
```

构造 index：

```go
index := &model.Index{
	Name: indexName,
	Table: tableName,
	Columns: columnNames,
	Unique: isUnique,
	Method: method,
	Definition: definition,
	WhereClause: predicate,
}
```

- [ ] **步骤 7.4：规范化 DB introspect 索引字段**

在 `internal/normalize/normalize.go` 的 `canonicalizeIndexInPlace` 中保留现有 `Columns -> Elements` 兼容逻辑，并规范化 `Definition`：

```go
idx.Definition = strings.Join(strings.Fields(idx.Definition), " ")
idx.WhereClause = strings.Join(strings.Fields(idx.WhereClause), " ")
```

- [ ] **步骤 7.5：编写 introspect 查询形状测试**

如果当前 introspect 测试不连接数据库，则创建针对 normalize 的模型测试，确保 DB 风格 index 规范化后不会丢失 `WhereClause` / `Definition`：

```go
func TestCanonicalizeIndex_PreservesDefinitionAndPredicate(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	tbl := model.NewTable("public", "users")
	ns.Tables["users"] = tbl
	tbl.Indexes["idx_users_email"] = &model.Index{
		Name: "idx_users_email",
		Table: "users",
		Columns: []string{"email"},
		Method: "btree",
		Definition: "CREATE INDEX idx_users_email ON public.users USING btree (email)",
		WhereClause: "email IS NOT NULL",
	}

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	idx := tbl.Indexes["idx_users_email"]
	if idx.WhereClause != "email IS NOT NULL" {
		t.Fatalf("unexpected predicate: %q", idx.WhereClause)
	}
	if idx.Definition == "" {
		t.Fatal("expected definition to be preserved")
	}
}
```

- [ ] **步骤 7.6：验证并提交**

运行：

```bash
rtk go test ./internal/introspect ./internal/normalize ./internal/diff
```

预期：全部通过。若本机没有数据库依赖，`internal/introspect` 现有测试仍应能通过；不要新增强依赖外部数据库的默认测试。

提交：

```bash
rtk git add internal/introspect/indexes.go internal/model/table.go internal/normalize/normalize.go internal/normalize/normalize_test.go
rtk git commit -m "feat(introspect): 增强索引定义自省"
```

## 任务 8：支持受控 `DROP SCHEMA` 与执行阶段归类

**文件：**
- 修改：`internal/diff/differ.go`
- 修改：`internal/plan/plan.go`
- 修改：`internal/app/diff_service_test.go`
- 修改：`internal/diff/differ_test.go`
- 修改：`internal/plan/plan_test.go`

- [ ] **步骤 8.1：编写 diff 失败测试**

在 `internal/diff/differ_test.go` 添加：

```go
func TestDiffer_DropSchema(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("old_schema")
	target := model.NewSchema()
	target.GetOrCreateNamespace("public")

	ops, warnings := NewDiffer().Diff(source, target)
	require.Empty(t, warnings)

	found := false
	for _, op := range ops {
		if drop, ok := op.(*DropSchemaOp); ok {
			found = true
			assert.Equal(t, "old_schema", drop.Schema)
		}
	}
	assert.True(t, found, "expected DropSchemaOp")
}
```

运行：

```bash
rtk go test ./internal/diff -run TestDiffer_DropSchema -v
```

预期：失败，当前只输出 warning。

- [ ] **步骤 8.2：生成 `DropSchemaOp`**

在 `internal/diff/differ.go` 中将 source-only namespace 逻辑改为：

```go
for _, name := range sourceNames {
	if _, exists := target.Schemas[name]; !exists {
		c.addOp(NewDropSchemaOp(name))
	}
}
```

- [ ] **步骤 8.3：调整计划阶段**

在 `internal/plan/plan.go` 的 `assignStage` 中：

```go
case diff.KindCreateSchema, diff.KindAddTable, diff.KindAddColumn,
	diff.KindAddIndex, diff.KindAddConstraint, diff.KindAddEnumType:
	return StagePreDeploy
```

删除阶段中加入：

```go
case diff.KindDropSchema, diff.KindDropTable, diff.KindDropColumn,
	diff.KindDropIndex, diff.KindDropConstraint, diff.KindDropEnumType,
	diff.KindDropIdentity:
	if p.unsafeDrops {
		return StagePostDeploy
	}
	return ""
```

- [ ] **步骤 8.4：编写 plan 测试**

在 `internal/plan/plan_test.go` 添加：

```go
func TestPlanner_SchemaStages(t *testing.T) {
	planner := NewPlanner(true)
	stages := planner.Plan([]diff.Operation{
		diff.NewCreateSchemaOp("auth"),
		diff.NewDropSchemaOp("old_schema"),
	})

	if len(stages[StagePreDeploy]) != 1 || stages[StagePreDeploy][0].Kind() != diff.KindCreateSchema {
		t.Fatalf("expected create_schema in pre-deploy, got %#v", stages[StagePreDeploy])
	}
	if len(stages[StagePostDeploy]) != 1 || stages[StagePostDeploy][0].Kind() != diff.KindDropSchema {
		t.Fatalf("expected drop_schema in post-deploy, got %#v", stages[StagePostDeploy])
	}
}
```

- [ ] **步骤 8.5：确认 destructive 过滤**

`DropSchemaOp.IsDestructive()` 已返回 true。为 `internal/app/diff_service_test.go` 添加或扩展测试，确认 `UnsafeDrop=false` 时 drop schema 不出现在 rendered ops 中，warnings 中包含 destructive 提示。

测试骨架：

```go
func TestFilterDestructiveOps_DropSchema(t *testing.T) {
	ops := []diff.Operation{diff.NewDropSchemaOp("old_schema")}
	filtered, warnings := FilterDestructiveOps(ops, false)
	require.Empty(t, filtered)
	require.NotEmpty(t, warnings)

	filtered, warnings = FilterDestructiveOps(ops, true)
	require.Len(t, filtered, 1)
	require.Empty(t, warnings)
}
```

- [ ] **步骤 8.6：验证并提交**

运行：

```bash
rtk go test ./internal/diff ./internal/plan ./internal/app ./internal/render
```

预期：全部通过。

提交：

```bash
rtk git add internal/diff/differ.go internal/diff/differ_test.go internal/plan/plan.go internal/plan/plan_test.go internal/app/diff_service_test.go
rtk git commit -m "feat(diff): 支持受控 drop schema"
```

## 任务 9：最终文档同步与全量验证

**文件：**
- 修改：`docs/DDL.md`
- 修改：`docs/plan/feat_ext.md`

- [ ] **步骤 9.1：根据实际完成项更新状态**

完成任务 1-8 后，将 `docs/DDL.md` 对应行调整为：

```markdown
| `ALTER TABLE ... ALTER COLUMN TYPE` | ✅ (含 `USING`) | ✅ | ✅ | ✅ | ✅ |
| `UNIQUE` 约束 (表级) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `UNIQUE` 约束 (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CHECK` 约束 (表级) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CHECK` 约束 (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... ADD CONSTRAINT` | ✅ | ✅ | ✅ | — | ✅ |
| `DROP SCHEMA` | — | ✅ (受 `--unsafe-drop` 控制) | ✅ | — | ✅ |
```

高级索引 introspect 若只完成 `pg_get_indexdef()` 保存，保持 `⚠️` 并说明「raw definition 已保留，结构化比较仍有限」。

- [ ] **步骤 9.2：更新 `docs/plan/feat_ext.md`**

将「需要修正文档或补齐实现的特性」表格改为完成记录：

```markdown
| 特性 | 当前情况 | 结果 |
| --- | --- | --- |
| `ALTER COLUMN TYPE ... USING` | 已补齐 | Parser 保留 `UsingExpr`，Renderer 输出 `USING`。 |
| `UNIQUE` / `CHECK` SQL 文件解析 | 已补齐 | 支持 table-level 与 inline 约束。 |
```

只记录实际完成项。未完成项保留原表述，不虚报。

- [ ] **步骤 9.3：全量验证**

运行：

```bash
rtk go test ./...
rtk git diff --check
```

预期：
- `rtk go test ./...` 退出码为 0。
- `rtk git diff --check` 退出码为 0。

- [ ] **步骤 9.4：最终提交**

```bash
rtk git add docs/DDL.md docs/plan/feat_ext.md
rtk git commit -m "docs(DDL): 同步补齐后的特性支持状态"
```

## 执行建议

推荐拆成 4 个 PR 或 4 个连续检查点：

1. **文档与低风险修复：** 任务 0、任务 3。
2. **约束解析：** 任务 1、任务 2。
3. **列变更语义：** 任务 4、任务 5。
4. **索引与 schema：** 任务 6、任务 7、任务 8、任务 9。

每个检查点都必须运行对应任务中的测试命令。进入下一检查点前，工作区应保持干净。

## 自检清单

- [x] **规格覆盖度：** 覆盖 `docs/plan/feat_ext.md` 中「需要修正文档或补齐实现的特性」全部 8 项。
- [x] **任务独立性：** 每个任务都有明确文件、失败测试、实现步骤、验证命令和提交命令。
- [x] **类型一致性：** 约束使用 `model.Constraint`，索引使用 `model.Index` / `model.IndexElem`，identity 使用独立 operation。
- [x] **范围控制：** 高级索引 introspect 与 renderer 分开处理，避免单提交跨越过大。
- [x] **验证闭环：** 每个任务都有局部测试，最终任务有全量 `rtk go test ./...` 与 `rtk git diff --check`。

计划已完成并保存到 `docs/superpowers/plans/2026-05-21-ddl-feature-fixes.md`。两种执行方式：

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代。

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点。
