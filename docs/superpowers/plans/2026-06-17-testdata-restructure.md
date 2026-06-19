# Testdata 目录重构实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 重构 testdata/ 目录结构，按复杂度分组，统一命名规范，改善可维护性

**架构：** 创建 complex/ 目录存放复杂场景测试数据，迁移版本演进序列、多 schema、重命名、IDENTITY、COLLATE、非表对象和边界条件场景，更新测试用例路径引用和文档

**技术栈：** Go, bash, git

---

## 文件结构

**创建的文件：**
- `testdata/complex/README.md` - 复杂场景测试数据说明文档
- `testdata/complex/evolution/v1/01_users.sql` - 版本演进 v1 用户表
- `testdata/complex/evolution/v1/02_posts.sql` - 版本演进 v1 文章表
- `testdata/complex/evolution/v1/03_indexes.sql` - 版本演进 v1 索引
- `testdata/complex/evolution/v1/04_enums.sql` - 版本演进 v1 枚举类型
- `testdata/complex/evolution/v2/schema.sql` - 版本演进 v2 合并快照
- `testdata/complex/evolution/v3/schema.sql` - 版本演进 v3 合并快照
- `testdata/complex/evolution/v3/README.md` - 版本演进 v3 变更说明
- `testdata/complex/evolution/v4/schema.sql` - 版本演进 v4 合并快照
- `testdata/complex/evolution/v4/README.md` - 版本演进 v4 变更说明
- `testdata/complex/evolution/snapshot.sql` - v2 快照
- `testdata/complex/multi-schema/v1/01_public.sql` - 多 schema v1 public
- `testdata/complex/multi-schema/v1/02_auth.sql` - 多 schema v1 auth
- `testdata/complex/multi-schema/v2/01_public.sql` - 多 schema v2 public
- `testdata/complex/multi-schema/v2/02_auth.sql` - 多 schema v2 auth
- `testdata/complex/multi-schema/v2/snapshot.sql` - 多 schema v2 快照
- `testdata/complex/rename/basic/v1/schema.sql` - 基础重命名 v1
- `testdata/complex/rename/basic/v2/schema.sql` - 基础重命名 v2
- `testdata/complex/rename/complex/v1/schema.sql` - 复杂重命名 v1
- `testdata/complex/rename/complex/v2/schema.sql` - 复杂重命名 v2
- `testdata/complex/identity/v1/schema.sql` - IDENTITY v1
- `testdata/complex/identity/v2/schema.sql` - IDENTITY v2
- `testdata/complex/collate/v1/schema.sql` - COLLATE v1
- `testdata/complex/collate/v2/schema.sql` - COLLATE v2
- `testdata/complex/objects/v1/schema.sql` - 非表对象 v1
- `testdata/complex/objects/v2/schema.sql` - 非表对象 v2
- `testdata/complex/edge-cases/nested/01_tables/users.sql` - 嵌套目录用户表
- `testdata/complex/edge-cases/nested/02_tables/posts.sql` - 嵌套目录文章表
- `testdata/complex/edge-cases/nested-target/01_tables/users.sql` - 嵌套目录目标用户表
- `testdata/complex/edge-cases/nested-target/02_tables/posts.sql` - 嵌套目录目标文章表
- `testdata/complex/edge-cases/directory/tables/users.sql` - DirectoryLoader 用户表
- `testdata/complex/edge-cases/directory/.hidden.sql` - 隐藏文件
- `testdata/complex/edge-cases/directory/.hidden_dir/secret.sql` - 隐藏目录
- `testdata/complex/edge-cases/directory/empty.sql` - 空 SQL 文件
- `testdata/complex/edge-cases/directory/encoding_utf8.sql` - UTF-8 编码文件
- `testdata/complex/edge-cases/directory/large/generated_schema.sql` - 大目录文件
- `testdata/complex/edge-cases/directory/no_extension_file` - 无后缀文件
- `testdata/complex/edge-cases/directory/readme.txt` - 非 SQL 文件

