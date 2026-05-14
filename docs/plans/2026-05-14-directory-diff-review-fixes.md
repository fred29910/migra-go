# Directory Diff 评审问题修复计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 DirectoryDiff 设计评审中仍然存在的 5 个问题（#3 spec文档、#5 file://目录、#6 .sql目录冲突、#7 testutil复用、#11 空目录warning）

**架构:** 核心修复是调整 Registry 注册顺序 + DirectoryLoader.Match() 增强，让 DirectoryLoader 早于 SQLFileLoader 匹配目录路径。file:// 和 .sql 后缀的目录通过 stat 判断而非硬编码拒绝。

**Tech Stack:** Go, model.Schema, source.DirectoryLoader, source.Registry

---

## 实际核查结论

| Review 问题 | 当前状态 | 处理 |
|---|---|---|
| #1 合并API不存在 | ✅ 代码使用有效 API (GetOrCreateNamespace + 直接 map) | 无需处理 |
| #2 缺少懒初始化 | ✅ 已实现 (dir_loader.go:71-74) | 无需处理 |
| #3 Spec文档未更新 | ⚠️ spec 与实现有偏差（错误处理策略未记录） | **更新文档** |
| #4 隐藏目录规则不明确 | ✅ 已用 SkipDir 处理 | 无需处理 |
| #5 file://目录不支持 | ❌ Match() 显式拒绝 file://，导致无法使用 | **修复** |
| #6 .sql后缀目录被误匹配 | ❌ Match() 显式拒绝 .sql，导致 SQLFileLoader 误匹配 | **修复** |
| #7 testutil未复用 | ⚠️ 未使用 testutil 辅助函数 | **可选增强** |
| #8 push兼容 | ✅ 共享 sourceRegistry | 无需处理 |
| #9 文件来源追踪 | ✅ seenTables/seenEnums 已实现 | 无需处理 |
| #10 跳过常见非SQL目录 | ✅ 隐藏目录已跳过 | 无需处理 |
| #11 空目录缺warning | ❌ 空目录静默返回空 schema | **修复** |

---

## 根因分析

### #5 + #6 的共通根因：Match() 策略偏保守

`DirectoryLoader.Match()` 当前用字符串前缀/后缀判断（`hasPrefix("file://")` + `hasSuffix(".sql")`），替代了真正的文件系统 `os.Stat().IsDir()` 检查。这导致两个问题：
- `file:///path/to/dir/` → 被拒绝（应该 stat 后判断为目录）
- `./schemas/v1.sql/` → 被拒绝（应该 stat 后判断为目录）

修复方案：去掉前缀/后缀拦截，统一用 `os.Stat` 判断。同时把 `DirectoryLoader` 注册到 `SQLFileLoader` **之前**，避免 `SQLFileLoader` 先匹配 `file://` 路径。

### #3 的根因：实现未回写 spec 文档

实现过程中做了设计决策，但没有更新 spec 文档。

---

## 实施计划

### Task 1: 修复 file:// 和 .sql 目录支持（#5 + #6）

这是核心修复，解决了 Registry 匹配顺序和 DirectoryLoader.Match() 的判断逻辑。

**涉及文件:**
- 修改: `internal/source/dir_loader.go` — Match() 和 Load()
- 修改: `cmd/migra/diff.go` — Registry 注册顺序

#### Step 1: 重新设计 DirectoryLoader.Match()

旧的 Match()：
```go
func (l *DirectoryLoader) Match(source string) bool {
    lower := strings.ToLower(source)
    if strings.HasPrefix(lower, "postgres://") ||
        strings.HasPrefix(lower, "postgresql://") ||
        strings.HasPrefix(lower, "pg://") ||
        strings.HasPrefix(lower, "file://") ||
        strings.HasSuffix(lower, ".sql") {
        return false
    }
    info, err := os.Stat(source)
    if err != nil {
        return false
    }
    return info.IsDir()
}
```

