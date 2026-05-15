# 多 Schema Diff 修复实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复 `migra diff --schema public --schema auth` 对多 Schema SQL 文件/目录源的解析失败问题，使 `--schema` 过滤对文件源也生效。

**架构：** 两个独立问题叠加导致：(1) Parser 缺少 `CreateSchemaStmt` Handler，遇到 `CREATE SCHEMA` 直接崩溃；(2) `--schema` 过滤逻辑只作用于 DBLoader，未对 DirectoryLoader/SQLFileLoader 生效。修复方案：新增 `CreateSchemaHandler` + `CreateSchemaMutation` 使解析通过；在 `ComputeDiff` 阶段添加 `FilterNamespaces` 使 `--schema` 语义一致。

**技术栈：** Go 1.26, pg_query_go v1.0.2 (AST 解析), pg_nodes (AST 节点类型)

---

## 文件结构

### 新增文件

| 文件 | 职责 |
|------|------|
| `internal/parser/create_schema_handler.go` | `CreateSchemaHandler`（实现 Handler 接口）+ `CreateSchemaMutation`（实现 SchemaMutation 接口） |

### 修改文件

| 文件 | 职责 | 改动范围 |
|------|------|----------|
| `internal/model/object_key.go` | 定义 ObjectKind 常量 | 新增 `KindSchema` 常量 |
| `internal/parser/registry.go` | Handler 注册中心 | `DefaultRegistry()` 中注册 `CreateSchemaStmt` |
| `internal/app/diff_service.go` | Diff 服务编排 | 新增 `FilterNamespaces` 函数 + `ComputeDiff` 中调用 |
| `internal/parser/handler_test.go` | Handler 单元测试 | 新增 `TestCreateSchemaHandler` + 类型安全检查 |
| `internal/parser/mutation_test.go` | Mutation 单元测试 | 新增 `TestCreateSchemaMutation_Apply` |
| `internal/app/diff_service_test.go` | Diff 服务测试 | 新增 `TestFilterNamespaces` |
| `cmd/migra/integration_test.go` | 集成测试 | 新增 `TestMultiSchemaDiff` |
| `docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md` | Bug 报告 | 状态改为 Fixed，添加修复记录 |

---

### 任务 1：新增 `KindSchema` 模型常量

**文件：** `internal/model/object_key.go`

- [ ] **步骤 1：添加 `KindSchema` 常量**

在 `KindFunction` 后面添加新常量：

```go
const (
    KindTable      ObjectKind = "table"
    KindColumn     ObjectKind = "column"
    KindIndex      ObjectKind = "index"
    KindConstraint ObjectKind = "constraint"
    KindType       ObjectKind = "type"
    KindView       ObjectKind = "view"
    KindFunction   ObjectKind = "function"
    KindSchema     ObjectKind = "schema"  // ← 新增
)
```

- [ ] **步骤 2：运行现有测试确认无损**

```bash
go test ./internal/model/... -v
```
预期：PASS，无回归。

- [ ] **步骤 3：Commit**

```bash
git add internal/model/object_key.go
git commit -m "feat(model): add KindSchema constant for schema-level operations"
```

---

### 任务 2：创建 `CreateSchemaHandler` + `CreateSchemaMutation`

#### 2a：新增 MutationKind

**文件：** `internal/parser/mutation.go`

- [ ] **步骤 1：添加 `MutKindCreateSchema` 常量**

在 `MutKindDropDefault` 后面添加：

```go
const (
    MutKindCreateTable     MutationKind = "create_table"
    MutKindAddColumn       MutationKind = "add_column"
    MutKindCreateEnumType  MutationKind = "create_enum_type"
    MutKindCreateIndex     MutationKind = "create_index"
    MutKindDropColumn      MutationKind = "drop_column"
    MutKindAlterColumnType MutationKind = "alter_column_type"
    MutKindSetNotNull      MutationKind = "set_not_null"
    MutKindDropNotNull     MutationKind = "drop_not_null"
    MutKindSetDefault      MutationKind = "set_default"
    MutKindDropDefault     MutationKind = "drop_default"
    MutKindCreateSchema    MutationKind = "create_schema"  // ← 新增
)
```

#### 2b：编写 Handler 测试（先红后绿）

**文件：** `internal/parser/handler_test.go`

- [ ] **步骤 2：编写 Handler 单元测试**

在文件末尾添加：

