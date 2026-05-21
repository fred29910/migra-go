# P2 P3 DDL 扩展实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 `docs/superpowers/plans/2026-05-21-ddl-feature-fixes.md` 已覆盖的 P0/P1 基础上，补齐 P2 高级索引闭环，并为 P3 的 view、sequence、extension 建立独立模型、diff、render、parser 和 introspect 支持。

**架构：** P2 将索引元素解析抽到共享包，确保 SQL 文件解析与 DB introspect 使用同一套结构化 `model.IndexElem` 语义。P3 为每类新对象新增独立 model、mutation、diff operation 和 renderer，不把 raw SQL 语句塞入现有 table/type 模型；对象终态保存在 `model.Namespace` 的独立 map 中。

**技术栈：** Go、pg_query_go v6、pgx v5、PostgreSQL catalog、现有 `rtk` 命令包装器。

---

## 前置状态

当前代码已经完成旧计划中的大部分 P0/P1 项：

- `ALTER COLUMN TYPE ... USING` 已有 `UsingExpr`。
- `UNIQUE` / `CHECK` 和 `ALTER TABLE ... ADD/DROP CONSTRAINT` 已有解析路径。
- `character(n)` 规范化、identity 语义、`DROP SCHEMA`、基础高级索引 renderer 已落地。
- DB introspect 已保存 `pg_get_indexdef()` 到 `model.Index.Definition`，并保存 partial index predicate 到 `WhereClause`。

本计划只处理剩余缺口：

- P2：`USING btree` 明确渲染、索引 collation / `IndexColName` diff、DB introspect 结构化还原表达式索引、opclass、collation、排序、`NULLS FIRST/LAST`。
- P3：优先支持 view、sequence、extension。function、trigger、policy 只预留边界，不在本计划实现。

## 文件结构与职责

| 文件 | 职责 | 计划内改动 |
| --- | --- | --- |
| `internal/indexdef/parse.go` | 共享索引元素解析 | 新建；从单个 index element SQL 解析 `model.IndexElem` |
| `internal/indexdef/parse_test.go` | 索引元素解析测试 | 新建；覆盖 expression、collation、opclass、排序、NULLS |
| `internal/parser/index_handler.go` | `CREATE INDEX` 解析 | 改为调用 `internal/indexdef`，删除重复解析逻辑 |
| `internal/introspect/indexes.go` | 数据库索引内省 | 使用 `pg_get_indexdef(index_oid, elem_no, true)` 结构化还原 `Elements` |
| `internal/diff/diff_tables.go` | 索引 diff | 比较 `Collation`、`IndexColName`，必要时比较 normalized `Definition` |
| `internal/render/render.go` | SQL 渲染 | 对非空 `idx.Method` 总是输出 `USING ...`；新增 view/sequence/extension 渲染 |
| `internal/model/schema.go` | namespace 根模型 | 增加 `Views`、`Sequences`、`Extensions` 和 `NewNamespace` |
| `internal/model/view.go` | view 模型 | 新建；保存视图定义与 materialized 标记 |
| `internal/model/sequence.go` | sequence 模型 | 新建；保存结构化 sequence 选项 |
| `internal/model/extension.go` | extension 模型 | 新建；保存扩展名与版本 |
| `internal/parser/view_handler.go` | `CREATE VIEW` 解析 | 新建 handler 和 mutation |
| `internal/parser/sequence_handler.go` | `CREATE SEQUENCE` 解析 | 新建 handler 和 mutation |
| `internal/parser/extension_handler.go` | `CREATE EXTENSION` 解析 | 新建 handler 和 mutation |
| `internal/parser/registry.go` | AST handler 注册 | 注册 view、sequence、extension |
| `internal/diff/operation.go` | diff operation 定义 | 增加 view、sequence、extension operation |
| `internal/diff/differ.go` | namespace diff | 增加新对象 diff 入口 |
| `internal/introspect/views.go` | view introspect | 新建；读取普通视图和物化视图 |
| `internal/introspect/sequences.go` | sequence introspect | 新建；读取 `pg_sequences` |
| `internal/introspect/extensions.go` | extension introspect | 新建；读取 `pg_extension` |
| `internal/introspect/introspect.go` | DB loader 编排 | 调用新对象 loader |
| `internal/normalize/normalize.go` | schema 规范化 | 规范化 view definition、extension/version、sequence 默认值 |
| `internal/plan/plan.go` | 执行阶段 | 将新建对象归入 pre-deploy，删除对象归入 post-deploy |
| `docs/DDL.md` | 支持矩阵 | 更新 P2/P3 支持状态和限制 |

## 任务 1：抽出共享索引元素解析

**文件：**
- 创建：`internal/indexdef/parse.go`
- 创建：`internal/indexdef/parse_test.go`
- 修改：`internal/parser/index_handler.go`

- [ ] **步骤 1.1：编写失败测试**

创建 `internal/indexdef/parse_test.go`：

```go
package indexdef

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestParseElement_AdvancedColumn(t *testing.T) {
	got, err := ParseElement(`email COLLATE "C" text_pattern_ops DESC NULLS LAST`)
	if err != nil {
		t.Fatalf("ParseElement failed: %v", err)
	}
	want := model.IndexElem{
		Name:          "email",
		Collation:     "C",
		Opclass:       "text_pattern_ops",
		Ordering:      "DESC",
		NullsOrdering: "LAST",
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseElement_Expression(t *testing.T) {
	got, err := ParseElement(`(lower(email))`)
	if err != nil {
		t.Fatalf("ParseElement failed: %v", err)
	}
	if got.Expr != "lower(email)" {
		t.Fatalf("expected expression lower(email), got %#v", got)
	}
}
```

运行：

```bash
rtk go test ./internal/indexdef -run TestParseElement -v
```

预期：失败，`internal/indexdef` 包尚不存在。

- [ ] **步骤 1.2：实现共享解析包**

创建 `internal/indexdef/parse.go`：