**修改的文件：**
- `cmd/migra/integration_test.go` - 更新测试用例路径引用
- `testdata/README.md` - 更新文档

**删除的文件：**
- `testdata/diff/` - 整个目录

---

## 任务 1：创建目录结构

**文件：**
- 创建：`testdata/complex/` 目录结构

- [ ] **步骤 1：创建 complex/ 目录结构**

```bash
mkdir -p testdata/complex/{evolution,multi-schema,rename/{basic,complex},identity,collate,objects,edge-cases/{nested,nested-target,directory}}
```

- [ ] **步骤 2：验证目录结构**

运行：`tree testdata/complex/`
预期：显示完整的目录结构

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/
git commit -m "test: create complex/ directory structure for testdata restructure"
```

---

## 任务 2：复制版本演进序列

**文件：**
- 复制：`testdata/diff/v1/` → `testdata/complex/evolution/v1/`
- 复制：`testdata/diff/v2/` → `testdata/complex/evolution/v2/`
- 复制：`testdata/diff/v3/` → `testdata/complex/evolution/v3/`
- 复制：`testdata/diff/v4/` → `testdata/complex/evolution/v4/`
- 复制：`testdata/diff/snapshot.sql` → `testdata/complex/evolution/snapshot.sql`

- [ ] **步骤 1：复制版本演进序列**

```bash
cp -r testdata/diff/v1/ testdata/complex/evolution/v1/
cp -r testdata/diff/v2/ testdata/complex/evolution/v2/
cp -r testdata/diff/v3/ testdata/complex/evolution/v3/
cp -r testdata/diff/v4/ testdata/complex/evolution/v4/
cp testdata/diff/snapshot.sql testdata/complex/evolution/
```

- [ ] **步骤 2：验证复制**

运行：`ls -la testdata/complex/evolution/`
预期：显示 v1/, v2/, v3/, v4/, snapshot.sql

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/evolution/
git commit -m "test: copy version evolution sequence to complex/evolution/"
```

---

## 任务 3：复制多 schema 场景

**文件：**
- 复制：`testdata/diff/multi_schema/` → `testdata/complex/multi-schema/`

- [ ] **步骤 1：复制多 schema 场景**

```bash
cp -r testdata/diff/multi_schema/ testdata/complex/multi-schema/
```

- [ ] **步骤 2：验证复制**

运行：`ls -la testdata/complex/multi-schema/`
预期：显示 v1/, v2/

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/multi-schema/
git commit -m "test: copy multi-schema scenario to complex/multi-schema/"
```

---

## 任务 4：复制重命名场景

**文件：**
- 复制：`testdata/diff/rename_example/` → `testdata/complex/rename/basic/`
- 复制：`testdata/diff/rename_complex/` → `testdata/complex/rename/complex/`

- [ ] **步骤 1：复制重命名场景**

```bash
cp -r testdata/diff/rename_example/ testdata/complex/rename/basic/
cp -r testdata/diff/rename_complex/ testdata/complex/rename/complex/
```

- [ ] **步骤 2：验证复制**

运行：`ls -la testdata/complex/rename/`
预期：显示 basic/, complex/

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/rename/
git commit -m "test: copy rename scenarios to complex/rename/"
```

---

## 任务 5：复制其他复杂场景

**文件：**
- 复制：`testdata/diff/identity_example/` → `testdata/complex/identity/`
- 复制：`testdata/diff/collate_example/` → `testdata/complex/collate/`
- 复制：`testdata/diff/objects_example/` → `testdata/complex/objects/`

- [ ] **步骤 1：复制其他复杂场景**

```bash
cp -r testdata/diff/identity_example/ testdata/complex/identity/
cp -r testdata/diff/collate_example/ testdata/complex/collate/
cp -r testdata/diff/objects_example/ testdata/complex/objects/
```