```go
func TestCreateSchemaHandler(t *testing.T) {
    node := mustParseFirstStmt(t, "CREATE SCHEMA auth")
    h := &CreateSchemaHandler{}

    mutations, err := h.Handle(node)
    require.NoError(t, err)
    require.Len(t, mutations, 1)

    mut, ok := mutations[0].(CreateSchemaMutation)
    require.True(t, ok)
    assert.Equal(t, "auth", mut.Schema)
}
```

- [ ] **步骤 3：运行测试预期失败**

```bash
go test ./internal/parser/... -v -run TestCreateSchemaHandler
```
预期：编译错误，`CreateSchemaHandler` 和 `CreateSchemaMutation` 未定义。

#### 2c：编写 Handler 实现

**文件：** `internal/parser/create_schema_handler.go`（新文件）

- [ ] **步骤 4：创建 Handler 实现代码**

```go
package parser

import (
    "fmt"

    pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateSchemaHandler handles CREATE SCHEMA statements.
// Note: The SchemaElts field (child statements like CREATE TABLE embedded in
// CREATE SCHEMA) is intentionally not processed here, because pg_query_go
// already returns them as separate top-level statements in the AST.
type CreateSchemaHandler struct{}

// Handle converts a pg_query CreateSchemaStmt into schema mutations.
func (h *CreateSchemaHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
    stmt, ok := node.(pg_nodes.CreateSchemaStmt)
    if !ok {
        return nil, fmt.Errorf("CreateSchemaHandler: expected pg_nodes.CreateSchemaStmt, got %T", node)
    }
    if stmt.Schemaname == nil {
        return nil, fmt.Errorf("CreateSchemaHandler: schema name is nil")
    }
    schemaName := *stmt.Schemaname
    return []SchemaMutation{CreateSchemaMutation{Schema: schemaName}}, nil
}

// CreateSchemaMutation describes creating a namespace/schema.
type CreateSchemaMutation struct {
    Schema string
}

func (m CreateSchemaMutation) Kind() MutationKind   { return MutKindCreateSchema }
func (m CreateSchemaMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, "", model.KindSchema)
}
func (m CreateSchemaMutation) Apply(schema *model.Schema) error {
    schema.GetOrCreateNamespace(m.Schema)
    return nil
}
```

注意：需要在文件顶部添加 `"github.com/fred29910/migra-go/internal/model"` import。

- [ ] **步骤 5：运行测试验证通过**

```bash
go test ./internal/parser/... -v -run TestCreateSchemaHandler
```
预期：PASS

#### 2d：编写 Mutation 测试

**文件：** `internal/parser/mutation_test.go`

- [ ] **步骤 6：编写 Mutation 单元测试**

在文件末尾添加：

```go
func TestCreateSchemaMutation_Apply(t *testing.T) {
    schema := model.NewSchema()
    mut := CreateSchemaMutation{Schema: "auth"}

    err := mut.Apply(schema)
    require.NoError(t, err)

    ns := schema.GetNamespace("auth")
    require.NotNil(t, ns)
    assert.Equal(t, "auth", ns.Name)
    assert.Empty(t, ns.Tables)
    assert.Empty(t, ns.Types)
}

func TestCreateSchemaMutation_Apply_Idempotent(t *testing.T) {
    schema := model.NewSchema()
    schema.GetOrCreateNamespace("auth")

    mut := CreateSchemaMutation{Schema: "auth"}
    err := mut.Apply(schema)
    require.NoError(t, err, "CreateSchemaMutation should be idempotent")

    // Verify exactly one namespace, no duplicates
    require.Len(t, schema.Schemas, 1)
    ns := schema.GetNamespace("auth")
    require.NotNil(t, ns)
}
```

- [ ] **步骤 7：运行测试验证通过**

```bash
go test ./internal/parser/... -v -run TestCreateSchemaMutation
```
预期：PASS

- [ ] **步骤 8：注册到 DefaultRegistry**

**文件：** `internal/parser/registry.go`，在 `DefaultRegistry()` 中添加一行：

```go
func DefaultRegistry() *HandlerRegistry {
    r := NewHandlerRegistry()
    r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
    r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
    r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})
    r.Register(pg_nodes.IndexStmt{}, &CreateIndexHandler{})
    r.Register(pg_nodes.CreateSchemaStmt{}, &CreateSchemaHandler{}) // ← 新增
    return r
}
```

- [ ] **步骤 9：Handler 类型安全测试**

**文件：** `internal/parser/handler_test.go`，在 `TestHandlers_TypeSafety` 中添加：

