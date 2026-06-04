# Bug 报告：场景 7 多 Schema diff — `snapshot.sql` 与分文件合并导致重复表定义

## 元信息

| 字段 | 值 |
|---|---|
| **发现日期** | 2026-05-15 |
| **严重程度** | 🔴 高（功能完全不可用） |
| **影响范围** | 所有在目录中同时包含分文件 DDL 和合并快照文件的 diff 场景 |
| **触发命令** | `./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/` |
| **前置修复** | 依赖 `docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md` 的修复（`CreateSchemaStmt` handler） |
| **状态** | 已修复 |

---

## 错误现象

```
Error: failed to load target: failed to parse directory contents: parsing completed with 4 errors, first: parse error at position 601: applied 1 mutations with 1 errors, first: mutation create_table on public.users: table public.users already exists
```

命令以非零状态码退出，未产生任何 diff 输出。

---

## 复现步骤

1. 确保已应用 `CreateSchemaStmt` handler 修复（否则错误会被更早的 `unsupported statement type` 拦截）
2. 构建二进制：`make build`
3. 执行多 schema diff：
   ```bash
   ./migra diff --schema public --schema auth \
     testdata/diff/multi_schema/v1/ \
     testdata/diff/multi_schema/v2/
   ```
4. 观察错误：加载 target (v2) 目录时失败。

**复现条件**：任何通过 `DirectoryLoader` 加载的目录，如果同时包含分文件 DDL（如 `01_public.sql`、`02_auth.sql`）和合并快照文件（如 `snapshot.sql`），且两者定义了相同的表，都会触发此错误。

---

## 根因分析

### 直接原因

`testdata/diff/multi_schema/v2/` 目录包含 **3 个 `.sql` 文件**：

```
testdata/diff/multi_schema/v2/
├── 01_public.sql      ← 定义 public.users + public.profiles
├── 02_auth.sql        ← 定义 auth.roles + auth.permissions
└── snapshot.sql       ← 合并快照，包含全部 4 张表（含 public.users）
```

`DirectoryLoader.Load()` 使用 `filepath.WalkDir` 递归扫描目录中**所有** `.sql` 文件（第 59 行），按字典序排序后合并为一个字符串，然后一次性解析。

文件排序后顺序为：
1. `01_public.sql`
2. `02_auth.sql`
3. `snapshot.sql`

合并后的 SQL 字符串中，`public.users` 表被定义了 **两次**：
- 第一次来自 `01_public.sql` 中的 `CREATE TABLE public.users (...)`
- 第二次来自 `snapshot.sql` 中的 `CREATE TABLE public.users (...)`

当 Parser 处理第二次 `CREATE TABLE public.users` 时，`CreateTableMutation.Apply()` 检测到表已存在且不是占位表，返回错误：

```go
// internal/parser/mutation.go:62-64
if exists {
    if !table.IsPlaceholder {
        return fmt.Errorf("table %s.%s already exists", m.Schema, m.Name)
    }
}
```

### 调用链追踪

```
migra diff --schema public --schema auth v1/ v2/
  │
  ├─ diffService.Run()
  │    ├─ loadSchema("v1/", schemas=["public","auth"])  ← 成功（v1 只有 2 个文件，无重复）
  │    │    └─ DirectoryLoader.Load("v1/")
  │    │         ├─ WalkDir → ["01_public.sql", "02_auth.sql"]
  │    │         ├─ 合并 SQL → "CREATE TABLE public.users (...)\nCREATE SCHEMA auth;\n..."
  │    │         └─ parser.ParseSQL(combinedSQL) → ✅ 成功
  │    │
  │    └─ loadSchema("v2/", schemas=["public","auth"])  ← 失败
  │         └─ DirectoryLoader.Load("v2/")
  │              ├─ WalkDir → ["01_public.sql", "02_auth.sql", "snapshot.sql"]
  │              ├─ 合并 SQL → "CREATE TABLE public.users (...)\nCREATE SCHEMA auth;\n"
  │              │              "CREATE TABLE auth.roles (...)\n...\n"
  │              │              "CREATE TABLE public.users (...)\n..."  ← 重复！
  │              └─ parser.ParseSQL(combinedSQL)
  │                   ├─ pg.Parse(combinedSQL) → AST（包含重复的 CreateStmt）
  │                   ├─ visitNode(CreateStmt for public.users) → ✅ 第一次成功
  │                   ├─ visitNode(CreateStmt for public.users) → ❌ 第二次失败
  │                   │    └─ CreateTableMutation.Apply() → "table public.users already exists"
  │                   └─ ParseSQL 返回 error
  │              → "failed to parse directory contents: parsing completed with 4 errors, ..."
  │         → "failed to load target: ..."
  │    → 返回 error → 命令失败
```

### 为什么 position 是 601？

合并后的 SQL 字符串中，`snapshot.sql` 的内容追加在 `01_public.sql` + `02_auth.sql` 之后。`snapshot.sql` 中 `CREATE TABLE public.users` 的起始位置大约在合并字符串的第 601 个字符处，与错误信息 `parse error at position 601` 吻合。

### 为什么 v1 不报错？

`testdata/diff/multi_schema/v1/` 目录只包含 2 个文件：

```
testdata/diff/multi_schema/v1/
├── 01_public.sql
└── 02_auth.sql
```

**没有 `snapshot.sql`**，因此不存在重复定义。v1 加载成功。

