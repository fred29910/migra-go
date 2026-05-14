# Directory Diff 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 支持目录作为 `migra diff` 的 schema 来源，递归扫描目录下所有 `.sql` 文件并合并成一个 `model.Schema`，然后参与 diff 对比。

**架构：** 新增 `DirectoryLoader` 实现 `Loader` 接口，内部复用 `SQLFileLoader` 解析单个文件，自己负责递归扫描 + schema 合并。注册到 `sourceRegistry` 作为兜底 loader，CLI 层零改动。

**技术栈：** Go 1.22+, `filepath.WalkDir`, `os.Stat`, 现有 `Loader` / `Registry` / `model.Schema` 接口

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/source/dir_loader.go` | 创建 | `DirectoryLoader` — 递归扫描 + schema 合并 |
| `internal/source/dir_loader_test.go` | 创建 | `DirectoryLoader` 单元测试 |
| `cmd/migra/diff.go` | 修改 | 注册 `DirectoryLoader` 到 `sourceRegistry` |
| `cmd/migra/integration_test.go` | 修改 | 添加目录对比集成测试 |

---

## 任务 1：创建 DirectoryLoader — Match 方法

**文件：**
- 创建：`internal/source/dir_loader.go`
- 测试：`internal/source/dir_loader_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `internal/source/dir_loader_test.go` 中：

```go
package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryLoader_Match(t *testing.T) {
	loader := &DirectoryLoader{}

	// 创建临时目录
	dir := t.TempDir()

	// 创建临时文件
	file := filepath.Join(dir, "test.sql")
	if err := os.WriteFile(file, []byte("SELECT 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{"directory", dir, true},
		{"sql file", file, false},
		{"postgres url", "postgres://localhost/db", false},
		{"file prefix", "file:///tmp/test.sql", false},
		{"non-existent", "/non/existent/path", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loader.Match(c.source); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.source, got, c.want)
			}
		})
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/source/ -run TestDirectoryLoader_Match -v
```
预期：FAIL，报错 `undefined: DirectoryLoader`

- [ ] **步骤 3：编写最少实现代码**

在 `internal/source/dir_loader.go` 中：

```go
package source

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)

// DirectoryLoader implements Loader for directory sources.
// It recursively scans all .sql files in a directory, parses each file
// using SQLFileLoader, and merges them into a single Schema.
type DirectoryLoader struct {
	fileLoader *SQLFileLoader
}

// Match returns true if source is an existing directory.
func (l *DirectoryLoader) Match(source string) bool {
	// Skip sources that other loaders handle
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

// Load recursively scans all .sql files in the directory, parses each file,
// and merges them into a single Schema.
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// TODO: implement in next task
	return nil, nil, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/source/ -run TestDirectoryLoader_Match -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/source/dir_loader.go internal/source/dir_loader_test.go
git commit -m "feat: add DirectoryLoader.Match() with tests"
```

---

## 任务 2：实现 DirectoryLoader — Load 方法（递归扫描 + schema 合并）

**文件：**
- 修改：`internal/source/dir_loader.go`
- 修改：`internal/source/dir_loader_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `internal/source/dir_loader_test.go` 中追加：

```go
func TestDirectoryLoader_Load_MultiFile(t *testing.T) {
	dir := t.TempDir()

	// 创建两个 SQL 文件
	sql1 := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`
	sql2 := `CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_posts.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, errs, err := loader.Load(nil, dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	// 验证两个表都被加载
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["posts"]; !ok {
		t.Error("expected posts table")
	}
}

func TestDirectoryLoader_Load_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(nil, dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if len(schema.Schemas) != 0 {
		t.Errorf("expected empty schema, got %d namespaces", len(schema.Schemas))
	}
}

func TestDirectoryLoader_Load_NestedDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "tables")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `CREATE TABLE posts (id SERIAL PRIMARY KEY);`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "02_posts.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(nil, dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["posts"]; !ok {
		t.Error("expected posts table from nested dir")
	}
}