```go
package indexdef

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func ParseElement(elemSQL string) (model.IndexElem, error) {
	sql := "CREATE INDEX __migra_idx ON __migra_table USING btree (" + elemSQL + ")"
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: %w", elemSQL, err)
	}
	if len(tree.Stmts) != 1 {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: expected one statement", elemSQL)
	}
	stmt := tree.Stmts[0].Stmt.GetIndexStmt()
	if stmt == nil || len(stmt.IndexParams) != 1 {
		return model.IndexElem{}, fmt.Errorf("parse index element %q: expected one index element", elemSQL)
	}
	return FromNode(stmt.IndexParams[0])
}

func FromNode(node *pg_query.Node) (model.IndexElem, error) {
	elem := node.GetIndexElem()
	if elem == nil {
		return model.IndexElem{}, fmt.Errorf("expected IndexElem, got %T", node)
	}
	result := model.IndexElem{}
	if elem.Name != "" {
		result.Name = elem.Name
	}
	if elem.Expr != nil {
		result.Expr = strings.Trim(parserutil.DeparseNode(elem.Expr), "()")
	}
	if elem.Indexcolname != "" {
		result.IndexColName = elem.Indexcolname
	}
	switch elem.Ordering {
	case pg_query.SortByDir_SORTBY_ASC:
		result.Ordering = "ASC"
	case pg_query.SortByDir_SORTBY_DESC:
		result.Ordering = "DESC"
	default:
		result.Ordering = "default"
	}
	switch elem.NullsOrdering {
	case pg_query.SortByNulls_SORTBY_NULLS_FIRST:
		result.NullsOrdering = "FIRST"
	case pg_query.SortByNulls_SORTBY_NULLS_LAST:
		result.NullsOrdering = "LAST"
	default:
		result.NullsOrdering = "default"
	}
	result.Opclass = joinStringNodes(elem.Opclass)
	result.Collation = joinStringNodes(elem.Collation)
	return result, nil
}

func joinStringNodes(nodes []*pg_query.Node) string {
	parts := make([]string, 0, len(nodes))
	for _, item := range nodes {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	return strings.Join(parts, ".")
}
```

- [ ] **步骤 1.3：让 parser 复用共享解析**

在 `internal/parser/index_handler.go` 中引入：

```go
import "github.com/fred29910/migra-go/internal/indexdef"
```

将 `parseIndexElem` 改为：

```go
func parseIndexElem(node *pg_query.Node) (model.IndexElem, error) {
	return indexdef.FromNode(node)
}
```

删除 `parseIndexElem` 内部重复的 opclass、collation、ordering 解析代码；如果 `strings` import 不再使用，同步删除。

- [ ] **步骤 1.4：验证并提交**

运行：

```bash
rtk go test ./internal/indexdef ./internal/parser
```

预期：全部通过。

提交：

```bash
rtk git add internal/indexdef/parse.go internal/indexdef/parse_test.go internal/parser/index_handler.go
rtk git commit -m "refactor(index): 共享索引元素解析"
```

## 任务 2：补齐索引渲染与 diff 语义

**文件：**
- 修改：`internal/render/render.go`
- 修改：`internal/render/render_test.go`
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/diff/diff_index_test.go`

- [ ] **步骤 2.1：编写 renderer 失败测试**

在 `internal/render/render_test.go` 添加：

```go
func TestRenderCreateIndex_AlwaysRendersMethod(t *testing.T) {
	idx := &model.Index{
		Name:     "idx_users_email",
		Table:    "users",
		Method:   "btree",
		Elements: []model.IndexElem{{Name: "email"}},
	}
	sql := NewRenderer().Render(diff.NewCreateIndexOp("public", idx))
	want := `ON "public"."users" USING btree ("email")`
	if !strings.Contains(sql, want) {
		t.Fatalf("expected %q in:\n%s", want, sql)
	}
}

func TestRenderCreateIndex_WithElementCollation(t *testing.T) {
	idx := &model.Index{
		Name:   "idx_users_email_c",
		Table:  "users",
		Method: "btree",
		Elements: []model.IndexElem{{
			Name:      "email",
			Collation: "C",
		}},
	}
	sql := NewRenderer().Render(diff.NewCreateIndexOp("public", idx))
	want := `"email" COLLATE "C"`
	if !strings.Contains(sql, want) {
		t.Fatalf("expected %q in:\n%s", want, sql)
	}
}
```

运行：

```bash
rtk go test ./internal/render -run "TestRenderCreateIndex_AlwaysRendersMethod|TestRenderCreateIndex_WithElementCollation" -v
```

预期：`AlwaysRendersMethod` 失败，因为当前 renderer 省略 `USING btree`。

- [ ] **步骤 2.2：编写 diff 失败测试**

在 `internal/diff/diff_index_test.go` 添加：

```go
func TestDiffIndex_CollationChange(t *testing.T) {
	source := model.NewSchema()
	sourceTable := model.NewTable("public", "users")
	sourceTable.Indexes["idx_users_email"] = &model.Index{
		Name: "idx_users_email", Table: "users", Method: "btree",
		Elements: []model.IndexElem{{Name: "email", Collation: ""}},
	}
	source.GetOrCreateNamespace("public").Tables["users"] = sourceTable

	target := model.NewSchema()
	targetTable := model.NewTable("public", "users")
	targetTable.Indexes["idx_users_email"] = &model.Index{
		Name: "idx_users_email", Table: "users", Method: "btree",
		Elements: []model.IndexElem{{Name: "email", Collation: "C"}},
	}
	target.GetOrCreateNamespace("public").Tables["users"] = targetTable

	ops, _ := NewDiffer().Diff(source, target)
	var foundDrop, foundCreate bool
	for _, op := range ops {
		if _, ok := op.(*DropIndexOp); ok {
			foundDrop = true
		}
		if _, ok := op.(*CreateIndexOp); ok {
			foundCreate = true
		}
	}
	if !foundDrop || !foundCreate {
		t.Fatalf("expected drop and create for collation change, got %#v", ops)
	}
}
```

运行：

```bash
rtk go test ./internal/diff -run TestDiffIndex_CollationChange -v
```

预期：失败，当前 `sameIndexContent` 未比较 `Collation`。

- [ ] **步骤 2.3：修正 renderer 方法输出**

在 `renderCreateIndex` 中将 method 判断改为：

```go
method := ""
if idx.Method != "" {
	method = " USING " + idx.Method
}
```

保留 `Method == ""` 的兼容路径，避免旧模型生成空 `USING`。

- [ ] **步骤 2.4：补齐索引内容比较**

在 `sameIndexContent` 的元素比较中加入：

```go
if a.Elements[i].IndexColName != b.Elements[i].IndexColName {
	return false
}
if a.Elements[i].Collation != b.Elements[i].Collation {
	return false
}
```

不要比较 `Concurrent` 和 `IfNotExists`。这两个字段影响创建语句，不代表数据库终态；DB introspect 无法恢复 `CONCURRENTLY` 和 `IF NOT EXISTS`。

- [ ] **步骤 2.5：验证并提交**

运行：

```bash
rtk go test ./internal/render ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/render/render.go internal/render/render_test.go internal/diff/diff_tables.go internal/diff/diff_index_test.go
rtk git commit -m "fix(index): 补齐索引方法渲染和元素比较"
```

## 任务 3：结构化还原 DB 高级索引定义

**文件：**
- 修改：`internal/introspect/indexes.go`
- 创建：`internal/introspect/indexes_test.go`
- 修改：`internal/normalize/normalize.go`
- 修改：`docs/DDL.md`

- [ ] **步骤 3.1：为索引 row 组装编写单元测试**

创建 `internal/introspect/indexes_test.go`：

```go
package introspect