- [ ] **步骤 2：验证复制**

运行：`ls -la testdata/complex/`
预期：显示 identity/, collate/, objects/

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/identity/ testdata/complex/collate/ testdata/complex/objects/
git commit -m "test: copy identity, collate, and objects scenarios to complex/"
```

---

## 任务 6：复制边界场景

**文件：**
- 复制：`testdata/diff/nested/` → `testdata/complex/edge-cases/nested/`
- 复制：`testdata/diff/nested_target/` → `testdata/complex/edge-cases/nested-target/`
- 复制：`testdata/diff/edge/` → `testdata/complex/edge-cases/directory/`

- [ ] **步骤 1：复制边界场景**

```bash
cp -r testdata/diff/nested/ testdata/complex/edge-cases/nested/
cp -r testdata/diff/nested_target/ testdata/complex/edge-cases/nested-target/
cp -r testdata/diff/edge/ testdata/complex/edge-cases/directory/
```

- [ ] **步骤 2：验证复制**

运行：`ls -la testdata/complex/edge-cases/`
预期：显示 nested/, nested-target/, directory/

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/edge-cases/
git commit -m "test: copy edge cases to complex/edge-cases/"
```

---

## 任务 7：创建 complex/README.md

**文件：**
- 创建：`testdata/complex/README.md`

- [ ] **步骤 1：创建 complex/README.md**

```markdown
# 复杂场景测试数据

本目录包含 migra-go 项目的复杂场景测试数据，按功能模块分组。

## 目录结构

```
complex/
├── README.md                    ← 本文件
├── evolution/                   ← 版本演进序列（v1→v2→v3→v4）
├── multi-schema/                ← 多 schema 场景
├── rename/                      ← 重命名场景
│   ├── basic/                   ←   基础重命名
│   └── complex/                 ←   复杂重命名（组合变更）
├── identity/                    ← IDENTITY 列场景
├── collate/                     ← COLLATE 场景
├── objects/                     ← 非表对象场景
└── edge-cases/                  ← 边界条件场景
    ├── nested/                  ←   嵌套目录
    ├── nested-target/           ←   嵌套目录目标
    └── directory/               ←   DirectoryLoader 边界情况
```

## 测试数据说明

### 版本演进序列

v1→v2→v3→v4 构成一个四阶段演进序列：

- **v1 (基线)**：users + posts + 索引 + 枚举
- **v2 (新增)**：v1 + age 列 + comments 表 + guest 枚举
- **v3 (删除/修改)**：v2 - age 列/comments 表；+ phone 列/categories 表/UNIQUE(email)
- **v4 (高级特性)**：v3 + IDENTITY + COLLATE + FK CASCADE

### 多 Schema 场景

测试多个 schema 的复杂场景，包括跨 schema 外键引用。

### 重命名场景

测试 RENAME COLUMN 的各种场景：
- **basic**：基础重命名
- **complex**：重命名 + 删列 + 新增组合

### IDENTITY 场景

测试 SERIAL → IDENTITY 变更检测。

### COLLATE 场景

测试 COLLATE 子句新增检测。

### 非表对象场景

测试 EXTENSION + SEQUENCE + VIEW 差异检测。

### 边界条件场景

测试 DirectoryLoader 的边界情况：
- **nested/**：嵌套子目录结构
- **nested-target/**：嵌套目录的目标版本
- **directory/**：隐藏文件、空目录、非 SQL 文件、大目录等

## 如何运行测试

```bash
# 运行所有测试
make test

# 运行复杂场景测试
go test ./cmd/migra/... -v -run TestComplex

# 运行版本演进测试
go test ./cmd/migra/... -v -run TestV3DirectoryDiff
```
```

- [ ] **步骤 2：验证文件**

运行：`cat testdata/complex/README.md`
预期：显示完整的 README 内容

- [ ] **步骤 3：Commit**