新的 Match()：
```go
func (l *DirectoryLoader) Match(source string) bool {
    // DB URLs are handled by DBLoader, not by us
    lower := strings.ToLower(source)
    if strings.HasPrefix(lower, "postgres://") ||
        strings.HasPrefix(lower, "postgresql://") ||
        strings.HasPrefix(lower, "pg://") {
        return false
    }
    // Strip file:// prefix for filesystem check
    path := source
    if strings.HasPrefix(lower, "file://") {
        path = source[len("file://"):]
    }
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return info.IsDir()
}
```

#### Step 2: DirectoryLoader.Load() 中处理 file:// 前缀

Load() 函数开始时：
```go
// Strip file:// prefix if present
sourcePath := source
if strings.HasPrefix(strings.ToLower(source), "file://") {
    sourcePath = source[len("file://"):]
}
// 后续所有操作使用 sourcePath 而不是 source
```

#### Step 3: 调整 Registry 注册顺序

`cmd/migra/diff.go` 中：
```go
// 旧顺序：DBLoader → SQLFileLoader → DirectoryLoader
reg.Register(&source.DBLoader{})
reg.Register(&source.SQLFileLoader{})
reg.Register(&source.DirectoryLoader{})

// 新顺序：DBLoader → DirectoryLoader → SQLFileLoader
reg.Register(&source.DBLoader{})
reg.Register(&source.DirectoryLoader{})
reg.Register(&source.SQLFileLoader{})
```

匹配逻辑流：
1. `postgres://...` → DBLoader 匹配 ✅
2. `./path/to/dir/` → DirectoryLoader stat 判断为目录 → 匹配 ✅
3. `file:///path/to/dir/` → DirectoryLoader strip file://, stat 判断为目录 → 匹配 ✅
4. `./schemas/v1.sql/` → DirectoryLoader stat 判断为目录 → 匹配 ✅ (以前 SQLFileLoader 会误匹配)
5. `./path/to/file.sql` → DirectoryLoader stat 判断为文件 → 不匹配 → SQLFileLoader 匹配 ✅
6. `file:///path/to/file.sql` → DirectoryLoader strip file://, stat 判断为文件 → 不匹配 → SQLFileLoader 匹配 ✅

#### Step 4: 运行已有测试验证无回归

```bash
cd /opt/codes/workspace/migra-go
rtk go test ./internal/source/ -run TestDirectoryLoader -v
rtk go test ./cmd/migra/ -run TestDirectoryVsDirectory_Diff -v
rtk go test ./cmd/migra/ -run TestDirectoryVsFile_Diff -v
```

#### Step 5: Commit

```bash
rtk git add internal/source/dir_loader.go cmd/migra/diff.go
rtk git commit -m "fix: support file:// directories and .sql-ending dirs in DirectoryLoader

- Reorder Registry: DirectoryLoader before SQLFileLoader
- Match() now uses os.Stat to detect directories instead of string prefix/suffix
- Load() strips file:// prefix before scanning
- Fixes file:///path/to/dir/ and ./v1.sql/ directory support"
```

---

### Task 2: 空目录返回 warning（#11）

**涉及文件:**
- 修改: `internal/source/dir_loader.go`

#### Step 1: 在 Load() 的 sort 之后、循环之前添加空目录检测

```go
sort.Strings(files)

if len(files) == 0 {
    return model.NewSchema(), []error{fmt.Errorf("no .sql files found in directory: %s", source)}, nil
}
```

#### Step 2: 运行测试验证

```bash
rtk go test ./internal/source/ -run TestDirectoryLoader_Load_EmptyDir -v
```

预期：测试需要更新以接受 warning 但不是 fatal error。

#### Step 3: 更新 TestDirectoryLoader_Load_EmptyDir 测试

当前测试：
```go
func TestDirectoryLoader_Load_EmptyDir(t *testing.T) {
    dir := t.TempDir()
    loader := &DirectoryLoader{}
    schema, _, err := loader.Load(nil, dir, LoadOptions{})
    // ...
}
```