import "testing"

func TestParseIndexElementDefinitions(t *testing.T) {
	got, err := parseIndexElementDefinitions([]string{
		`email COLLATE "C" text_pattern_ops DESC NULLS LAST`,
		`(lower(name))`,
	})
	if err != nil {
		t.Fatalf("parseIndexElementDefinitions failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	if got[0].Name != "email" || got[0].Collation != "C" || got[0].Opclass != "text_pattern_ops" || got[0].Ordering != "DESC" || got[0].NullsOrdering != "LAST" {
		t.Fatalf("unexpected first element: %#v", got[0])
	}
	if got[1].Expr != "lower(name)" {
		t.Fatalf("unexpected expression element: %#v", got[1])
	}
}
```

运行：

```bash
rtk go test ./internal/introspect -run TestParseIndexElementDefinitions -v
```

预期：失败，helper 尚不存在。

- [ ] **步骤 3.2：实现 helper**

在 `internal/introspect/indexes.go` 添加：

```go
func parseIndexElementDefinitions(defs []string) ([]model.IndexElem, error) {
	elements := make([]model.IndexElem, 0, len(defs))
	for _, def := range defs {
		elem, err := indexdef.ParseElement(def)
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}
	return elements, nil
}
```

并引入：

```go
import "github.com/fred29910/migra-go/internal/indexdef"
```

- [ ] **步骤 3.3：修改 introspect SQL**

将 `loadIndexes` 查询改成按索引聚合 element definition：

```sql
SELECT
	idx.relname AS index_name,
	t.relname AS table_name,
	array_agg(a.attname ORDER BY key_pos.n) FILTER (WHERE a.attname IS NOT NULL) AS column_names,
	array_agg(pg_get_indexdef(i.indexrelid, key_pos.n, true) ORDER BY key_pos.n) AS element_defs,
	i.indisunique AS is_unique,
	am.amname AS method,
	pg_get_indexdef(i.indexrelid) AS definition,
	COALESCE(pg_get_expr(i.indpred, i.indrelid), '') AS predicate
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_class t ON t.oid = i.indrelid
JOIN pg_namespace n ON n.oid = idx.relnamespace
JOIN pg_am am ON am.oid = idx.relam
LEFT JOIN LATERAL generate_series(1, i.indnkeyatts) AS key_pos(n) ON true
LEFT JOIN LATERAL unnest(i.indkey::int[]) WITH ORDINALITY AS k(attnum, ordinality) ON k.ordinality = key_pos.n
LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE n.nspname = $1 AND i.indisprimary = false
GROUP BY idx.oid, idx.relname, t.relname, i.indisunique, am.amname, i.indexrelid, i.indpred, i.indrelid
```

说明：`pg_get_indexdef(index_oid, column_no, true)` 会返回单个索引键定义，包含表达式、opclass、collation、排序和 NULLS 子句；`indnkeyatts` 排除 `INCLUDE` 列。本计划不支持 `INCLUDE` 索引。

- [ ] **步骤 3.4：scan 并落结构化 Elements**

在 scan 变量中加入：

```go
var elementDefs []string
```

scan 顺序调整为：

```go
err := rows.Scan(&indexName, &tableName, &columnNames, &elementDefs, &isUnique, &method, &definition, &predicate)
```

构造模型前解析元素：

```go
elements, err := parseIndexElementDefinitions(elementDefs)
if err != nil {
	return fmt.Errorf("parse index %s elements: %w", indexName, err)
}
index := &model.Index{
	Name:        indexName,
	Table:       tableName,
	Columns:     columnNames,
	Elements:    elements,
	Unique:      isUnique,
	Method:      method,
	Definition:  definition,
	WhereClause: predicate,
}
```

- [ ] **步骤 3.5：保留 normalize 兼容逻辑**

确认 `canonicalizeIndexInPlace` 只在 `len(idx.Elements) == 0 && len(idx.Columns) > 0` 时从 `Columns` 回填 `Elements`。DB introspect 新路径会填充 `Elements`，不应被旧 `Columns` 覆盖。

- [ ] **步骤 3.6：更新文档**

在 `docs/DDL.md` 的索引章节中，将 DB introspect 状态改为：

```markdown
| 表达式索引 (`ON tbl (lower(col))`) | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
| 操作符类 (`text_pattern_ops`, 等) | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
| 排序规则 (`ASC`/`DESC`, `NULLS FIRST/LAST`) | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
```

保留一条限制说明：

```markdown
> **限制**：`INCLUDE` 列、存储参数和表达式等价性规范化仍未支持；表达式按 PostgreSQL 反解析文本比较。
```

- [ ] **步骤 3.7：验证并提交**

运行：

```bash
rtk go test ./internal/indexdef ./internal/introspect ./internal/normalize ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/introspect/indexes.go internal/introspect/indexes_test.go internal/normalize/normalize.go docs/DDL.md
rtk git commit -m "feat(introspect): 结构化还原高级索引定义"
```

## 任务 4：新增 view、sequence、extension 模型

**文件：**
- 修改：`internal/model/schema.go`
- 修改：`internal/model/object_key.go`
- 创建：`internal/model/view.go`
- 创建：`internal/model/sequence.go`
- 创建：`internal/model/extension.go`
- 修改：`internal/model/schema_test.go`

- [ ] **步骤 4.1：编写失败测试**

在 `internal/model/schema_test.go` 添加：

```go
func TestNewNamespace_InitializesObjectMaps(t *testing.T) {
	ns := NewNamespace("public")
	if ns.Tables == nil || ns.Types == nil || ns.Views == nil || ns.Sequences == nil || ns.Extensions == nil {
		t.Fatalf("namespace maps must be initialized: %#v", ns)
	}
}
```

运行：

```bash
rtk go test ./internal/model -run TestNewNamespace_InitializesObjectMaps -v
```

预期：失败，`NewNamespace` 和新对象 map 尚不存在。

- [ ] **步骤 4.2：新增模型文件**

创建 `internal/model/view.go`：

```go
package model

type View struct {
	Name         string
	Definition   string
	Materialized bool
}
```

创建 `internal/model/sequence.go`：

```go
package model

type Sequence struct {
	Name        string
	DataType    string
	StartValue  int64
	IncrementBy int64
	MinValue    int64
	MaxValue    int64
	CacheSize   int64
	Cycle       bool
	OwnedByTable  string
	OwnedByColumn string
}
```

创建 `internal/model/extension.go`：

```go
package model

type Extension struct {
	Name    string
	Version string
}
```

- [ ] **步骤 4.3：扩展 Namespace**

在 `internal/model/schema.go` 中调整：

```go
type Namespace struct {
	Name       string
	Tables     map[string]*Table
	Types      map[string]*EnumType
	Views      map[string]*View
	Sequences  map[string]*Sequence
	Extensions map[string]*Extension
}

func NewNamespace(name string) *Namespace {
	return &Namespace{
		Name:       name,
		Tables:     make(map[string]*Table),
		Types:      make(map[string]*EnumType),
		Views:      make(map[string]*View),
		Sequences:  make(map[string]*Sequence),
		Extensions: make(map[string]*Extension),
	}
}
```

将 `GetOrCreateNamespace` 中的 literal 替换为：

```go
ns := NewNamespace(name)
```

- [ ] **步骤 4.4：补充 ObjectKind**

在 `internal/model/object_key.go` 中保留已有 `KindView`，新增：

```go
KindSequence  ObjectKind = "sequence"
KindExtension ObjectKind = "extension"
```

- [ ] **步骤 4.5：替换其他 Namespace literal**

将 `internal/introspect/introspect.go` 中的 namespace 初始化改为：

```go
ns := model.NewNamespace(schemaName)
```

在测试中若有手写 `model.Namespace{...}`，补齐新 map 或改为 `model.NewNamespace`。

- [ ] **步骤 4.6：验证并提交**

运行：

```bash
rtk go test ./internal/model ./internal/introspect ./internal/parser
```

预期：全部通过。

提交：

```bash
rtk git add internal/model/schema.go internal/model/object_key.go internal/model/view.go internal/model/sequence.go internal/model/extension.go internal/model/schema_test.go internal/introspect/introspect.go
rtk git commit -m "feat(model): 添加 view sequence extension 模型"
```

## 任务 5：实现 P3 对象 parser 与 mutation

**文件：**
- 创建：`internal/parser/view_handler.go`
- 创建：`internal/parser/sequence_handler.go`
- 创建：`internal/parser/extension_handler.go`
- 修改：`internal/parser/registry.go`
- 修改：`internal/parser/parser_test.go`

- [ ] **步骤 5.1：编写 parser 失败测试**

在 `internal/parser/parser_test.go` 添加：

```go
func TestParseP3Objects(t *testing.T) {
	sql := `
CREATE VIEW active_users AS SELECT id, email FROM users WHERE active = true;
CREATE SEQUENCE invoice_id_seq AS bigint START WITH 100 INCREMENT BY 5 CACHE 20 CYCLE;
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH VERSION '1.3';
`
	schema, err := NewParser().ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns.Views["active_users"] == nil {
		t.Fatal("expected active_users view")
	}
	if ns.Sequences["invoice_id_seq"] == nil {
		t.Fatal("expected invoice_id_seq sequence")
	}
	if ns.Extensions["pgcrypto"] == nil {
		t.Fatal("expected pgcrypto extension")
	}
}
```

运行：

```bash
rtk go test ./internal/parser -run TestParseP3Objects -v
```

预期：失败，registry 尚未注册新 handler。

- [ ] **步骤 5.2：实现 view handler**

创建 `internal/parser/view_handler.go`：

```go
package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type CreateViewHandler struct{}

func (h *CreateViewHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetViewStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateViewHandler: expected ViewStmt, got %T", node)
	}
	name, schemaName := parserutil.ParseRelation(stmt.View)
	definition := strings.TrimSuffix(parserutil.DeparseNode(stmt.Query), ";")
	return []SchemaMutation{CreateViewMutation{
		Schema: schemaName,
		View: model.View{Name: name, Definition: definition, Materialized: false},
	}}, nil
}

