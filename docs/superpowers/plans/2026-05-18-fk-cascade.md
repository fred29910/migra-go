# ON DELETE / ON UPDATE 级联 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。

**目标：** 支持外键约束的 `ON DELETE` / `ON UPDATE` 级联选项：模型存储、解析器提取、内省查询、渲染输出、diff 对比。

**技术栈：** Go 1.26, pg_query_go v6, pgx v5

**涉及文件：**
- 修改：`internal/model/table.go` — Constraint 新增 `OnDelete`/`OnUpdate` 字段
- 修改：`internal/parser/create_table_handler.go` — 提取 FkDelAction/FkUpdAction
- 修改：`internal/introspect/constraints.go` — 查询 confupdtype/confdeltype
- 修改：`internal/render/render.go` — `renderConstraintDefinition` 输出级联子句
- 修改：`internal/diff/diff_tables.go` — `sameConstraintContent` 比较级联字段
- 修改：`internal/normalize/normalize.go` — 归一化级联关键字
- 测试：多个文件

---

### 任务 1：Constraint 模型添加级联字段

**文件：**
- 修改: `internal/model/table.go:37-47`

- [ ] **步骤 1.1：新增字段**

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
	OnDelete   string // e.g. "CASCADE", "SET NULL", "NO ACTION", ""
	OnUpdate   string // e.g. "CASCADE", "SET NULL", "NO ACTION", ""
}
```

- [ ] **步骤 1.2：提交**

```bash
git add internal/model/table.go
git commit -m "feat(model): add OnDelete and OnUpdate fields to Constraint"
```

---

### 任务 2：Parser 提取级联操作

**文件：**
- 修改: `internal/parser/create_table_handler.go:81-112` — FK constraint 构建

- [ ] **步骤 2.1：编写失败的测试**

在 `internal/parser/handler_test.go` 新增：

```go
func TestCreateTableHandler_ForeignKeyCascade(t *testing.T) {
	sql := `CREATE TABLE orders (
		id integer PRIMARY KEY,
		user_id integer REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL
	)`
	node := mustParseFirstStmt(t, sql)
	handler := &CreateTableHandler{}
	mutations, err := handler.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	ctMut := mutations[0].(CreateTableMutation)
	require.Len(t, ctMut.Constraints, 2) // pkey + fkey

	var fkConstraint *model.Constraint
	for _, c := range ctMut.Constraints {
		if c.Type == "foreign_key" {
			fkConstraint = &c
			break
		}
	}
	require.NotNil(t, fkConstraint)
	assert.Equal(t, "CASCADE", fkConstraint.OnDelete)
	assert.Equal(t, "SET NULL", fkConstraint.OnUpdate)
}
```

- [ ] **步骤 2.2：运行测试验证失败**

```bash
go test ./internal/parser/ -run TestCreateTableHandler_ForeignKeyCascade -v
# 预期: FAIL - OnDelete/OnUpdate 为空
```

- [ ] **步骤 2.3：实现级联提取**

在 `internal/parser/create_table_handler.go` 的 FK constraint 构建中，`constraint.FkDelAction` / `constraint.FkUpdAction` 是 pg_query 的整型枚举，需要转换。

在构建 `model.Constraint` 时，在 `con` 赋值之前添加转换函数：

```go
// fkActionMap maps pg_query FK action enums to SQL keywords
func fkActionMap(action int) string {
	switch action {
	case 1: // FKCONSTRACTION_NOACTION
		return "NO ACTION"
	case 2: // FKCONSTRACTION_RESTRICT
		return "RESTRICT"
	case 3: // FKCONSTRACTION_CASCADE
		return "CASCADE"
	case 4: // FKCONSTRACTION_SETNULL
		return "SET NULL"
	case 5: // FKCONSTRACTION_SETDEFAULT
		return "SET DEFAULT"
	default:
		return ""
	}
}
```

然后在 `con := model.Constraint{...}` 初始化中新增：

```go
con := model.Constraint{
	Name:       "",
	Type:       "foreign_key",
	Columns:    fkCols,
	RefSchema:  refSchema,
	RefTable:   refTable,
	RefColumns: refCols,
	OnDelete:   fkActionMap(int(constraint.FkDelAction)),
	OnUpdate:   fkActionMap(int(constraint.FkUpdAction)),
}
```

- [ ] **步骤 2.4：运行测试验证通过**

```bash
go test ./internal/parser/ -run TestCreateTableHandler_ForeignKeyCascade -v
# 预期: PASS
```

- [ ] **步骤 2.5：提交**

```bash
git add internal/parser/create_table_handler.go internal/parser/handler_test.go
git commit -m "feat(parser): extract ON DELETE/ON UPDATE actions from FK constraints"
```

---

### 任务 3：内省层查询级联操作

**文件：**
- 修改: `internal/introspect/constraints.go` — 外键 SQL 查询

- [ ] **步骤 3.1：扩展 SQL 查询**

在 `loadForeignKeys` 函数的 SQL 中新增 `c.confupdtype, c.confdeltype`：

```go
query := `
SELECT
	c.conname,
	t.relname AS table_name,
	array_agg(a.attname ORDER BY k.ordinality) AS column_names,
	pg_get_constraintdef(c.oid) AS definition,
	rt.relname AS ref_table,
	rn.nspname AS ref_schema,
	array_agg(ra.attname ORDER BY rk.ordinality) AS ref_column_names,
	c.confupdtype,
	c.confdeltype
FROM pg_constraint c
...`
```

- [ ] **步骤 3.2：扩展 Scan 变量和映射**

在 scan 变量中新增：

```go
var (
	// existing vars...
	confUpdType string
	confDelType string
)
```

扩展 Scan 调用（在 `refColumnNames` 之后）：

```go
err := rows.Scan(&conName, &tableName, &columnNames, &definition, &refTable, &refSchema, &refColumnNames,
	&confUpdType, &confDelType)