```bash
git add testdata/complex/README.md
git commit -m "test: add complex/README.md documentation"
```

---

## 任务 8：更新测试用例路径

**文件：**
- 修改：`cmd/migra/integration_test.go`

- [ ] **步骤 1：更新 TestV3DirectoryDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/v1/` → `../../testdata/complex/evolution/v1/`
- `../../testdata/diff/v2/` → `../../testdata/complex/evolution/v2/`

```go
// 更新前
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/diff/v1/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/diff/v2/",
},

// 更新后
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/complex/evolution/v1/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/complex/evolution/v2/",
},
```

- [ ] **步骤 2：更新 TestV3UnsafeDropDiff_Safe 和 TestV3UnsafeDropDiff_Unsafe 测试用例**

找到并替换以下路径：
- `../../testdata/diff/v2/schema.sql` → `../../testdata/complex/evolution/v2/schema.sql`
- `../../testdata/diff/v3/schema.sql` → `../../testdata/complex/evolution/v3/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/v2/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/v3/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/evolution/v2/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/evolution/v3/schema.sql",
},
```

- [ ] **步骤 3：更新 TestV3ToV4Diff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/v3/schema.sql` → `../../testdata/complex/evolution/v3/schema.sql`
- `../../testdata/diff/v4/schema.sql` → `../../testdata/complex/evolution/v4/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/v3/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/v4/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/evolution/v3/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/evolution/v4/schema.sql",
},
```

- [ ] **步骤 4：更新 TestNestedDirectoryDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/nested/` → `../../testdata/complex/edge-cases/nested/`
- `../../testdata/diff/nested_target/` → `../../testdata/complex/edge-cases/nested-target/`

```go
// 更新前
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/diff/nested/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/diff/nested_target/",
},

// 更新后
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/complex/edge-cases/nested/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/complex/edge-cases/nested-target/",
},
```

- [ ] **步骤 5：更新 TestMultiSchemaDirectoryDiffStatic 测试用例**

找到并替换以下路径：
- `../../testdata/diff/multi_schema/v1/` → `../../testdata/complex/multi-schema/v1/`
- `../../testdata/diff/multi_schema/v2/` → `../../testdata/complex/multi-schema/v2/`

```go
// 更新前
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/diff/multi_schema/v1/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/diff/multi_schema/v2/",
},

// 更新后
Source: app.SourceConfig{
    Type: "directory",
    Source: "../../testdata/complex/multi-schema/v1/",
},
Target: app.TargetConfig{
    Type: "directory",
    Source: "../../testdata/complex/multi-schema/v2/",
},
```

- [ ] **步骤 6：更新 TestRenameColumnDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/rename_example/v1/schema.sql` → `../../testdata/complex/rename/basic/v1/schema.sql`
- `../../testdata/diff/rename_example/v2/schema.sql` → `../../testdata/complex/rename/basic/v2/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/rename_example/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/rename_example/v2/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/rename/basic/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/rename/basic/v2/schema.sql",
},
```

- [ ] **步骤 7：更新 TestRenameColumnComplexDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/rename_complex/v1/schema.sql` → `../../testdata/complex/rename/complex/v1/schema.sql`
- `../../testdata/diff/rename_complex/v2/schema.sql` → `../../testdata/complex/rename/complex/v2/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/rename_complex/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/rename_complex/v2/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/rename/complex/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/rename/complex/v2/schema.sql",
},
```

- [ ] **步骤 8：更新 TestIdentityColumnDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/identity_example/v1/schema.sql` → `../../testdata/complex/identity/v1/schema.sql`
- `../../testdata/diff/identity_example/v2/schema.sql` → `../../testdata/complex/identity/v2/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/identity_example/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/identity_example/v2/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/identity/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/identity/v2/schema.sql",
},
```