type CreateViewMutation struct {
	Schema string
	View   model.View
}

func (m CreateViewMutation) Kind() MutationKind { return MutationKind("create_view") }
func (m CreateViewMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.View.Name, model.KindView)
}
func (m CreateViewMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	view := m.View
	ns.Views[view.Name] = &view
	return nil
}
```

如果编译提示 pg_query_go v6 的 materialized view 字段名不同，本任务只保持普通 `CREATE VIEW` 通过；物化视图放在任务 8 的 introspect 文档限制中标记为不支持创建解析。

- [ ] **步骤 5.3：实现 sequence handler**

创建 `internal/parser/sequence_handler.go`：

```go
package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type CreateSequenceHandler struct{}

func (h *CreateSequenceHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateSeqStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateSequenceHandler: expected CreateSeqStmt, got %T", node)
	}
	name, schemaName := parserutil.ParseRelation(stmt.Sequence)
	seq := model.Sequence{Name: name, DataType: "bigint", IncrementBy: 1, CacheSize: 1}
	applySequenceOptions(&seq, stmt.Options)
	return []SchemaMutation{CreateSequenceMutation{Schema: schemaName, Sequence: seq}}, nil
}

func applySequenceOptions(seq *model.Sequence, options []*pg_query.Node) {
	for _, node := range options {
		def := node.GetDefElem()
		if def == nil {
			continue
		}
		switch strings.ToLower(def.Defname) {
		case "as":
			seq.DataType = sequenceStringValue(def.Arg)
		case "increment":
			seq.IncrementBy = sequenceIntValue(def.Arg)
		case "start":
			seq.StartValue = sequenceIntValue(def.Arg)
		case "minvalue":
			seq.MinValue = sequenceIntValue(def.Arg)
		case "maxvalue":
			seq.MaxValue = sequenceIntValue(def.Arg)
		case "cache":
			seq.CacheSize = sequenceIntValue(def.Arg)
		case "cycle":
			seq.Cycle = true
		}
	}
}
```

同文件补齐 `sequenceIntValue`、`sequenceStringValue`：

```go
func sequenceIntValue(node *pg_query.Node) int64 {
	if node == nil {
		return 0
	}
	if a := node.GetAConst(); a != nil {
		if i := a.GetIval(); i != nil {
			return int64(i.Ival)
		}
	}
	return 0
}

