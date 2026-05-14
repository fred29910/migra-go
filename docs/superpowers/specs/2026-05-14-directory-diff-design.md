# Directory Diff 功能设计

## 1. 背景与目标

### 1.1 当前状态

`migra diff` 目前支持以下对比模式：

- `file.sql` vs `file.sql` ✅
- `file.sql` vs `postgres://` ✅
- `postgres://` vs `postgres://` ✅

但不支持目录（文件夹）作为输入源。

### 1.2 目标

支持目录作为 schema 来源，递归扫描目录下所有 `.sql` 文件，合并成一个完整的 `model.Schema`，然后参与 diff 对比。

### 1.3 使用示例

```bash
# 目录 vs 目录
migra diff ./schemas/v1/ ./schemas/v2/

# 目录 vs 数据库
migra diff ./schemas/v1/ postgres://localhost/mydb

# 目录 vs 单文件
migra diff ./schemas/v1/ ./schemas/v2/snapshot.sql

# 带参数
migra diff ./v1/ ./v2/ --schema public,auth --format json
```

## 2. 方案选择

### 方案 A：新增 DirectoryLoader（选中）

在 `internal/source/` 下新建 `dir_loader.go`，实现 `Loader` 接口。内部复用 `SQLFileLoader` 解析单个文件，自己负责递归扫描 + schema 合并。

**优点：**
- 完全复用现有 `Loader` 接口和 `Registry` 分发机制，零侵入
- 与 `SQLFileLoader`、`DBLoader` 并列，职责清晰
- CLI 无需改动，`migra diff dir_a/ dir_b/` 直接可用
- 可独立测试

### 方案 B：扩展 SQLFileLoader（未选中）

违反单一职责原则，一个 loader 同时处理单文件和目录，后续难以维护。

### 方案 C：在 diff_service 预处理（未选中）

绕过了 `Registry` 的分发机制，破坏了架构一致性。

## 3. 架构设计

```
migra diff dir_a/ dir_b/
         │         │
         ▼         ▼
    ┌─────────────────┐
    │  sourceRegistry  │  ← Registry.Match() 识别目录路径
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │ DirectoryLoader  │  ← 新增，实现 Loader 接口
    │  .Match()        │     检测 os.Stat().IsDir()
    │  .Load()         │     递归扫描 → 逐个解析 → 合并 schema
    └────────┬────────┘
             │ 内部复用
             ▼
    ┌─────────────────┐
    │  SQLFileLoader   │  ← 解析单个 .sql 文件
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  model.Schema    │  ← 合并后的完整 schema
    └─────────────────┘
```

## 4. 组件设计

### 4.1 DirectoryLoader

**文件：** `internal/source/dir_loader.go`

```go
type DirectoryLoader struct {
    fileLoader *SQLFileLoader  // 复用单文件解析
}

// Match 返回 true 如果 source 是一个存在的目录路径
func (l *DirectoryLoader) Match(source string) bool

// Load 递归扫描目录下所有 .sql 文件，合并后返回单个 schema
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
```

### 4.2 Registry 匹配顺序

```
DBLoader → DirectoryLoader → SQLFileLoader
```

`DirectoryLoader` 在 `SQLFileLoader` 之前注册，通过 `os.Stat().IsDir()` 判断路径是否为目录，确保以下场景正确路由：

| 输入路径 | 匹配 loader | 原因 |
|---------|------------|------|
| `postgres://...` | DBLoader | DBLoader 优先匹配 |
| `./schemas/v1/` | DirectoryLoader | os.Stat 判断为目录 |
| `file:///path/to/dir/` | DirectoryLoader | strip `file://` 后 os.Stat 判断为目录 |
| `./schemas/v1.sql/` | DirectoryLoader | os.Stat 判断为目录（此前会被 SQLFileLoader 误匹配） |
| `file.sql` | SQLFileLoader | DirectoryLoader 判断非目录 |
| `file:///path/to/file.sql` | SQLFileLoader | DirectoryLoader strip `file://` 后判断非目录 |

**注意：** 目录名若以 `.sql` 结尾（例如 `./schemas/v1.sql/`），`SQLFileLoader` 不会误匹配，因为 `DirectoryLoader` 先通过 `os.Stat` 判断为目录。这是已知的预期行为，生产环境中仍建议避免此类命名以减少混淆。

### 4.3 注册

在 `cmd/migra/diff.go` 的 `sourceRegistry` 中添加：

```go
reg.Register(&source.DirectoryLoader{})
```

## 5. 文件扫描规则

```
dir_a/
├── tables/
│   ├── users.sql      ✅ 匹配 *.sql
│   ├── posts.sql      ✅ 匹配 *.sql
│   └── readme.txt     ❌ 跳过
├── indexes/
│   └── idx.sql        ✅ 匹配 *.sql
└── types.sql          ✅ 匹配 *.sql
```

