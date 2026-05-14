# Spec 评审：Directory Diff 功能设计

**评审日期：** 2026-05-14
**评审对象：** `docs/superpowers/specs/2026-05-14-directory-diff-design.md`
**代码基准：** `migra-go` 当前 main 分支

---

## 总体评价

方案选型正确（方案 A → 新增 DirectoryLoader），架构设计与现有 `Loader` 接口和 `Registry` 分发机制完全吻合，CLI 兼容无需改动。但 schema 合并策略中使用了不存在的 API，需要在实现前修正。

---

## 必须修复

### 1. Schema 合并策略使用了不存在的 API

**位置：** 第 6 节（Schema 合并策略）伪代码

**问题：** spec 中使用了 `merged.AddNamespace(namespace)`、`merged.PutTable(table)`、`merged.PutEnum(enum)` 三个方法，但 `model.Schema` 和 `model.Namespace` 上**均不存在**这些方法。

**实际 API（`internal/model/schema.go`）：**

```go
type Schema struct {
    Schemas map[string]*Namespace
}

type Namespace struct {
    Name   string
    Tables map[string]*Table
    Types  map[string]*EnumType
}

// 已有的方法
func (s *Schema) GetOrCreateNamespace(name string) *Namespace
func (s *Schema) GetNamespace(name string) *Namespace
```

目前操作 table/enum 的方式是直接操作 map：`ns.Tables[name] = table`、`ns.Types[name] = enum`。

**建议：**

推荐在 `internal/model/schema.go` 中新增 `Merge` 方法，将合并逻辑集中到 model 层：

```go
// Merge 将另一个 schema 合并到当前 schema，遇到同名 table/enum 返回 error。
func (s *Schema) Merge(other *Schema) error {
    for name, ns := range other.Schemas {
        existing := s.GetNamespace(name)
        if existing == nil {
            s.Schemas[name] = ns
            continue
        }
        for tName, table := range ns.Tables {
            if _, exists := existing.Tables[tName]; exists {
                return fmt.Errorf("duplicate table %q in namespace %q", tName, name)
            }
            existing.Tables[tName] = table
        }
        for eName, enum := range ns.Types {
            if _, exists := existing.Types[eName]; exists {
                return fmt.Errorf("duplicate enum %q in namespace %q", eName, name)
            }
            existing.Types[eName] = enum
        }
    }
    return nil
}
```

优势：
- DirectoryLoader 只需要 `merged.Merge(schema_i)` 一行
- model 层可独立测试
- 后续其他 loader（如果有）也能复用

---

### 2. DirectoryLoader 缺少初始化策略

**位置：** 第 4.1 节

**问题：** `DirectoryLoader` 内部持有 `fileLoader *SQLFileLoader`，但 spec 未说明它如何初始化。当前 `diff.go` 中注册方式是 `reg.Register(&source.DirectoryLoader{})`，此时 `fileLoader` 为 nil。

**建议：** 在 `Load()` 方法中惰性初始化，保证零值结构体也能正常工作：

```go
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    if l.fileLoader == nil {
        l.fileLoader = &SQLFileLoader{}
    }
    // ...
}
```

---

## 建议修改

### 3. 解析失败策略与 SQLFileLoader 不一致

**位置：** 第 7 节错误处理表格

**问题：** spec 规定"任一 .sql 文件解析失败 → fatal error（即使非 strict 模式）"。但 `SQLFileLoader.Load()` 在非 strict 模式下会忽略解析错误并返回部分 schema。**这个差异是有意设计还是遗漏？** 当前 spec 没有解释。

目录加载的场景确实特殊——部分失败会导致 schema 不完整，视为 fatal 是合理的。但这个决策应当**显式记录**，避免后续维护者困惑。

**建议：** 在错误处理表格中加一行说明：

| .sql 文件解析失败（非 strict 模式） | SQLFileLoader: 返回 warning + 部分 schema | DirectoryLoader: fatal error（目录级加载要求完整 schema） |
|---|---|---|

或者——如果业务需要——复用 `opt.Strict` 标志，strict 时 fatal，非 strict 时跳过失败文件继续合并。

---

### 4. 隐藏目录遍历规则不明确

**位置：** 第 5 节，第 133 行

**问题：** spec 只写了"跳过隐藏文件（以 `.` 开头的）"，但 `filepath.WalkDir` 也会进入隐藏目录。当目录结构如下时规则不明确：

```
dir_a/
├── .hidden_dir/
│   └── users.sql      ← 是否被扫描？
├── tables/
│   ├── .draft.sql     ← 跳过（隐藏文件）
│   └── posts.sql
```

**建议：** 明确规则，推荐选项 A：

- **选项 A（推荐）：** 遇到隐藏目录直接 `filepath.SkipDir`，不进入。`.git/`、`.svn/` 等常见目录自然被排除。
- **选项 B：** 只跳过隐藏文件，但正常遍历隐藏目录下的内容。