- [ ] **步骤 9：更新 TestCollateClauseDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/collate_example/v1/schema.sql` → `../../testdata/complex/collate/v1/schema.sql`
- `../../testdata/diff/collate_example/v2/schema.sql` → `../../testdata/complex/collate/v2/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/collate_example/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/collate_example/v2/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/collate/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/collate/v2/schema.sql",
},
```

- [ ] **步骤 10：更新 TestObjectsDiff 测试用例**

找到并替换以下路径：
- `../../testdata/diff/objects_example/v1/schema.sql` → `../../testdata/complex/objects/v1/schema.sql`
- `../../testdata/diff/objects_example/v2/schema.sql` → `../../testdata/complex/objects/v2/schema.sql`

```go
// 更新前
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/diff/objects_example/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/diff/objects_example/v2/schema.sql",
},

// 更新后
Source: app.SourceConfig{
    Type: "file",
    Source: "../../testdata/complex/objects/v1/schema.sql",
},
Target: app.TargetConfig{
    Type: "file",
    Source: "../../testdata/complex/objects/v2/schema.sql",
},
```

- [ ] **步骤 11：验证测试用例路径**

运行：`grep -n "testdata/diff/" cmd/migra/integration_test.go`
预期：没有结果（所有路径已更新）

- [ ] **步骤 12：Commit**

```bash
git add cmd/migra/integration_test.go
git commit -m "test: update testdata paths in integration tests"
```

---

## 任务 9：更新测试数据文档

**文件：**
- 修改：`testdata/README.md`

- [ ] **步骤 1：更新目录结构说明**

找到并替换目录结构部分：

```markdown
## 目录结构

```
testdata/
├── README.md                     ← 本文件
├── example_source.sql            ← 基础 diff 测试：源（2 表 + 索引 + 枚举）
├── example_target.sql            ← 基础 diff 测试：目标（增列 + 增表 + 枚举标签）
├── alter_operations.sql          ← 12 种 ALTER TABLE 变体（ADD/DROP/SET/ALTER COLUMN TYPE）
├── complex_ddl.sql               ← 复合约束、高级类型、自定义枚举
├── drop_scenarios.sql            ← DROP 语义测试（残留 schema 状态）
├── edge_cases.sql                ← 边界 SQL 模式（引号标识符、继承表、分区表）
│
└── complex/                      ← 复杂场景测试数据（按功能模块分组）
    ├── README.md                 ←   复杂场景说明文档
    ├── evolution/                ←   版本演进序列（v1→v2→v3→v4）
    ├── multi-schema/             ←   多 schema 场景
    ├── rename/                   ←   重命名场景
    │   ├── basic/                ←     基础重命名
    │   └── complex/              ←     复杂重命名（组合变更）
    ├── identity/                 ←   IDENTITY 列场景
    ├── collate/                  ←   COLLATE 场景
    ├── objects/                  ←   非表对象场景
    └── edge-cases/               ←   边界条件场景
        ├── nested/               ←     嵌套目录
        ├── nested-target/        ←     嵌套目录目标
        └── directory/            ←     DirectoryLoader 边界情况
```
```

- [ ] **步骤 2：更新测试数据映射表**

找到并替换测试数据映射表部分：

```markdown
### 测试数据 vs 测试用例映射