- 使用 `filepath.WalkDir` 递归遍历
- 只处理 `.sql` 后缀文件（大小写不敏感）
- 按文件路径排序后依次解析（确定性输出）
- 跳过隐藏文件（以 `.` 开头的）

## 6. Schema 合并策略

```
merged = new Schema()
seenTables = {}    // map[namespace.table] → file path
seenEnums = {}     // map[namespace.enum] → file path

for each sorted .sql file:
    schema_i, parseErr = SQLFileLoader.Load(file, strict=true)  // 始终使用 strict 模式
    if parseErr != nil:
        return fatal error  // 任一文件失败则整个目录加载失败

    for each (nsName, ns) in schema_i.Schemas:
        mergedNs = merged.GetOrCreateNamespace(nsName)

        for each (tableName, table) in ns.Tables:
            key = nsName + "." + tableName
            if key in seenTables:
                return fatal error: "duplicate table 'tableName' in file (first in seenTables[key])"
            seenTables[key] = file
            mergedNs.Tables[tableName] = table

        for each (typeName, enum) in ns.Types:
            key = nsName + "." + typeName
            if key in seenEnums:
                return fatal error: "duplicate enum 'typeName' in file (first in seenEnums[key])"
            seenEnums[key] = file
            mergedNs.Types[typeName] = enum
```

**关键规则：**
- 同名 table/enum 在不同文件中 → **fatal error**，包含冲突对象名和文件路径
- 任一文件解析失败 → **fatal error**，整个目录加载失败（即使非 strict 模式）

## 7. 错误处理

| 场景 | 行为 |
|------|------|
| 路径不存在 | fatal error: "directory not found: %s" |
| 路径不是目录 | `Match()` 返回 false，Registry 不会选中此 loader |
| 目录为空（无 .sql 文件） | 返回空 schema + warning（`[]error: "no .sql files found"`） |
| 某个 .sql 文件解析失败 | fatal error，整个目录加载失败 |
| 同名 table/enum 冲突 | fatal error，包含冲突对象名和文件路径 |

### 与 SQLFileLoader 的行为差异

| 场景 | SQLFileLoader | DirectoryLoader |
|------|--------------|-----------------|
| .sql 文件解析失败（非 strict 模式） | 返回 warning + 部分 schema | fatal error（始终使用 `Strict: true`） |
| 空文件/空字符串 | 返回空 schema | （同 SQLFileLoader） |

**设计理由：** DirectoryLoader 合并多个文件构建完整 schema，其中任何一个文件失败都会导致 schema 不完整。因此即使调用方传入非 strict 模式，DirectoryLoader 内部也始终使用 `Strict: true`。

## 8. 测试策略

### 8.1 单元测试 (`internal/source/dir_loader_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestDirectoryLoader_Match_Directory` | 目录路径返回 true |
| `TestDirectoryLoader_Match_File` | 文件路径返回 false |
| `TestDirectoryLoader_Match_NotExist` | 不存在路径返回 false |
| `TestDirectoryLoader_Load_MultiFile` | 正常多文件合并 |
| `TestDirectoryLoader_Load_EmptyDir` | 空目录返回空 schema |
| `TestDirectoryLoader_Load_NestedDir` | 嵌套子目录递归扫描 |
| `TestDirectoryLoader_Load_DuplicateTable` | 同名表返回 fatal error |
| `TestDirectoryLoader_Load_DuplicateEnum` | 同名 enum 返回 fatal error |
| `TestDirectoryLoader_Load_ParseError` | 某文件解析失败返回 fatal error |
| `TestDirectoryLoader_Load_SkipNonSQL` | 跳过非 .sql 文件 |
| `TestDirectoryLoader_Load_SkipHidden` | 跳过隐藏文件 |

### 8.2 集成测试 (`cmd/migra/integration_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestDirectoryVsDirectory_Diff` | 两个目录对比生成 diff |
| `TestDirectoryVsFile_Diff` | 目录 vs 单文件对比 |
| `TestDirectoryVsDB_Diff` | 目录 vs 数据库对比 |

## 9. 文件清单

| 操作 | 文件 |
|------|------|
| 新增 | `internal/source/dir_loader.go` |
| 新增 | `internal/source/dir_loader_test.go` |
| 修改 | `cmd/migra/diff.go` — 注册 DirectoryLoader |
| 修改 | `cmd/migra/integration_test.go` — 添加集成测试 |

## 10. 不在范围内

- 逐文件配对对比（按文件名配对生成多个 diff）
- 目录 vs 目录的增量迁移策略
- 文件级别的 diff 标注（标注每个操作来自哪个文件）