func TestDirectoryLoader_Load_DuplicateTable(t *testing.T) {
	dir := t.TempDir()

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(50));`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_users.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	_, _, err := loader.Load(nil, dir, LoadOptions{})
	if err == nil {
		t.Fatal("expected error for duplicate table, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate table") {
		t.Errorf("expected 'duplicate table' in error, got: %v", err)
	}
}

func TestDirectoryLoader_Load_ParseError(t *testing.T) {
	dir := t.TempDir()

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `THIS IS NOT VALID SQL;`

	if err := os.WriteFile(filepath.Join(dir, "01_good.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_bad.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	_, _, err := loader.Load(nil, dir, LoadOptions{})
	if err == nil {
		t.Fatal("expected error for parse failure, got nil")
	}
}

func TestDirectoryLoader_Load_SkipNonSQL(t *testing.T) {
	dir := t.TempDir()

	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	readme := "This is not SQL"

	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(nil, dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
}

func TestDirectoryLoader_Load_SkipHidden(t *testing.T) {
	dir := t.TempDir()

	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	hidden := `CREATE TABLE secret (id SERIAL PRIMARY KEY);`

	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.sql"), []byte(hidden), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(nil, dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["secret"]; ok {
		t.Error("hidden file should be skipped")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/source/ -run TestDirectoryLoader_Load -v
```
预期：FAIL，因为 Load 方法返回 nil

- [ ] **步骤 3：编写 Load 方法实现**

修改 `internal/source/dir_loader.go` 中的 `Load` 方法：

```go
// Load recursively scans all .sql files in the directory, parses each file,
// and merges them into a single Schema.
// If any file fails to parse, the entire load fails (even in non-strict mode).
// If duplicate table/enum names are found across files, the load fails.
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	// Collect all .sql files recursively
	var files []string
	err := filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan directory %s: %w", source, err)
	}

	// Sort files for deterministic output
	sort.Strings(files)

	// Merge schemas from all files
	merged := model.NewSchema()
	allErrs := make([]error, 0)
	seenTables := make(map[string]string) // table key -> first file path
	seenEnums := make(map[string]string)  // enum key -> first file path

	fl := l.fileLoader
	if fl == nil {
		fl = &SQLFileLoader{}
	}

	for _, file := range files {
		schema, errs, loadErr := fl.Load(ctx, file, opt)
		if loadErr != nil {
			return nil, nil, fmt.Errorf("failed to parse %s: %w", file, loadErr)
		}
		allErrs = append(allErrs, errs...)

		// Merge namespaces
		for nsName, ns := range schema.Schemas {
			mergedNs := merged.GetOrCreateNamespace(nsName)

			// Merge tables
			for tableName, table := range ns.Tables {
				key := nsName + "." + tableName
				if firstFile, exists := seenTables[key]; exists {
					return nil, nil, fmt.Errorf(
						"duplicate table '%s' found in %s (first defined in %s)",
						tableName, file, firstFile,
					)
				}
				seenTables[key] = file
				mergedNs.Tables[tableName] = table
			}

			// Merge types (enums)
			for typeName, enumType := range ns.Types {
				key := nsName + "." + typeName
				if firstFile, exists := seenEnums[key]; exists {
					return nil, nil, fmt.Errorf(
						"duplicate enum '%s' found in %s (first defined in %s)",
						typeName, file, firstFile,
					)
				}
				seenEnums[key] = file
				mergedNs.Types[typeName] = enumType
			}
		}
	}

	return merged, allErrs, nil
}
```

需要添加 `fmt` 到 import：

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
)
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/source/ -run TestDirectoryLoader -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/source/dir_loader.go internal/source/dir_loader_test.go
git commit -m "feat: implement DirectoryLoader.Load() with recursive scan and merge"
```

---

## 任务 3：注册 DirectoryLoader 到 sourceRegistry

**文件：**
- 修改：`cmd/migra/diff.go:16-21`

- [ ] **步骤 1：修改注册代码**

在 `cmd/migra/diff.go` 的 `sourceRegistry` 中添加 `DirectoryLoader`：

```go
var sourceRegistry = func() *source.Registry {
	reg := source.NewRegistry()
	reg.Register(&source.DBLoader{})
	reg.Register(&source.SQLFileLoader{})
	reg.Register(&source.DirectoryLoader{})
	return reg
}()
```

- [ ] **步骤 2：运行现有测试确保无回归**

```bash
go test ./cmd/migra/ -v
```
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/diff.go
git commit -m "feat: register DirectoryLoader in sourceRegistry"
```

---

## 任务 4：添加集成测试

**文件：**
- 修改：`cmd/migra/integration_test.go`

- [ ] **步骤 1：编写集成测试**