```go
t.Run("CreateSchemaHandler", func(t *testing.T) {
    h := &CreateSchemaHandler{}
    _, err := h.Handle(node)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "expected pg_nodes.CreateSchemaStmt")
})
```

- [ ] **步骤 10：运行全部 Parser 测试确认无损**

```bash
go test ./internal/parser/... -v
```
预期：全部 PASS

- [ ] **步骤 11：Commit**

```bash
git add internal/parser/create_schema_handler.go internal/parser/mutation.go internal/parser/registry.go internal/parser/handler_test.go internal/parser/mutation_test.go
git commit -m "feat(parser): add CreateSchemaStmt handler and CreateSchemaMutation"
```

---

### 任务 3：添加 `FilterNamespaces` 到 `ComputeDiff`

#### 3a：编写测试（先红后绿）

**文件：** `internal/app/diff_service_test.go`

- [ ] **步骤 1：编写 `FilterNamespaces` 单元测试**

在文件末尾添加：

```go
func TestFilterNamespaces_EmptySchemas(t *testing.T) {
    source := model.NewSchema()
    source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
    source.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}

    target := model.NewSchema()
    target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
    target.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
    target.GetOrCreateNamespace("extra").Tables["t3"] = &model.Table{Name: "t3"}

    // When schemas is nil/empty, all namespaces should be preserved
    FilterNamespaces(source, target, nil)
    require.Len(t, source.Schemas, 2)
    require.Len(t, target.Schemas, 3)

    FilterNamespaces(source, target, []string{})
    require.Len(t, source.Schemas, 2)
    require.Len(t, target.Schemas, 3)
}

func TestFilterNamespaces_FilterToSpecified(t *testing.T) {
    source := model.NewSchema()
    source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
    source.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
    source.GetOrCreateNamespace("internal").Tables["t3"] = &model.Table{Name: "t3"}

    target := model.NewSchema()
    target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
    target.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
    target.GetOrCreateNamespace("extra").Tables["t4"] = &model.Table{Name: "t4"}

    FilterNamespaces(source, target, []string{"public", "auth"})

    // Source: internal should be removed
    require.Len(t, source.Schemas, 2)
    require.NotNil(t, source.GetNamespace("public"))
    require.NotNil(t, source.GetNamespace("auth"))
    require.Nil(t, source.GetNamespace("internal"))

    // Target: extra should be removed
    require.Len(t, target.Schemas, 2)
    require.NotNil(t, target.GetNamespace("public"))
    require.NotNil(t, target.GetNamespace("auth"))
    require.Nil(t, target.GetNamespace("extra"))
}

func TestFilterNamespaces_NonExistentSchema(t *testing.T) {
    source := model.NewSchema()
    source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

    target := model.NewSchema()
    target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

    // Filtering with a schema that doesn't exist should be a no-op
    FilterNamespaces(source, target, []string{"public", "nonexistent"})
    require.Len(t, source.Schemas, 1)
    require.Len(t, target.Schemas, 1)
}

func TestFilterNamespaces_EmptyResultWarning(t *testing.T) {
    source := model.NewSchema()
    source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

    target := model.NewSchema()
    target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

    // When schemas don't match, all namespaces are filtered out
    warnings := FilterNamespaces(source, target, []string{"nonexistent"})
    require.Len(t, source.Schemas, 0)
    require.Len(t, target.Schemas, 0)
    require.Len(t, warnings, 1)
    assert.Contains(t, warnings[0], "no schemas matched")
}
```

- [ ] **步骤 2：运行测试预期失败**

```bash
go test ./internal/app/... -v -run TestFilterNamespaces
```
预期：编译错误，`FilterNamespaces` 未定义。

#### 3b：实现 FilterNamespaces

**文件：** `internal/app/diff_service.go`

- [ ] **步骤 3：添加 `FilterNamespaces` 函数**

在 `NormalizeSchemas` 函数后面添加：

```go
// FilterNamespaces filters source and target schemas to only include
// the specified schema names. If schemas is nil or empty, no filtering is performed.
// Returns warnings if no schemas match the filter.
func FilterNamespaces(source, target *model.Schema, schemas []string) []string {
    if len(schemas) == 0 {
        return nil
    }

    schemaSet := make(map[string]bool, len(schemas))
    for _, s := range schemas {
        schemaSet[s] = true
    }

    filterSchema := func(s *model.Schema) {
        for name := range s.Schemas {
            if !schemaSet[name] {
                delete(s.Schemas, name)
            }
        }
    }

    filterSchema(source)
    filterSchema(target)

    // Warn if filtering removed everything (likely a typo in --schema flag)
    if len(source.Schemas) == 0 && len(target.Schemas) == 0 {
        return []string{"no schemas matched the filter — check your --schema flag(s)"}
    }
    return nil
}
```