需要修改为验证 `errs` 参数包含 `"no .sql files found"` warning:
```go
func TestDirectoryLoader_Load_EmptyDir(t *testing.T) {
    dir := t.TempDir()
    loader := &DirectoryLoader{}
    schema, errs, err := loader.Load(nil, dir, LoadOptions{})
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if schema == nil {
        t.Fatal("expected non-nil schema")
    }
    if len(schema.Schemas) != 0 {
        t.Errorf("expected empty schema, got %d namespaces", len(schema.Schemas))
    }
    if len(errs) == 0 {
        t.Error("expected warning for empty directory, got none")
    }
    if !strings.Contains(errs[0].Error(), "no .sql files found") {
        t.Errorf("expected 'no .sql files found' warning, got: %v", errs[0])
    }
}
```

#### Step 4: 运行完整测试

```bash
rtk go test ./internal/source/ -run TestDirectoryLoader -v
```

#### Step 5: Commit

```bash
rtk git add internal/source/dir_loader.go internal/source/dir_loader_test.go
rtk git commit -m "feat: emit warning for empty directory in DirectoryLoader

- Return warning in []error when directory has no .sql files
- Previously returned empty schema silently"
```

---

### Task 3: 更新 spec 文档（#3）

**涉及文件:**
- 修改: `docs/superpowers/specs/2026-05-14-directory-diff-design.md`

#### Step 1: 更新第 7 节（错误处理表格）

添加 SQLFileLoader 与 DirectoryLoader 行为差异对比行：

```
| .sql 文件解析失败（非 strict 模式） | SQLFileLoader: 返回 warning + 部分 schema | DirectoryLoader: fatal error（目录级加载要求完整 schema，始终使用 strict 模式） |
```

#### Step 2: 更新第 4.2 节（Registry 匹配顺序）

将注册顺序从 `DBLoader → SQLFileLoader → DirectoryLoader` 更新为 `DBLoader → DirectoryLoader → SQLFileLoader`，并补充说明为什么：

```
DBLoader → DirectoryLoader → SQLFileLoader

DirectoryLoader 通过 os.Stat 判断路径是否为目录，支持：
- 裸目录路径（`./schemas/v1/`）
- file:// 目录路径（`file:///path/to/dir/`）
- .sql 结尾的目录路径（`./v1.sql/` — 此前会被 SQLFileLoader 误匹配）
```

#### Step 3: 更新第 6 节（Schema 合并策略）

将伪代码中的 `merged.AddNamespace`、`merged.PutTable`、`merged.PutEnum` 替换为实际使用的 `merged.GetOrCreateNamespace` + 直接 map 赋值模式，反映真实实现。

#### Step 4: Commit

```bash
rtk git add docs/superpowers/specs/2026-05-14-directory-diff-design.md
rtk git commit -m "docs: sync DirectoryDiff spec with actual implementation

- Update Registry ordering: DirectoryLoader before SQLFileLoader
- Update error handling: document DirectoryLoader vs SQLFileLoader diff
- Update merge pseudo-code to match real implementation"
```

---

### Task 4（可选）: 增强测试使用 testutil（#7）

**涉及文件:**
- 修改: `internal/source/dir_loader_test.go`

将 MultiFile 和 NestedDir 测试中的直接 assert 模式改为使用 `testutil.LoadSchemaFromJSON` / golden file 模式。这一步可选，当前测试已足够覆盖功能。

步骤略（优先级低）。

---

## 验证清单

| 验证项 | 方法 | 预期结果 |
|--------|------|---------|
| `file:///path/to/dir/` 正常加载 | 创建临时目录 + .sql 文件，通过 Match + Load 测试 | 匹配成功，schema 正确合并 |
| `./v1.sql/` 目录正常加载 | 创建以 `.sql` 结尾的临时目录，通过 Match 测试 | 匹配成功，不再被 SQLFileLoader 误截 |
| 空目录返回 warning | 现有 TestDirectoryLoader_Load_EmptyDir 更新 | `errs` 包含 "no .sql files found" |
| 全部单元测试通过 | `rtk go test ./internal/source/ -v` | 全部 PASS |
| 集成测试通过 | `rtk go test ./cmd/migra/ -v` | TestDirectoryVs* 全部 PASS |
| 全部测试无回归 | `rtk go test ./...` | 0 failures |

## 执行顺序

```
Task 1 (核心修复) ──→ Task 2 (空目录 warning) ──→ Task 3 (文档更新)
     ↑                      ↑
 必须优先              依赖 Task 1 的基础
```