| 测试数据文件 | 被哪些测试引用 | 测试场景 |
|---|---|---|
| `example_source.sql` + `example_target.sql` | `internal/app/diff_service_test.go:TestDiffService_Run`<br>`cmd/migra/integration_test.go:TestExampleSQLFilesDiffIncludesEnumAndConstraints` | Mock 编排 + 全流水线 diff：增列、增表、枚举加标签 |
| `complex/evolution/v1/` → `complex/evolution/v2/` | `cmd/migra/integration_test.go:TestV3DirectoryDiff` | 目录 vs 目录 diff（多文件拆分 vs 单文件） |
| `complex/evolution/v3/schema.sql` | `cmd/migra/integration_test.go:TestV3UnsafeDropDiff_Safe`<br>`cmd/migra/integration_test.go:TestV3UnsafeDropDiff_Unsafe` | 删除/修改场景：删列、删表、改索引名、改枚举 |
| `complex/edge-cases/nested/` → `complex/edge-cases/nested-target/` | `cmd/migra/integration_test.go:TestNestedDirectoryDiff` | 嵌套子目录 diff |
| `complex/multi-schema/v1/` → `complex/multi-schema/v2/` | `cmd/migra/integration_test.go:TestMultiSchemaDirectoryDiffStatic` | 多 schema（public + auth）diff |
| `complex/rename/basic/v1/` → `complex/rename/basic/v2/` | `cmd/migra/integration_test.go:TestRenameColumnDiff` | RENAME COLUMN 基础场景 |
| `complex/rename/complex/v1/` → `complex/rename/complex/v2/` | `cmd/migra/integration_test.go:TestRenameColumnComplexDiff` | RENAME COLUMN + 删列 + 新增组合 |
| `complex/identity/v1/` → `complex/identity/v2/` | `cmd/migra/integration_test.go:TestIdentityColumnDiff` | IDENTITY 列检测专用 |
| `complex/collate/v1/` → `complex/collate/v2/` | `cmd/migra/integration_test.go:TestCollateClauseDiff` | COLLATE 子句检测专用 |
| `complex/objects/v1/` → `complex/objects/v2/` | `cmd/migra/integration_test.go:TestObjectsDiff` | VIEW/SEQUENCE/EXTENSION 检测专用 |
| `complex/evolution/v3/schema.sql` → `complex/evolution/v4/schema.sql` | `cmd/migra/integration_test.go:TestV3ToV4Diff` | v3→v4 演进：IDENTITY + COLLATE + FK CASCADE |
| `complex/edge-cases/directory/` | `internal/source/dir_loader_test.go`（部分用例用 `t.TempDir()` 动态创建） | DirectoryLoader 边界条件 |
| `alter_operations.sql` | `internal/parser/handler_test.go:TestParseAlterOperations` | ALTER TABLE 全操作集解析 |
| `complex_ddl.sql` | `internal/parser/handler_test.go:TestParseComplexDDL` | 复合约束/高级类型解析 |
| `drop_scenarios.sql` | `internal/parser/handler_test.go:TestParseDropScenarios` | DROP 语义残留状态 |
| `edge_cases.sql` | `internal/parser/handler_test.go:TestParseEdgeCases` | 边界 SQL 模式 |
| `internal/model/testdata/schema_golden.json` | `internal/model/golden_test.go:TestGoldenSchema` | Schema JSON 序列化稳定性 |
```

- [ ] **步骤 3：更新版本演进图谱**

找到并替换版本演进图谱部分：

```markdown
### 版本演进图谱

`complex/evolution/v1/` → `complex/evolution/v2/` → `complex/evolution/v3/` → `complex/evolution/v4/` 构成一个四阶段演进序列：
```

- [ ] **步骤 4：更新验证命令速查表**

找到并替换验证命令速查表部分：

```markdown
### 验证命令速查表