- [ ] **步骤 4：在 `ComputeDiff` 中调用 `FilterNamespaces`**

修改 `ComputeDiff` 函数，在 `NormalizeSchemas` 之后、`Differ.Diff` 之前添加 `FilterNamespaces`，并将过滤警告合并到 warnings 中：

```go
func ComputeDiff(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
    if err := NormalizeSchemas(source, target); err != nil {
        return nil, nil, err
    }

    // Filter to only compare specified schemas
    filterWarnings := FilterNamespaces(source, target, cfg.Schemas)

    differ := diff.NewDiffer()
    operations, warnings := differ.Diff(source, target)
    warnings = append(warnings, filterWarnings...)

    filteredOps, filterWarnings := FilterDestructiveOps(operations, cfg.UnsafeDrop)
    warnings = append(warnings, filterWarnings...)
    // ... rest unchanged
}
```

- [ ] **步骤 5：运行测试验证通过**

```bash
go test ./internal/app/... -v -run TestFilterNamespaces
```
预期：PASS

- [ ] **步骤 6：运行全部测试确认无损**

```bash
go test ./... -short
```
预期：全部 PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/app/diff_service.go internal/app/diff_service_test.go
git commit -m "feat(app): add FilterNamespaces to apply --schema filtering on file sources"
```

---

### 任务 4：添加多 Schema 集成测试

**文件：** `cmd/migra/integration_test.go`

- [ ] **步骤 1：编写多 Schema 集成测试**

在 `TestDirectoryVsFile_Diff` 之后添加：

```go
func TestMultiSchemaDiff(t *testing.T) {
    // Create source directory with multi-schema SQL files
    sourceDir := t.TempDir()

    // public schema
    publicSQL := `CREATE TABLE public.users (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100) NOT NULL
    );`
    if err := os.WriteFile(filepath.Join(sourceDir, "01_public.sql"), []byte(publicSQL), 0644); err != nil {
        t.Fatal(err)
    }

    // auth schema (with CREATE SCHEMA statement)
    authSQL := `CREATE SCHEMA auth;

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE auth.permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES auth.roles(id),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL
);`
    if err := os.WriteFile(filepath.Join(sourceDir, "02_auth.sql"), []byte(authSQL), 0644); err != nil {
        t.Fatal(err)
    }

    // Create target directory with additional tables/columns
    targetDir := t.TempDir()

    // public schema (with new profiles table and email column)
    targetPublicSQL := `CREATE TABLE public.users (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100) NOT NULL,
        email VARCHAR(255)
    );

CREATE TABLE public.profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES public.users(id),
    avatar_url TEXT,
    bio TEXT
);`
    if err := os.WriteFile(filepath.Join(targetDir, "01_public.sql"), []byte(targetPublicSQL), 0644); err != nil {
        t.Fatal(err)
    }

    // auth schema (with new description column)
    targetAuthSQL := `CREATE SCHEMA auth;

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT DEFAULT ''
);

CREATE TABLE auth.permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES auth.roles(id),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL
);`
    if err := os.WriteFile(filepath.Join(targetDir, "02_auth.sql"), []byte(targetAuthSQL), 0644); err != nil {
        t.Fatal(err)
    }

    // Run diff with --schema public --schema auth
    out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
        Source:     sourceDir,
        Target:     targetDir,
        Schemas:    []string{"public", "auth"},
        Format:     "sql",
        Timeout:    defaultDiffTimeout,
        UnsafeDrop: false,
    })
    if err != nil {
        t.Fatalf("multi-schema diff failed: %v", err)
    }

    // Verify expected changes
    for _, want := range []string{
        `CREATE TABLE "public"."profiles"`,
        `ALTER TABLE "public"."users" ADD COLUMN "email"`,
        `ALTER TABLE "auth"."roles" ADD COLUMN "description"`,
    } {
        if !strings.Contains(out, want) {
            t.Fatalf("expected output to contain %q, got:\n%s", want, out)
        }
    }

    // Should NOT have parse errors
    if strings.Contains(out, "parse error") {
        t.Fatalf("unexpected parse error in output:\n%s", out)
    }
}
```

- [ ] **步骤 2：运行测试验证通过**

```bash
go test ./cmd/migra/... -v -run TestMultiSchemaDiff
```
预期：PASS

- [ ] **步骤 3：运行全部测试确认无损**

```bash
go test ./... -short
```
预期：全部 PASS

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/integration_test.go
git commit -m "test: add multi-schema integration test with CREATE SCHEMA statements"
```