```

添加 pg_constraint 单字符码到 SQL 关键字的映射函数，在 `constraint` 构建之前调用：

```go
// pgConstraintAction maps pg_constraint single-char codes to SQL keywords
func pgConstraintAction(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return ""
	}
}
```

在 `constraint := &model.Constraint{...}` 初始化中新增：

```go
constraint := &model.Constraint{
	Name:       conName,
	Type:       "foreign_key",
	Table:      tableName,
	Definition: definition,
	Columns:    columnNames,
	RefSchema:  refSchema,
	RefTable:   refTable,
	RefColumns: refColumnNames,
	OnDelete:   pgConstraintAction(confDelType),
	OnUpdate:   pgConstraintAction(confUpdType),
}
```

- [ ] **步骤 3.3：运行测试**

```bash
go build ./...
# 预期: 编译通过
```

- [ ] **步骤 3.4：提交**

```bash
git add internal/introspect/constraints.go
git commit -m "feat(introspect): query confupdtype/confdeltype for FK cascade actions"
```

---

### 任务 4：渲染 ON DELETE/ON UPDATE

**文件：**
- 修改: `internal/render/render.go` — `renderConstraintDefinition`

- [ ] **步骤 4.1：编写渲染测试**

在 `internal/render/render_test.go` 新增：

```go
func TestRenderForeignKeyCascade(t *testing.T) {
	r := NewRenderer()

	t.Run("CASCADE ON DELETE", func(t *testing.T) {
		c := &model.Constraint{
			Name:    "fk_user_id",
			Type:    "foreign_key",
			Columns: []string{"user_id"},
			RefSchema: "public",
			RefTable:  "users",
			RefColumns: []string{"id"},
			OnDelete:  "CASCADE",
		}
		sql := renderConstraintDefinition(c)
		assert.Contains(t, sql, `ON DELETE CASCADE`)
	})

	t.Run("SET NULL ON UPDATE", func(t *testing.T) {
		c := &model.Constraint{
			Name:    "fk_user_id",
			Type:    "foreign_key",
			Columns: []string{"user_id"},
			RefSchema: "public",
			RefTable:  "users",
			RefColumns: []string{"id"},
			OnUpdate:  "SET NULL",
		}
		sql := renderConstraintDefinition(c)
		assert.Contains(t, sql, `ON UPDATE SET NULL`)
	})

	t.Run("Both CASCADE and SET NULL", func(t *testing.T) {
		c := &model.Constraint{
			Name:    "fk_user_id",
			Type:    "foreign_key",
			Columns: []string{"user_id"},
			RefSchema: "public",
			RefTable:  "users",
			RefColumns: []string{"id"},
			OnDelete:  "CASCADE",
			OnUpdate:  "SET NULL",
		}
		sql := renderConstraintDefinition(c)
		assert.Contains(t, sql, `ON DELETE CASCADE`)
		assert.Contains(t, sql, `ON UPDATE SET NULL`)
	})
}
```

注意：`renderConstraintDefinition` 是包内函数（小写开头），测试文件需在 `package render` 中。

- [ ] **步骤 4.2：修改 renderConstraintDefinition**

在 `internal/render/render.go` 的 `renderConstraintDefinition` 中外键 case 处追加级联子句：

```go
case "foreign_key":
	sql := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
		quoteIdentifierList(c.Columns),
		quoteQualifiedIdentifier(c.RefSchema, c.RefTable),
		quoteIdentifierList(c.RefColumns),
	)
	if c.OnDelete != "" {
		sql += fmt.Sprintf(" ON DELETE %s", c.OnDelete)
	}
	if c.OnUpdate != "" {
		sql += fmt.Sprintf(" ON UPDATE %s", c.OnUpdate)
	}
	return sql
```

- [ ] **步骤 4.3：运行测试**

```bash
go test ./internal/render/ -run TestRenderForeignKeyCascade -v
# 预期: PASS
```

- [ ] **步骤 4.4：提交**

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "feat(render): render ON DELETE/ON UPDATE in foreign key constraints"
```

---

### 任务 5：Diff 引擎比较级联字段

**文件：**
- 修改: `internal/diff/diff_tables.go` — `sameConstraintContent`

- [ ] **步骤 5.1：扩展约束内容比较**

在 `internal/diff/diff_tables.go` 的 `sameConstraintContent` 函数中追加：

```go
func sameConstraintContent(a, b *model.Constraint) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Type == b.Type &&
		reflect.DeepEqual(a.Columns, b.Columns) &&
		a.RefSchema == b.RefSchema &&
		a.RefTable == b.RefTable &&
		reflect.DeepEqual(a.RefColumns, b.RefColumns) &&
		a.Expression == b.Expression &&
		a.OnDelete == b.OnDelete &&  // NEW
		a.OnUpdate == b.OnUpdate     // NEW
}
```

- [ ] **步骤 5.2：运行测试**

```bash
go test ./internal/diff/ -run TestDiff -v
# 预期: 已有测试通过
```

- [ ] **步骤 5.3：提交**

```bash
git add internal/diff/diff_tables.go
git commit -m "fix(diff): compare OnDelete/OnUpdate in constraint content equality"
```