---

### 5. `file://` 前缀的目录路径未被考虑

**位置：** 第 1.3 节使用示例

**问题：** `SQLFileLoader.Match()` 匹配 `file://` 前缀。如果用户传入 `file:///path/to/dir/`，`SQLFileLoader` 会先匹配成功，然后尝试读取整个目录作为文件内容——必然失败。

**建议（三选一，写入 spec）：**

1. `DirectoryLoader.Match()` 先于 `SQLFileLoader` 注册，且同时检测 `file://` 前缀后的路径是否为目录
2. 明确声明 `file://` 前缀只支持单文件，目录必须用裸路径
3. `DirectoryLoader.Match()` 对 `file://` 路径 strip 前缀后再 stat

推荐方案 1：兼容性最好。

---

### 6. Match 优先级潜在冲突需注明

**位置：** 第 4.2 节

**问题：** Registry 是顺序匹配的。现有匹配链：

```
DBLoader → SQLFileLoader → DirectoryLoader
```

对于绝大多数情况没问题，但如果**目录名恰好以 `.sql` 结尾**（例如 `./schemas/v1.sql/`），`SQLFileLoader` 会误匹配。

**建议：** 在第 4.2 节添加注释说明这是已知假设：

```
// 注意：目录名若以 .sql 结尾会被 SQLFileLoader 误匹配。
// 这是已知限制，生产环境中建议避免此类命名。
```

---

### 7. 测试策略可补充 testutil 复用

**位置：** 第 8 节测试策略

**问题：** 测试用例清单全面，但没有利用项目中已有的 `internal/testutil/testutil.go`，它提供了 `LoadSchemaFromJSON`、`SaveSchemaToJSON`、`GoldenFile` 等辅助工具。

**建议：** 在测试策略中补充：

> 合并后的 schema 可使用 `testutil.SaveSchemaToJSON(t, schema)` 输出为 JSON 进行 golden 文件比对，或使用 `testutil.LoadSchemaFromJSON` 加载预期结果。参考 `internal/model/golden_test.go` 的模式。

---

## 仅供参考

### 8. push 命令天然兼容

push 命令（`push_runner.go` 第 136 行）共享同一个 `sourceRegistry`，通过 `loadSchemaWithContext` 加载 schema。因此 `migra push ./schemas/v1/ postgres://localhost/db` 无需额外开发即可工作。可以在第 3 节或第 10 节提一下。

### 9. 合并策略跟踪文件来源

spec 中错误信息包含冲突文件路径（`"duplicate table 'X' in file Y (first in Z)"`）。实现时需要在 DirectoryLoader 内部维护一个 `fileOrigin map[string]string` 来记录每个 object 来源于哪个文件。

### 10. 可考虑跳过常见非 SQL 目录

`filepath.WalkDir` 默认遍历所有子目录。可以通过 `SkipDir` 跳过 `node_modules`、`.git`、`__pycache__` 等目录。第 5 节可以作为可扩展点提到。

### 11. 空目录 warning 的准确表示

第 165 行"目录为空返回 warning"。`Load()` 返回签名是 `(*model.Schema, []error, error)`——warning 应放在 `[]error` 中：

```go
return model.NewSchema(), []error{fmt.Errorf("no .sql files found in directory: %s", source)}, nil
```

---

## 逐项检查清单

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 架构方案合理 | ✅ | 新增 DirectoryLoader，完美适配 Loader 接口 + Registry |
| CLI 兼容 | ✅ | 无需改动参数解析，`migra diff dir/ dir/` 直接可用 |
| Loader 接口签名 | ✅ | Match/Load 签名完全一致 |
| Registry 注册 | ⚠️ | 需要惰性初始化保证零值可用（见 #2） |
| Schema 合并策略 | ❌ | 使用了不存在的 API（PutTable/PutEnum），需修正（见 #1） |
| 错误处理 | ⚠️ | 需补充与 SQLFileLoader 行为差异说明（见 #3） |
| 文件扫描规则 | ⚠️ | 隐藏目录遍历规则不明确（见 #4） |
| file:// 兼容 | ⚠️ | 未考虑 file:// 前缀路径（见 #5） |
| push 命令 | ✅ | 共享 sourceRegistry，自动兼容 |
| 测试覆盖 | ✅ | 用例清单完整，建议补充 testutil 指引（见 #7） |

---

## 建议修复优先级

1. **实现前必须修复：** 修复 #1（合并 API）+ #2（初始化策略）
2. **实现前应明确：** #3（解析失败策略）+ #4（隐藏目录规则）+ #5（file:// 兼容）
3. **可跟进补充：** #6（Match 注释）+ #7（testutil 指引）+ #8-#11（仅供参考）