---

### 任务 5：更新 Bug 报告文档

**文件：** `docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md`

- [ ] **步骤 1：更新状态和添加修复记录**

将 `| **状态** | Open |` 改为 `| **状态** | Fixed |`

在文件末尾 `验证方法` 章节之后（或 `相关代码文件` 之后）添加修复记录：

```markdown
---

## 修复记录

### 修复时间

2026-05-15

### 修改文件

| 文件 | 变更说明 |
|------|----------|
| [`internal/parser/create_schema_handler.go`](../internal/parser/create_schema_handler.go) | **新增**：`CreateSchemaHandler` 处理 `CreateSchemaStmt` AST 节点；`CreateSchemaMutation` 确保 namespace 存在 |
| [`internal/parser/registry.go`](../internal/parser/registry.go) | `DefaultRegistry` 中注册 `CreateSchemaStmt` → `CreateSchemaHandler` |
| [`internal/parser/mutation.go`](../internal/parser/mutation.go) | 新增 `MutKindCreateSchema` 常量 |
| [`internal/model/object_key.go`](../internal/model/object_key.go) | 新增 `KindSchema` 常量 |
| [`internal/app/diff_service.go`](../internal/app/diff_service.go) | 新增 `FilterNamespaces` 函数；`ComputeDiff` 中于 Normalize 后、Diff 前调用 |
| [`internal/parser/handler_test.go`](../internal/parser/handler_test.go) | 新增 `TestCreateSchemaHandler` + 类型安全检查 |
| [`internal/parser/mutation_test.go`](../internal/parser/mutation_test.go) | 新增 `TestCreateSchemaMutation_Apply` + `_Idempotent` |
| [`internal/app/diff_service_test.go`](../internal/app/diff_service_test.go) | 新增 `TestFilterNamespaces_*` |
| [`cmd/migra/integration_test.go`](../cmd/migra/integration_test.go) | 新增 `TestMultiSchemaDiff` |

### 修复要点

1. **Parser 支持 `CREATE SCHEMA`**：新增 `CreateSchemaHandler`，从 AST 提取 `Schemaname`，返回 `CreateSchemaMutation` 确保 namespace 在 model.Schema 中存在
2. **`--schema` 过滤对文件源生效**：在 `ComputeDiff` 的 Normalize 之后、Diff 之前，调用 `FilterNamespaces` 移除不在 `--schema` 列表中的 namespace。`schemas` 为空时不过滤，保持向后兼容
```

- [ ] **步骤 2：Commit**

```bash
git add docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md
git commit -m("docs: mark multi-schema diff bug as fixed with resolution notes")
```

---

### 任务 6：最终验证

- [ ] **步骤 1：完整 CI 检查**

```bash
make ci
```
预期：fmt → vet → lint → 全部测试通过

- [ ] **步骤 2：手动 QA — 多 Schema diff**

```bash
make build
./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/
```

预期：
```sql
-- op: add_table risk:low
CREATE TABLE "public"."profiles" (...);

-- op: add_column risk:low
ALTER TABLE "auth"."roles" ADD COLUMN "description" text DEFAULT '';

-- op: add_column risk:low
ALTER TABLE "public"."users" ADD COLUMN "email" varchar(255);
```

验证点：
- ✅ 无解析错误
- ✅ 包含 `profiles` 表、`description` 列、`email` 列
- ✅ 无 DROP 操作（safe mode）
- ✅ `--schema public` 只比较 public schema（仅 `profiles` + `users.email`）
- ✅ `--schema auth` 包括 auth schema（`roles.description`）

- [ ] **步骤 3：JSON 格式验证**

```bash
./migra diff --format json --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/
```
预期：合法的 JSON 数组格式，包含 `kind`、`object_key`、`sql` 字段。

- [ ] **步骤 4：`--schema` 单一值验证**

```bash
./migra diff --schema public testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/
```
预期：只输出 public schema 的变更（profiles + users.email），不输出 auth 的变更。