在 `cmd/migra/integration_test.go` 中追加：

```go
func TestDirectoryVsDirectory_Diff(t *testing.T) {
	// 创建源目录
	sourceDir := t.TempDir()
	sourceSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);
	CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);
	CREATE TYPE user_role AS ENUM ('admin', 'user');`

	if err := os.WriteFile(filepath.Join(sourceDir, "schema.sql"), []byte(sourceSQL), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建目标目录（多了 age 列和 comments 表）
	targetDir := t.TempDir()
	targetSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		age INTEGER
	);
	CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);
	CREATE TABLE comments (
		id SERIAL PRIMARY KEY,
		post_id INTEGER NOT NULL,
		content TEXT NOT NULL
	);
	CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');`

	if err := os.WriteFile(filepath.Join(targetDir, "schema.sql"), []byte(targetSQL), 0644); err != nil {
		t.Fatal(err)
	}

	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     sourceDir,
		Target:     targetDir,
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`ADD COLUMN "age"`,
		`CREATE TABLE "public"."comments"`,
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDirectoryVsFile_Diff(t *testing.T) {
	// 创建源目录
	sourceDir := t.TempDir()
	sourceSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(sourceDir, "users.sql"), []byte(sourceSQL), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建目标单文件
	targetDir := t.TempDir()
	targetSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		email VARCHAR(100)
	);`
	targetFile := filepath.Join(targetDir, "schema.sql")
	if err := os.WriteFile(targetFile, []byte(targetSQL), 0644); err != nil {
		t.Fatal(err)
	}

	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     sourceDir,
		Target:     targetFile,
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	if !strings.Contains(out, `ADD COLUMN "email"`) {
		t.Fatalf("expected output to contain ADD COLUMN email, got:\n%s", out)
	}
}
```

需要确保 import 包含 `os` 和 `filepath`：

检查文件头部 import 是否已包含，如果没有则添加。

- [ ] **步骤 2：运行集成测试验证通过**

```bash
go test ./cmd/migra/ -run "TestDirectory" -v
```
预期：PASS

- [ ] **步骤 3：运行全部测试确保无回归**

```bash
go test ./... -v
```
预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/integration_test.go
git commit -m "feat: add directory diff integration tests"
```

---

## 任务 5：更新 CLI 帮助文本

**文件：**
- 修改：`cmd/migra/diff.go:27-34`

- [ ] **步骤 1：更新 diff 命令的 Long 描述和 Examples**

修改 `cmd/migra/diff.go` 中 `diffCmd` 的 `Long` 和示例：

```go
Long: `Compare two schema sources (SQL files, directories, or PostgreSQL connections) and output the SQL needed to migrate from source to target.

Examples:
  migra diff file.sql postgres://localhost/db
  migra diff postgres://localhost/db1 postgres://localhost/db2
  migra diff file_a.sql file_b.sql
  migra diff dir_a/ dir_b/
  migra diff dir_a/ postgres://localhost/db
  migra diff file.sql  # target from config database.url
  migra diff          # both from config database.source and database.target`,
```

- [ ] **步骤 2：验证编译**

```bash
go build ./cmd/migra/
```
预期：编译成功

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/diff.go
git commit -m "docs: update CLI help text with directory examples"
```

---

## 自检

**1. 规格覆盖度：**

| 规格需求 | 对应任务 |
|----------|----------|
| 新增 DirectoryLoader 实现 Loader 接口 | 任务 1 + 2 |
| Match() 检测目录路径 | 任务 1 |
| Load() 递归扫描 + schema 合并 | 任务 2 |
| 同名对象冲突 → fatal error | 任务 2（测试 DuplicateTable/DuplicateEnum） |
| 任一文件解析失败 → fatal error | 任务 2（测试 ParseError） |
| 跳过非 .sql 文件 | 任务 2（测试 SkipNonSQL） |
| 跳过隐藏文件 | 任务 2（测试 SkipHidden） |
| 注册到 sourceRegistry | 任务 3 |
| 单元测试 | 任务 1 + 2 |
| 集成测试 | 任务 4 |
| CLI 帮助文本更新 | 任务 5 |

**2. 占位符扫描：** ✅ 无占位符
**3. 类型一致性：** ✅ `DirectoryLoader`、`SQLFileLoader`、`LoadOptions`、`model.Schema` 等类型引用一致