### 深层原因：`snapshot.sql` 的设计目的与 DirectoryLoader 的扫描策略冲突

`snapshot.sql` 是作为 v2 的**合并快照**设计的，用于"目录 vs 单文件"的等价性测试（见 `testdata/README.md` 第 288 行设计原则："与对应目录内容等效"）。

但 `DirectoryLoader` 的扫描策略是**无差别地收录目录中所有 `.sql` 文件**，没有机制识别或排除合并快照文件。这导致：

- **用户视角**：`snapshot.sql` 是 v2 的合并快照，与分文件等价，不应该同时被加载
- **代码视角**：`DirectoryLoader` 看到 3 个 `.sql` 文件，全部合并，导致重复定义

### 受影响的数据文件

| 目录 | 包含 `snapshot.sql` | 分文件 | 重复风险 |
|---|---|---|---|
| `testdata/diff/multi_schema/v1/` | ❌ | `01_public.sql`, `02_auth.sql` | 无 |
| `testdata/diff/multi_schema/v2/` | ✅ | `01_public.sql`, `02_auth.sql` | **有** |
| `testdata/diff/v2/` | ❌ | `schema.sql`（单文件） | 无 |
| `testdata/diff/v1/` | ❌ | `01_users.sql`, `02_posts.sql`, `03_indexes.sql`, `04_enums.sql` | 无 |

---

## 问题分类

这是一个**测试数据组织问题**，但暴露了 DirectoryLoader 的设计缺陷：

### 问题 1：测试数据目录中包含冲突的合并快照（直接原因）

- **位置**：`testdata/diff/multi_schema/v2/snapshot.sql`
- **性质**：测试数据设计缺陷 — 合并快照与分文件共存于同一目录
- **影响**：DirectoryLoader 加载时重复定义表

### 问题 2：DirectoryLoader 无机制排除合并快照（设计缺陷）

- **位置**：`internal/source/dir_loader.go:48-63`（`WalkDir` 扫描逻辑）
- **性质**：DirectoryLoader 无差别扫描所有 `.sql` 文件，没有识别"合并快照"的语义
- **影响**：如果用户意外将合并快照放在源目录中，会导致不可预期的重复定义错误

---

## 修复方向（不实施，仅记录）

### 方案 A：移除冲突的 `snapshot.sql`（推荐，最简单）

将 `testdata/diff/multi_schema/v2/snapshot.sql` 移到 `testdata/diff/` 根目录（与 `testdata/diff/snapshot.sql` 并列），或移到 `testdata/diff/multi_schema/` 下作为 v2 的快照。

**优点**：
- 零代码变更
- 测试数据语义清晰：v2 目录只包含分文件，快照独立存放
- 与 `testdata/diff/v1/`、`testdata/diff/v2/` 的组织方式一致（`testdata/diff/snapshot.sql` 放在根目录，不在 v2/ 子目录中）

**缺点**：
- 需要更新 `testdata/README.md` 中的目录结构说明

### 方案 B：DirectoryLoader 支持排除特定文件

在 `DirectoryLoader` 中添加排除模式（如 `--exclude` 标志或自动排除 `snapshot.sql`）。

**优点**：
- 更灵活，用户可以将快照放在同一目录

**缺点**：
- 增加复杂度
- `snapshot.sql` 的排除规则是 migra 特有的，不够通用

### 方案 C：`CreateTableMutation.Apply` 对相同定义的重复表做幂等处理

如果重复的 `CREATE TABLE` 定义完全相同，则静默跳过而不是报错。

**优点**：
- 容错性更强

**缺点**：
- 可能掩盖真正的错误（如两次定义不同却被静默忽略）
- 增加 Mutation 逻辑复杂度

**推荐方案 A**，因为：
1. `testdata/diff/snapshot.sql` 已经在根目录，`multi_schema/v2/snapshot.sql` 放在子目录中是组织不一致
2. 零代码变更，风险最低
3. 测试数据的设计原则（README 第 288 行）说"与对应目录内容等效"，意味着快照应该独立于目录存在

---

## 相关代码文件

| 文件 | 角色 |
|---|---|
| `internal/source/dir_loader.go` | DirectoryLoader，WalkDir 扫描所有 `.sql` 文件 |
| `internal/parser/mutation.go` | `CreateTableMutation.Apply()`，重复表检测 |
| `internal/parser/applier.go` | `MutationApplier.Apply()`，错误聚合 |
| `internal/parser/parser.go` | `Parser.ParseSQL()`，解析入口 |
| `testdata/diff/multi_schema/v2/snapshot.sql` | 冲突的合并快照文件 |
| `testdata/diff/multi_schema/v2/01_public.sql` | 分文件，定义 public.users + public.profiles |
| `testdata/diff/multi_schema/v2/02_auth.sql` | 分文件，定义 auth.roles + auth.permissions |
| `testdata/README.md` | 测试数据目录结构说明 |

---

## 验证方法

修复后应满足：

1. `./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/` 成功执行
2. 输出包含：
   - `CREATE TABLE "public"."profiles"` (新增表)
   - `ALTER TABLE "auth"."roles" ADD COLUMN "description"` (新增列)
   - `ALTER TABLE "public"."users" ADD COLUMN "email"` (新增列)
3. 不输出任何 `DROP` 操作（safe mode）
4. 不输出解析错误或警告