func sequenceStringValue(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	if tn := node.GetTypeName(); tn != nil {
		return parserutil.ParseTypeName(tn)
	}
	if s := node.GetString_(); s != nil {
		return s.Sval
	}
	return ""
}
```

添加 mutation：

```go
type CreateSequenceMutation struct {
	Schema   string
	Sequence model.Sequence
}

func (m CreateSequenceMutation) Kind() MutationKind { return MutationKind("create_sequence") }
func (m CreateSequenceMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Sequence.Name, model.KindSequence)
}
func (m CreateSequenceMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	seq := m.Sequence
	ns.Sequences[seq.Name] = &seq
	return nil
}
```

- [ ] **步骤 5.4：实现 extension handler**

创建 `internal/parser/extension_handler.go`：

```go
package parser

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type CreateExtensionHandler struct{}

func (h *CreateExtensionHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateExtensionStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateExtensionHandler: expected CreateExtensionStmt, got %T", node)
	}
	ext := model.Extension{Name: stmt.Extname}
	schemaName := "public"
	for _, item := range stmt.Options {
		def := item.GetDefElem()
		if def == nil {
			continue
		}
		switch strings.ToLower(def.Defname) {
		case "schema":
			if s := def.Arg.GetString_(); s != nil {
				schemaName = s.Sval
			}
		case "new_version":
			if s := def.Arg.GetString_(); s != nil {
				ext.Version = s.Sval
			}
		}
	}
	return []SchemaMutation{CreateExtensionMutation{Schema: schemaName, Extension: ext}}, nil
}

type CreateExtensionMutation struct {
	Schema    string
	Extension model.Extension
}