```bash
# 构建
make build

# 基本 diff
./migra diff testdata/example_source.sql testdata/example_target.sql

# JSON 格式
./migra diff --format json testdata/example_source.sql testdata/example_target.sql

# 目录 diff
./migra diff testdata/complex/evolution/v1/ testdata/complex/evolution/v2/

# 目录 vs 快照
./migra diff testdata/complex/evolution/v2/ testdata/complex/evolution/snapshot.sql

# 删除场景（安全模式）
./migra diff testdata/complex/evolution/v2/schema.sql testdata/complex/evolution/v3/schema.sql

# 删除场景（含 DROP）
./migra diff --unsafe-drop testdata/complex/evolution/v2/schema.sql testdata/complex/evolution/v3/schema.sql

# 嵌套目录
./migra diff testdata/complex/edge-cases/nested/ testdata/complex/edge-cases/nested-target/

# 多 schema
./migra diff --schema public --schema auth testdata/complex/multi-schema/v1/ testdata/complex/multi-schema/v2/

# RENAME COLUMN 基础
./migra diff testdata/complex/rename/basic/v1/schema.sql testdata/complex/rename/basic/v2/schema.sql

# RENAME COLUMN 组合变更
./migra diff testdata/complex/rename/complex/v1/schema.sql testdata/complex/rename/complex/v2/schema.sql

# 输出到文件
./migra diff -o /tmp/result.sql testdata/example_source.sql testdata/example_target.sql

# 版本信息
./migra --version

# 帮助
./migra diff --help
```
```

- [ ] **步骤 5：验证文档**

运行：`grep -n "testdata/diff/" testdata/README.md`
预期：没有结果（所有路径已更新）

- [ ] **步骤 6：Commit**

```bash
git add testdata/README.md
git commit -m "docs: update testdata/README.md with new directory structure"
```

---

## 任务 10：删除旧目录

**文件：**
- 删除：`testdata/diff/`

- [ ] **步骤 1：验证所有路径已更新**

运行：`grep -r "testdata/diff/" . --include="*.go" --include="*.md"`
预期：没有结果（所有路径已更新）

- [ ] **步骤 2：删除旧目录**

```bash
rm -rf testdata/diff/
```

- [ ] **步骤 3：验证删除**

运行：`ls testdata/`
预期：不显示 diff/ 目录

- [ ] **步骤 4：Commit**

```bash
git add -A testdata/
git commit -m "test: remove old testdata/diff/ directory"
```

---

## 任务 11：运行测试验证

**文件：**
- 无

- [ ] **步骤 1：运行所有测试**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 2：运行复杂场景测试**

运行：`go test ./cmd/migra/... -v -run TestV3DirectoryDiff`
预期：测试通过

- [ ] **步骤 3：运行版本演进测试**

运行：`go test ./cmd/migra/... -v -run TestV3ToV4Diff`
预期：测试通过

- [ ] **步骤 4：运行嵌套目录测试**

运行：`go test ./cmd/migra/... -v -run TestNestedDirectoryDiff`
预期：测试通过

- [ ] **步骤 5：运行多 schema 测试**

运行：`go test ./cmd/migra/... -v -run TestMultiSchemaDirectoryDiffStatic`
预期：测试通过

- [ ] **步骤 6：运行重命名测试**

运行：`go test ./cmd/migra/... -v -run TestRenameColumnDiff`
预期：测试通过

- [ ] **步骤 7：运行 IDENTITY 测试**

运行：`go test ./cmd/migra/... -v -run TestIdentityColumnDiff`
预期：测试通过

- [ ] **步骤 8：运行 COLLATE 测试**

运行：`go test ./cmd/migra/... -v -run TestCollateClauseDiff`
预期：测试通过

- [ ] **步骤 9：运行非表对象测试**

运行：`go test ./cmd/migra/... -v -run TestObjectsDiff`
预期：测试通过

- [ ] **步骤 10：Commit**

```bash
git add -A
git commit -m "test: verify all tests pass after testdata restructure"
```

---

## 任务 12：最终验证

**文件：**
- 无

- [ ] **步骤 1：验证目录结构**

运行：`tree testdata/`
预期：显示新的目录结构

- [ ] **步骤 2：验证文件完整性**

运行：`find testdata/complex/ -name "*.sql" | wc -l`
预期：显示 SQL 文件数量

- [ ] **步骤 3：验证文档**

运行：`cat testdata/complex/README.md`
预期：显示完整的 README 内容

- [ ] **步骤 4：验证测试用例路径**

运行：`grep -r "testdata/diff/" . --include="*.go" --include="*.md"`
预期：没有结果

- [ ] **步骤 5：最终 Commit**

```bash
git add -A
git commit -m "test: complete testdata directory restructure"
```