func (m CreateExtensionMutation) Kind() MutationKind { return MutationKind("create_extension") }
func (m CreateExtensionMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Extension.Name, model.KindExtension)
}
func (m CreateExtensionMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	ext := m.Extension
	ns.Extensions[ext.Name] = &ext
	return nil
}
```

- [ ] **步骤 5.5：注册 handler**

在 `internal/parser/registry.go` 的 `DefaultRegistry` 添加：

```go
r.Register(&pg_query.Node{Node: &pg_query.Node_ViewStmt{}}, &CreateViewHandler{})
r.Register(&pg_query.Node{Node: &pg_query.Node_CreateSeqStmt{}}, &CreateSequenceHandler{})
r.Register(&pg_query.Node{Node: &pg_query.Node_CreateExtensionStmt{}}, &CreateExtensionHandler{})
```

- [ ] **步骤 5.6：验证并提交**

运行：

```bash
rtk go test ./internal/parser
```

预期：全部通过。

提交：

```bash
rtk git add internal/parser/view_handler.go internal/parser/sequence_handler.go internal/parser/extension_handler.go internal/parser/registry.go internal/parser/parser_test.go
rtk git commit -m "feat(parser): 支持 view sequence extension"
```

## 任务 6：实现 P3 对象 diff operation

**文件：**
- 修改：`internal/diff/operation.go`
- 修改：`internal/diff/differ.go`
- 创建：`internal/diff/diff_objects_test.go`

- [ ] **步骤 6.1：编写失败测试**

创建 `internal/diff/diff_objects_test.go`：

```go
package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffP3Objects_AddChangeDrop(t *testing.T) {
	source := model.NewSchema()
	srcNS := source.GetOrCreateNamespace("public")
	srcNS.Views["old_view"] = &model.View{Name: "old_view", Definition: "SELECT 1"}
	srcNS.Sequences["invoice_id_seq"] = &model.Sequence{Name: "invoice_id_seq", DataType: "bigint", StartValue: 1}
	srcNS.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.2"}

	target := model.NewSchema()
	tgtNS := target.GetOrCreateNamespace("public")
	tgtNS.Views["active_users"] = &model.View{Name: "active_users", Definition: "SELECT id FROM users"}
	tgtNS.Sequences["invoice_id_seq"] = &model.Sequence{Name: "invoice_id_seq", DataType: "bigint", StartValue: 100}
	tgtNS.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.3"}

	ops, _ := NewDiffer().Diff(source, target)
	kinds := map[Kind]bool{}
	for _, op := range ops {
		kinds[op.Kind()] = true
	}
	for _, want := range []Kind{KindCreateView, KindDropView, KindAlterSequence, KindAlterExtensionUpdate} {
		if !kinds[want] {
			t.Fatalf("expected %s in ops %#v", want, ops)
		}
	}
}
```

运行：

```bash
rtk go test ./internal/diff -run TestDiffP3Objects_AddChangeDrop -v
```

预期：编译失败，新 operation 尚不存在。

- [ ] **步骤 6.2：新增 operation kind 和结构体**

在 `internal/diff/operation.go` 添加：

```go
KindCreateView           Kind = "create_view"
KindDropView             Kind = "drop_view"
KindReplaceView          Kind = "replace_view"
KindCreateSequence       Kind = "create_sequence"
KindDropSequence         Kind = "drop_sequence"
KindAlterSequence        Kind = "alter_sequence"
KindCreateExtension      Kind = "create_extension"
KindDropExtension        Kind = "drop_extension"
KindAlterExtensionUpdate Kind = "alter_extension_update"
```

添加构造函数，保持字段结构明确：

```go
type CreateViewOp struct { baseOperation; Schema string; View *model.View }
type DropViewOp struct { baseOperation; Schema string; Name string }
type ReplaceViewOp struct { baseOperation; Schema string; View *model.View }
type CreateSequenceOp struct { baseOperation; Schema string; Sequence *model.Sequence }
type DropSequenceOp struct { baseOperation; Schema string; Name string }
type AlterSequenceOp struct { baseOperation; Schema string; From *model.Sequence; To *model.Sequence }
type CreateExtensionOp struct { baseOperation; Schema string; Extension *model.Extension }
type DropExtensionOp struct { baseOperation; Schema string; Name string }
type AlterExtensionUpdateOp struct { baseOperation; Schema string; Extension *model.Extension }
```

每个构造函数使用对应 `model.KindView`、`model.KindSequence`、`model.KindExtension`。`Drop*Op.IsDestructive()` 返回 true；`ReplaceViewOp`、`AlterSequenceOp`、`AlterExtensionUpdateOp` 返回 false。

- [ ] **步骤 6.3：实现 namespace diff 入口**

在 `internal/diff/differ.go` 的 `diffNamespace` 末尾添加：

```go
c.diffViews(source, target)
c.diffSequences(source, target)
c.diffExtensions(source, target)
```

实现 `diffViews`：

```go
func (c *diffContext) diffViews(source, target *model.Namespace) {
	if source == nil {
		for _, name := range sortedViewNames(target.Views) {
			c.addOp(NewCreateViewOp(target.Name, target.Views[name]))
		}
		return
	}
	for _, name := range sortedViewNames(target.Views) {
		if src, ok := source.Views[name]; !ok {
			c.addOp(NewCreateViewOp(target.Name, target.Views[name]))
		} else if src.Definition != target.Views[name].Definition || src.Materialized != target.Views[name].Materialized {
			c.addOp(NewReplaceViewOp(target.Name, target.Views[name]))
		}
	}
	for _, name := range sortedViewNames(source.Views) {
		if _, ok := target.Views[name]; !ok {
			c.addOp(NewDropViewOp(source.Name, name))
		}
	}
}
```

按同样模式实现 `diffSequences` 和 `diffExtensions`：

- sequence 新增：`CreateSequenceOp`
- sequence 缺失：`DropSequenceOp`
- sequence 任一结构化字段变化：`AlterSequenceOp`
- extension 新增：`CreateExtensionOp`
- extension 缺失：`DropExtensionOp`
- extension version 从非空变为不同非空：`AlterExtensionUpdateOp`

- [ ] **步骤 6.4：验证并提交**

运行：

```bash
rtk go test ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/diff/operation.go internal/diff/differ.go internal/diff/diff_objects_test.go
rtk git commit -m "feat(diff): 支持 view sequence extension 差异"
```

## 任务 7：渲染 P3 对象 SQL 并归类执行阶段

**文件：**
- 修改：`internal/render/render.go`
- 修改：`internal/render/render_test.go`
- 修改：`internal/plan/plan.go`
- 修改：`internal/plan/plan_test.go`

- [ ] **步骤 7.1：编写 renderer 失败测试**

在 `internal/render/render_test.go` 添加：

```go
func TestRenderP3Objects(t *testing.T) {
	r := NewRenderer()
	cases := map[string]diff.Operation{
		`CREATE VIEW "public"."active_users" AS SELECT id FROM users;`: diff.NewCreateViewOp("public", &model.View{Name: "active_users", Definition: "SELECT id FROM users"}),
		`DROP VIEW IF EXISTS "public"."old_view";`: diff.NewDropViewOp("public", "old_view"),
		`CREATE SEQUENCE "public"."invoice_id_seq" AS bigint START WITH 100 INCREMENT BY 5 CACHE 20 CYCLE;`: diff.NewCreateSequenceOp("public", &model.Sequence{Name: "invoice_id_seq", DataType: "bigint", StartValue: 100, IncrementBy: 5, CacheSize: 20, Cycle: true}),
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto" WITH VERSION '1.3';`: diff.NewCreateExtensionOp("public", &model.Extension{Name: "pgcrypto", Version: "1.3"}),
		`ALTER EXTENSION "pgcrypto" UPDATE TO '1.3';`: diff.NewAlterExtensionUpdateOp("public", &model.Extension{Name: "pgcrypto", Version: "1.3"}),
	}
	for want, op := range cases {
		if got := r.Render(op); !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
}
```

运行：

```bash
rtk go test ./internal/render -run TestRenderP3Objects -v
```

预期：失败，renderer switch 尚未处理新 operation。

- [ ] **步骤 7.2：实现 renderer switch**

在 `Render` switch 添加：

```go
case *diff.CreateViewOp:
	return r.renderCreateView(v)
case *diff.DropViewOp:
	return r.renderDropView(v)
case *diff.ReplaceViewOp:
	return r.renderReplaceView(v)
case *diff.CreateSequenceOp:
	return r.renderCreateSequence(v)
case *diff.DropSequenceOp:
	return r.renderDropSequence(v)
case *diff.AlterSequenceOp:
	return r.renderAlterSequence(v)
case *diff.CreateExtensionOp:
	return r.renderCreateExtension(v)
case *diff.DropExtensionOp:
	return r.renderDropExtension(v)
case *diff.AlterExtensionUpdateOp:
	return r.renderAlterExtensionUpdate(v)
```

实现函数：

```go
func (r *Renderer) renderCreateView(op *diff.CreateViewOp) string {
	return fmt.Sprintf("-- op: create_view risk:low\nCREATE VIEW %s AS %s;",
		quoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func (r *Renderer) renderReplaceView(op *diff.ReplaceViewOp) string {
	return fmt.Sprintf("-- op: replace_view risk:medium\nCREATE OR REPLACE VIEW %s AS %s;",
		quoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func (r *Renderer) renderDropView(op *diff.DropViewOp) string {
	return fmt.Sprintf("-- op: drop_view risk:high\nDROP VIEW %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}
```

Sequence renderer 按非零字段输出：

```go
func (r *Renderer) renderCreateSequence(op *diff.CreateSequenceOp) string {
	seq := op.Sequence
	parts := []string{"CREATE SEQUENCE " + quoteQualifiedIdentifier(op.Schema, seq.Name)}
	if seq.DataType != "" {
		parts = append(parts, "AS "+seq.DataType)
	}
	if seq.StartValue != 0 {
		parts = append(parts, fmt.Sprintf("START WITH %d", seq.StartValue))
	}
	if seq.IncrementBy != 0 {
		parts = append(parts, fmt.Sprintf("INCREMENT BY %d", seq.IncrementBy))
	}
	if seq.MinValue != 0 {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", seq.MinValue))
	}
	if seq.MaxValue != 0 {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", seq.MaxValue))
	}
	if seq.CacheSize != 0 {
		parts = append(parts, fmt.Sprintf("CACHE %d", seq.CacheSize))
	}
	if seq.Cycle {
		parts = append(parts, "CYCLE")
	}
	return "-- op: create_sequence risk:low\n" + strings.Join(parts, " ") + ";"
}
```

Extension renderer：

```go
func (r *Renderer) renderCreateExtension(op *diff.CreateExtensionOp) string {
	sql := "CREATE EXTENSION IF NOT EXISTS " + quoteIdentifier(op.Extension.Name)
	if op.Extension.Version != "" {
		sql += " WITH VERSION " + quoteString(op.Extension.Version)
	}
	return "-- op: create_extension risk:low\n" + sql + ";"
}
```

- [ ] **步骤 7.3：补充 plan 阶段测试**

在 `internal/plan/plan_test.go` 添加：

```go
func TestPlanner_P3ObjectStages(t *testing.T) {
	stages := NewPlanner(true).Plan([]diff.Operation{
		diff.NewCreateViewOp("public", &model.View{Name: "v"}),
		diff.NewDropViewOp("public", "old_v"),
		diff.NewCreateSequenceOp("public", &model.Sequence{Name: "s"}),
		diff.NewDropExtensionOp("public", "old_ext"),
	})
	if len(stages[StagePreDeploy]) != 2 {
		t.Fatalf("expected 2 pre-deploy ops, got %#v", stages[StagePreDeploy])
	}
	if len(stages[StagePostDeploy]) != 2 {
		t.Fatalf("expected 2 post-deploy ops, got %#v", stages[StagePostDeploy])
	}
}
```

- [ ] **步骤 7.4：更新执行阶段**

在 `assignStage` 中：

- pre-deploy 加入 `KindCreateView`、`KindCreateSequence`、`KindCreateExtension`。
- deploy 加入 `KindReplaceView`、`KindAlterSequence`、`KindAlterExtensionUpdate`。
- post-deploy 加入 `KindDropView`、`KindDropSequence`、`KindDropExtension`。

- [ ] **步骤 7.5：验证并提交**

运行：

```bash
rtk go test ./internal/render ./internal/plan ./internal/app
```

预期：全部通过。

提交：

```bash
rtk git add internal/render/render.go internal/render/render_test.go internal/plan/plan.go internal/plan/plan_test.go
rtk git commit -m "feat(render): 输出 view sequence extension DDL"
```

## 任务 8：实现 P3 对象 DB introspect 与 normalize

**文件：**
- 创建：`internal/introspect/views.go`
- 创建：`internal/introspect/sequences.go`
- 创建：`internal/introspect/extensions.go`
- 修改：`internal/introspect/introspect.go`
- 修改：`internal/normalize/normalize.go`
- 创建：`internal/normalize/objects_test.go`

- [ ] **步骤 8.1：编写 normalize 失败测试**

创建 `internal/normalize/objects_test.go`：

```go
package normalize

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestCanonicalizeSchema_P3Objects(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	ns.Views["v"] = &model.View{Name: `"V"`, Definition: "SELECT  id  FROM users;"}
	ns.Extensions["pgcrypto"] = &model.Extension{Name: `"PgCrypto"`, Version: "1.3"}
	ns.Sequences["seq"] = &model.Sequence{Name: `"Seq"`, DataType: "INT8"}

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	if ns.Views["v"].Definition != "SELECT id FROM users" {
		t.Fatalf("unexpected view definition: %q", ns.Views["v"].Definition)
	}
	if ns.Extensions["pgcrypto"].Name != "PgCrypto" {
		t.Fatalf("quoted extension name should preserve case, got %q", ns.Extensions["pgcrypto"].Name)
	}
	if ns.Sequences["seq"].DataType != "bigint" {
		t.Fatalf("expected bigint, got %q", ns.Sequences["seq"].DataType)
	}
}
```

运行：

```bash
rtk go test ./internal/normalize -run TestCanonicalizeSchema_P3Objects -v
```

预期：失败，normalize 尚未处理新对象。

- [ ] **步骤 8.2：实现 views loader**

创建 `internal/introspect/views.go`：

```go
package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

func loadViews(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
SELECT viewname, definition, false AS materialized
FROM pg_views
WHERE schemaname = $1
UNION ALL
SELECT matviewname, definition, true AS materialized
FROM pg_matviews
WHERE schemaname = $1`
	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query views: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, definition string
		var materialized bool
		if err := rows.Scan(&name, &definition, &materialized); err != nil {
			return fmt.Errorf("scan view row: %w", err)
		}
		ns.Views[name] = &model.View{Name: name, Definition: strings.TrimSuffix(strings.TrimSpace(definition), ";"), Materialized: materialized}
	}
	return rows.Err()
}
```

- [ ] **步骤 8.3：实现 sequences loader**

创建 `internal/introspect/sequences.go`：

```go
package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

func loadSequences(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
SELECT sequencename, data_type, start_value, min_value, max_value, increment_by, cycle, cache_size
FROM pg_sequences
WHERE schemaname = $1`
	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query sequences: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		seq := &model.Sequence{}
		if err := rows.Scan(&seq.Name, &seq.DataType, &seq.StartValue, &seq.MinValue, &seq.MaxValue, &seq.IncrementBy, &seq.Cycle, &seq.CacheSize); err != nil {
			return fmt.Errorf("scan sequence row: %w", err)
		}
		ns.Sequences[seq.Name] = seq
	}
	return rows.Err()
}
```

- [ ] **步骤 8.4：实现 extensions loader**

创建 `internal/introspect/extensions.go`：

```go
package introspect

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
)

func loadExtensions(ctx context.Context, conn *pgx.Conn, schemaName string, ns *model.Namespace) error {
	query := `
SELECT e.extname, e.extversion
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE n.nspname = $1`
	rows, err := conn.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query extensions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		ext := &model.Extension{}
		if err := rows.Scan(&ext.Name, &ext.Version); err != nil {
			return fmt.Errorf("scan extension row: %w", err)
		}
		ns.Extensions[ext.Name] = ext
	}
	return rows.Err()
}
```

- [ ] **步骤 8.5：接入 loader**

在 `loadNamespace` 中，`loadEnumTypes` 之后添加：

```go
if err := loadViews(ctx, conn, schemaName, ns); err != nil {
	return nil, fmt.Errorf("failed to load views: %w", err)
}
if err := loadSequences(ctx, conn, schemaName, ns); err != nil {
	return nil, fmt.Errorf("failed to load sequences: %w", err)
}
if err := loadExtensions(ctx, conn, schemaName, ns); err != nil {
	return nil, fmt.Errorf("failed to load extensions: %w", err)
}
```

- [ ] **步骤 8.6：实现 normalize**

在 `canonicalizeNamespaceInPlace` 中添加：

```go
for _, view := range ns.Views {
	canonicalizeViewInPlace(view)
}
for _, seq := range ns.Sequences {
	canonicalizeSequenceInPlace(seq)
}
for _, ext := range ns.Extensions {
	canonicalizeExtensionInPlace(ext)
}
```

新增函数：

```go
func canonicalizeViewInPlace(v *model.View) {
	v.Name = normalizeIdentifier(v.Name)
	v.Definition = strings.TrimSuffix(strings.Join(strings.Fields(v.Definition), " "), ";")
}

func canonicalizeSequenceInPlace(s *model.Sequence) {
	s.Name = normalizeIdentifier(s.Name)
	s.DataType = normalizeDataType(s.DataType)
}

func canonicalizeExtensionInPlace(e *model.Extension) {
	e.Name = normalizeIdentifier(e.Name)
}
```

- [ ] **步骤 8.7：验证并提交**

运行：

```bash
rtk go test ./internal/introspect ./internal/normalize ./internal/diff
```

预期：全部通过。

提交：

```bash
rtk git add internal/introspect/views.go internal/introspect/sequences.go internal/introspect/extensions.go internal/introspect/introspect.go internal/normalize/normalize.go internal/normalize/objects_test.go
rtk git commit -m "feat(introspect): 读取 view sequence extension"
```

## 任务 9：端到端文档同步与全量验证

**文件：**
- 修改：`docs/DDL.md`
- 修改：`docs/plan/feat_ext.md`

- [ ] **步骤 9.1：更新 DDL 支持矩阵**

在 `docs/DDL.md` 的高级对象章节中更新：

```markdown
| **视图** (`CREATE VIEW`) | ✅ | 支持普通 view 的 parse、diff、render、introspect；物化视图仅 introspect 建模，创建渲染不在本轮支持。 |
| **序列** (`CREATE SEQUENCE`) | ✅ | 支持结构化 sequence 选项：类型、start、increment、min/max、cache、cycle。 |
| **扩展** (`CREATE EXTENSION`) | ✅ | 支持 create/drop 和版本更新；schema 按 `pg_extension.extnamespace` 建模。 |
| **触发器** (`CREATE TRIGGER`) | ❌ | 未实现，需独立 model 与依赖函数语义。 |
| **行级安全策略** (`CREATE POLICY`) | ❌ | 未实现，需独立 policy model。 |
| **函数 / 过程** (`CREATE FUNCTION/PROCEDURE`) | ❌ | 未实现，需函数签名级 ObjectKey 与 body diff 语义。 |
```

索引章节更新为 P2 完成状态，并保留 `INCLUDE`、storage parameter 未支持限制。

- [ ] **步骤 9.2：更新 feat_ext 结论**

在 `docs/plan/feat_ext.md` 的 P2/P3 小节后添加完成说明：

```markdown
> 实施计划：`docs/superpowers/plans/2026-05-21-p2-p3-index-object-expansion.md`。
>
> P2 目标是完成 SQL 文件和 DB introspect 的结构化索引闭环。P3 首批对象限定为 view、sequence、extension；function、trigger、policy 保持为下一阶段扩展。
```

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
rtk git commit -m "docs(DDL): 同步 P2 P3 扩展支持状态"
```

## 执行建议

建议拆成 4 个检查点：

1. **索引解析闭环：** 任务 1、任务 2。
2. **索引 DB introspect：** 任务 3。
3. **P3 对象模型与 SQL 文件路径：** 任务 4、任务 5、任务 6、任务 7。
4. **P3 DB introspect 与文档：** 任务 8、任务 9。

每个检查点结束时运行对应任务的测试命令，并提交一次。进入下一检查点前，工作区应只包含当前检查点相关变更。

## 自检清单

- [x] **规格覆盖度：** 覆盖 P2 的 renderer、partial index、表达式索引、opclass、collation、NULLS、DB introspect 结构化还原；覆盖 P3 首批 view、sequence、extension。
- [x] **任务独立性：** 每个任务都有明确文件、失败测试、实现步骤、验证命令和提交命令。
- [x] **类型一致性：** 索引使用 `model.Index` / `model.IndexElem`；view、sequence、extension 使用独立模型和 operation。
- [x] **范围控制：** function、trigger、policy 不在本计划实现，只保留下一阶段边界。
- [x] **验证闭环：** 每个任务都有局部测试，最终任务有全量 `rtk go test ./...` 与 `rtk git diff --check`。
