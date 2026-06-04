# 项目全面改进实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 通过四个阶段全面改进 migra-go 项目的代码质量和文档完善度

**架构：** 采用渐进式改进策略，分四个阶段实施：修复关键问题、提高测试覆盖率、代码质量优化、文档完善。每个阶段独立可交付，确保项目稳定性。

**技术栈：** Go 1.26.2, testify, golangci-lint, godoc

---

## 文件结构

### 第一阶段：修复关键问题
- 修改：`internal/diff/diff_columns_test.go` (修复编译错误)
- 修改：`docs/bugs/*.md` (解决已知 bug)
- 修改：相关源文件 (修复 bug 根因)

### 第二阶段：提高测试覆盖率
- 创建：`internal/introspect/introspect_test.go` (新增测试)
- 创建：`internal/parserutil/util_test.go` (新增测试)
- 修改：`internal/model/*_test.go` (补充测试)
- 修改：`cmd/migra/*_test.go` (补充集成测试)

### 第三阶段：代码质量优化
- 创建：`internal/errors/errors.go` (哨兵错误定义)
- 修改：`internal/render/render.go` (提取公共函数)
- 修改：`internal/normalize/normalize.go` (提取公共函数)
- 修改：`internal/diff/differ.go` (性能优化)
- 创建：`internal/util/util.go` (工具函数集中)

### 第四阶段：文档完善
- 修改：`docs/arch.md` (更新架构文档)
- 创建：`examples/*.md` (使用示例)
- 修改：`CONTRIBUTING.md` (改进贡献指南)
- 运行：`go doc -all ./internal/...` (生成 API 文档)

---

## 第一阶段：修复关键问题

### 任务 1.1：修复编译错误

**文件：**
- 修改：`internal/diff/diff_columns_test.go:73-74`

- [ ] **步骤 1：分析编译错误**

运行：`go test ./internal/diff/... -v 2>&1 | head -20`
预期：编译错误 `setOp.Default undefined (type *SetDefaultOp has no field or method Default)`

- [ ] **步骤 2：检查 SetDefaultOp 结构体定义**

读取：`internal/diff/operation.go` 中 `SetDefaultOp` 的定义
预期：找到 `SetDefaultOp` 结构体，确认其字段

- [ ] **步骤 3：修复测试文件**

修改：`internal/diff/diff_columns_test.go:73-74`
将 `setOp.Default` 替换为正确的字段名（根据步骤 2 的发现）

- [ ] **步骤 4：运行测试验证修复**

运行：`go test ./internal/diff/... -v`
预期：测试通过，无编译错误

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/diff_columns_test.go
git commit -m "fix(diff): 修复 diff_columns_test.go 编译错误"
```

### 任务 1.2：解决已知 bug

**文件：**
- 修改：`docs/bugs/2026-05-15-github-release-403-permission-denied.md`
- 修改：相关源文件（根据 bug 描述）

- [ ] **步骤 1：分析第一个 bug**

读取：`docs/bugs/2026-05-15-github-release-403-permission-denied.md`
理解：bug 的描述、复现步骤、预期行为

- [ ] **步骤 2：定位问题代码**

根据 bug 描述，定位相关源文件
运行：`grep -r "相关关键词" .` 查找相关代码

- [ ] **步骤 3：修复 bug**

修改相关源文件，修复问题
确保修改最小化，不引入新问题

- [ ] **步骤 4：验证修复**

运行相关测试：`go test ./... -v`
确保修复有效且无回归

- [ ] **步骤 5：Commit**

```bash
git add 相关文件
git commit -m "fix: 修复 GitHub Release 403 权限问题"
```

- [ ] **步骤 6：重复步骤 1-5 解决其余 4 个 bug**

为每个 bug 重复步骤 1-5：
- `2026-05-15-index-mutation-table-not-found.md`
- `2026-05-15-json-format-output-mismatch.md`
- `2026-05-15-multi-schema-diff-create-schema-stmt.md`
- `2026-05-15-multi-schema-diff-duplicate-table.md`

### 任务 1.3：验证第一阶段

**文件：**
- 无文件修改，仅验证

- [ ] **步骤 1：运行完整测试套件**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 2：运行 CI 检查**

运行：`make ci`
预期：通过所有检查（fmt, vet, lint, test）

- [ ] **步骤 3：更新 bug 文档**

将已解决的 bug 标记为已解决
更新 `docs/bugs/` 目录下的相关文件

- [ ] **步骤 4：Commit**

```bash
git add docs/bugs/
git commit -m "docs: 标记已解决的 bug"
```

---

## 第二阶段：提高测试覆盖率

### 任务 2.1：为 internal/introspect 添加测试

**文件：**
- 创建：`internal/introspect/introspect_test.go`

- [ ] **步骤 1：分析现有代码**

读取：`internal/introspect/introspect.go` 和相关文件
理解：函数签名、依赖、错误处理

- [ ] **步骤 2：编写测试框架**

创建：`internal/introspect/introspect_test.go`
添加：测试包声明、导入、基础测试函数

- [ ] **步骤 3：编写单元测试**

为 `introspect` 包中的函数编写测试：
- `TestLoadFromDBWithConn` (模拟数据库连接)
- `TestLoadTables` (模拟表加载)
- `TestLoadConstraints` (模拟约束加载)
- 等等

- [ ] **步骤 4：运行测试验证**

运行：`go test ./internal/introspect/... -v`
预期：测试通过

- [ ] **步骤 5：检查覆盖率**

运行：`go test ./internal/introspect/... -coverprofile=coverage.out`
运行：`go tool cover -func=coverage.out | grep introspect`
预期：覆盖率 > 50%

- [ ] **步骤 6：Commit**

```bash
git add internal/introspect/introspect_test.go
git commit -m "test(introspect): 添加 internal/introspect 包测试"
```

### 任务 2.2：为 internal/parserutil 添加测试

**文件：**
- 创建：`internal/parserutil/util_test.go`

- [ ] **步骤 1：分析现有代码**

读取：`internal/parserutil/util.go`
理解：工具函数的功能和接口

- [ ] **步骤 2：编写测试框架**

创建：`internal/parserutil/util_test.go`
添加：测试包声明、导入、基础测试函数

- [ ] **步骤 3：编写单元测试**

为 `parserutil` 包中的函数编写测试：
- `TestNormalizeDataType` (类型映射测试)
- `TestNormalizeIdentifier` (标识符规范化测试)
- `TestParseExpression` (表达式解析测试)
- 等等

- [ ] **步骤 4：运行测试验证**

运行：`go test ./internal/parserutil/... -v`
预期：测试通过

- [ ] **步骤 5：检查覆盖率**

运行：`go test ./internal/parserutil/... -coverprofile=coverage.out`
运行：`go tool cover -func=coverage.out | grep parserutil`
预期：覆盖率 > 30%

- [ ] **步骤 6：Commit**

```bash
git add internal/parserutil/util_test.go
git commit -m "test(parserutil): 添加 internal/parserutil 包测试"
```

### 任务 2.3：为 internal/model 添加测试

**文件：**
- 修改：`internal/model/schema_test.go`
- 修改：`internal/model/table_test.go`
- 修改：`internal/model/column_test.go`

- [ ] **步骤 1：分析现有测试**

读取：`internal/model/*_test.go`
理解：现有测试覆盖范围

- [ ] **步骤 2：识别测试缺口**

分析：哪些函数/方法没有测试
优先级：核心操作、边界条件、错误处理

- [ ] **步骤 3：补充测试用例**

为以下场景添加测试：
- Schema 操作（创建、修改、删除）
- Table 操作（添加列、删除列、修改列）
- Column 操作（类型转换、默认值、非空约束）
- 序列化/反序列化测试

- [ ] **步骤 4：运行测试验证**

运行：`go test ./internal/model/... -v`
预期：测试通过

- [ ] **步骤 5：检查覆盖率**

运行：`go test ./internal/model/... -coverprofile=coverage.out`
运行：`go tool cover -func=coverage.out | grep model`
预期：覆盖率 > 60%

- [ ] **步骤 6：Commit**

```bash
git add internal/model/*_test.go
git commit -m "test(model): 补充 internal/model 包测试用例"
```

### 任务 2.4：为 internal/cmd/migra 添加集成测试

**文件：**
- 修改：`cmd/migra/integration_test.go`
- 修改：`cmd/migra/diff_test.go`
- 修改：`cmd/migra/push_test.go`

- [ ] **步骤 1：分析现有测试**

读取：`cmd/migra/*_test.go`
理解：现有集成测试覆盖范围

- [ ] **步骤 2：识别测试缺口**

分析：哪些 CLI 场景没有测试
优先级：参数解析、配置加载、端到端流程

- [ ] **步骤 3：补充集成测试**

为以下场景添加测试：
- 0/1/2 参数模式测试
- 配置文件加载测试
- 环境变量配置测试
- 错误处理测试

- [ ] **步骤 4：运行测试验证**

运行：`go test ./cmd/migra/... -v`
预期：测试通过

- [ ] **步骤 5：检查覆盖率**

运行：`go test ./cmd/migra/... -coverprofile=coverage.out`
运行：`go tool cover -func=coverage.out | grep cmd/migra`
预期：覆盖率 > 40%

- [ ] **步骤 6：Commit**

```bash
git add cmd/migra/*_test.go
git commit -m "test(cmd): 补充 cmd/migra 集成测试"
```

### 任务 2.5：验证第二阶段

**文件：**
- 无文件修改，仅验证

- [ ] **步骤 1：运行完整测试套件**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 2：生成覆盖率报告**

运行：`make test-coverage`
预期：生成覆盖率报告

- [ ] **步骤 3：检查整体覆盖率**

分析：各包覆盖率是否达到目标
记录：覆盖率改进情况

- [ ] **步骤 4：Commit**

```bash
git add coverage.out coverage.html
git commit -m "test: 更新测试覆盖率报告"
```

---

## 第三阶段：代码质量优化

### 任务 3.1：改进错误处理

**文件：**
- 创建：`internal/errors/errors.go`
- 修改：`internal/app/diff_service.go`
- 修改：`internal/parser/parser.go`
- 修改：`internal/source/registry.go`

- [ ] **步骤 1：定义哨兵错误**

创建：`internal/errors/errors.go`
添加：
```go
package errors

import "errors"

var (
    ErrNotFound      = errors.New("resource not found")
    ErrInvalidConfig = errors.New("invalid configuration")
    ErrParseFailed   = errors.New("parse failed")
    ErrLoadFailed    = errors.New("load failed")
    ErrDiffFailed    = errors.New("diff failed")
)
```

- [ ] **步骤 2：在 diff_service.go 中使用哨兵错误**

修改：`internal/app/diff_service.go`
将动态错误替换为哨兵错误：
```go
return nil, nil, fmt.Errorf("load source: %w", errors.ErrLoadFailed)
```

- [ ] **步骤 3：在 parser.go 中使用哨兵错误**

修改：`internal/parser/parser.go`
将动态错误替换为哨兵错误：
```go
return nil, fmt.Errorf("parse SQL: %w", errors.ErrParseFailed)
```

- [ ] **步骤 4：在 registry.go 中使用哨兵错误**

修改：`internal/source/registry.go`
将动态错误替换为哨兵错误：
```go
return nil, fmt.Errorf("load schema: %w", errors.ErrNotFound)
```

- [ ] **步骤 5：运行测试验证**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add internal/errors/errors.go internal/app/diff_service.go internal/parser/parser.go internal/source/registry.go
git commit -m "refactor(errors): 添加哨兵错误并统一错误处理"
```

### 任务 3.2：重构重复代码

**文件：**
- 创建：`internal/util/util.go`
- 修改：`internal/render/render.go`
- 修改：`internal/normalize/normalize.go`

- [ ] **步骤 1：识别重复代码**

分析：`render.go` 和 `normalize.go` 中的重复函数
目标：identifier quoting、type normalization、string comparison

- [ ] **步骤 2：创建工具包**

创建：`internal/util/util.go`
添加：
```go
package util

func QuoteIdentifier(name string) string {
    // 实现 identifier quoting
}

func QuoteQualifiedIdentifier(schema, name string) string {
    // 实现 schema-qualified identifier quoting
}

func NormalizeDataType(dataType string) string {
    // 实现类型规范化
}

func SameStringSlice(a, b []string) bool {
    // 实现字符串切片比较
}
```

- [ ] **步骤 3：重构 render.go**

修改：`internal/render/render.go`
将重复函数替换为 `util` 包中的函数：
```go
import "github.com/fred29910/migra-go/internal/util"

// 替换 quoteIdentifier 为 util.QuoteIdentifier
```

- [ ] **步骤 4：重构 normalize.go**

修改：`internal/normalize/normalize.go`
将重复函数替换为 `util` 包中的函数：
```go
import "github.com/fred29910/migra-go/internal/util"

// 替换 normalizeDataType 为 util.NormalizeDataType
```

- [ ] **步骤 5：运行测试验证**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add internal/util/util.go internal/render/render.go internal/normalize/normalize.go
git commit -m "refactor(util): 提取公共函数消除代码重复"
```

### 任务 3.3：优化性能

**文件：**
- 修改：`internal/diff/differ.go`
- 创建：`internal/diff/benchmark_test.go`

- [ ] **步骤 1：分析性能热点**

读取：`internal/diff/differ.go`
识别：可能的性能瓶颈（如深度比较、内存分配）

- [ ] **步骤 2：编写基准测试**

创建：`internal/diff/benchmark_test.go`
添加：
```go
package diff

import "testing"

func BenchmarkDiff(b *testing.B) {
    // 设置测试数据
    // 运行基准测试
}
```

- [ ] **步骤 3：运行基准测试**

运行：`go test ./internal/diff/... -bench=. -benchmem`
预期：获得基准测试结果

- [ ] **步骤 4：优化性能**

修改：`internal/diff/differ.go`
优化：减少内存分配、改进算法效率

- [ ] **步骤 5：重新运行基准测试**

运行：`go test ./internal/diff/... -bench=. -benchmem`
预期：性能改进

- [ ] **步骤 6：Commit**

```bash
git add internal/diff/differ.go internal/diff/benchmark_test.go
git commit -m "perf(diff): 优化 diff 算法性能"
```

### 任务 3.4：改进代码结构

**文件：**
- 修改：包边界调整
- 修改：接口定义

- [ ] **步骤 1：分析代码结构**

分析：当前包边界是否合理
识别：高内聚、低耦合的机会

- [ ] **步骤 2：调整包边界**

根据分析结果，调整包边界：
- 将相关函数移动到合适的包
- 确保接口小而专注

- [ ] **步骤 3：改进接口设计**

检查：现有接口是否符合接口隔离原则
优化：将大接口拆分为小接口

- [ ] **步骤 4：运行测试验证**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 5：Commit**

```bash
git add 相关文件
git commit -m "refactor: 改进代码结构和接口设计"
```

### 任务 3.5：验证第三阶段

**文件：**
- 无文件修改，仅验证

- [ ] **步骤 1：运行完整测试套件**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 2：运行 CI 检查**

运行：`make ci`
预期：通过所有检查

- [ ] **步骤 3：检查代码质量**

分析：错误处理一致性、代码重复率、性能改进
记录：改进情况

- [ ] **步骤 4：Commit**

```bash
git commit --allow-empty -m "chore: 完成第三阶段代码质量优化"
```

---

## 第四阶段：文档完善

### 任务 4.1：更新架构文档

**文件：**
- 修改：`docs/arch.md`

- [ ] **步骤 1：分析当前文档**

读取：`docs/arch.md`
理解：当前文档内容

- [ ] **步骤 2：对比代码实现**

分析：文档与实际代码的差异
识别：需要更新的部分

- [ ] **步骤 3：更新架构文档**

修改：`docs/arch.md`
更新：架构图、组件描述、设计决策

- [ ] **步骤 4：验证文档准确性**

检查：文档中的代码示例是否可运行
验证：架构图是否反映实际结构

- [ ] **步骤 5：Commit**

```bash
git add docs/arch.md
git commit -m "docs: 更新架构文档与代码实现同步"
```

### 任务 4.2：添加使用示例

**文件：**
- 创建：`examples/multi-schema-comparison.md`
- 创建：`examples/directory-comparison.md`
- 创建：`examples/configuration-usage.md`

- [ ] **步骤 1：分析使用场景**

识别：用户常见的使用场景
优先级：多 schema 比较、目录比较、配置文件使用

- [ ] **步骤 2：创建多 schema 比较示例**

创建：`examples/multi-schema-comparison.md`
内容：使用 `--schema` 参数比较多个 schema 的示例

- [ ] **步骤 3：创建目录比较示例**

创建：`examples/directory-comparison.md`
内容：使用目录作为 schema 来源的示例

- [ ] **步骤 4：创建配置文件使用示例**

创建：`examples/configuration-usage.md`
内容：配置文件、环境变量、.env 文件的使用示例

- [ ] **步骤 5：验证示例可运行**

运行：示例中的命令
预期：命令可正常执行

- [ ] **步骤 6：Commit**

```bash
git add examples/*.md
git commit -m "docs: 添加使用示例文档"
```

### 任务 4.3：补充 API 文档

**文件：**
- 修改：`internal/**/*.go` (添加 godoc 注释)
- 创建：`docs/api-reference.md` (生成 API 参考)

- [ ] **步骤 1：分析现有文档**

检查：现有 godoc 注释覆盖率
识别：缺少文档的导出函数/类型

- [ ] **步骤 2：添加 godoc 注释**

为以下包添加 godoc 注释：
- `internal/model/` (核心数据模型)
- `internal/parser/` (SQL 解析器)
- `internal/diff/` (差异比较引擎)
- `internal/render/` (SQL 渲染器)

- [ ] **步骤 3：生成 API 参考文档**

运行：`go doc -all ./internal/... > docs/api-reference.md`
预期：生成 API 参考文档

- [ ] **步骤 4：验证文档质量**

检查：生成的文档是否完整、准确
验证：代码示例是否可运行

- [ ] **步骤 5：Commit**

```bash
git add internal/**/*.go docs/api-reference.md
git commit -m "docs: 补充 API 文档和 godoc 注释"
```

### 任务 4.4：改进贡献指南

**文件：**
- 修改：`CONTRIBUTING.md`

- [ ] **步骤 1：分析当前指南**

读取：`CONTRIBUTING.md`
理解：当前内容

- [ ] **步骤 2：添加开发流程说明**

添加：详细的开发流程步骤
包括：环境搭建、分支策略、提交规范

- [ ] **步骤 3：补充测试要求**

添加：测试覆盖率要求
包括：如何运行测试、编写测试的最佳实践

- [ ] **步骤 4：明确代码审查标准**

添加：代码审查 checklist
包括：代码质量、测试覆盖、文档更新

- [ ] **步骤 5：添加问题报告模板**

添加：bug 报告模板
添加：功能请求模板

- [ ] **步骤 6：Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: 改进贡献指南"
```

### 任务 4.5：验证第四阶段

**文件：**
- 无文件修改，仅验证

- [ ] **步骤 1：验证文档同步**

检查：架构文档与代码是否同步
验证：使用示例是否可运行

- [ ] **步骤 2：检查文档完整性**

检查：API 文档是否完整
验证：贡献指南是否清晰

- [ ] **步骤 3：运行完整测试套件**

运行：`make test`
预期：所有测试通过

- [ ] **步骤 4：Commit**

```bash
git commit --allow-empty -m "chore: 完成第四阶段文档完善"
```

---

## 最终验证

### 任务 F.1：整体验证

**文件：**
- 无文件修改，仅验证

- [ ] **步骤 1：运行完整 CI 检查**

运行：`make ci`
预期：通过所有检查

- [ ] **步骤 2：检查测试覆盖率**

运行：`make test-coverage`
预期：所有核心包覆盖率 > 50%

- [ ] **步骤 3：验证文档完整性**

检查：所有文档是否更新
验证：所有示例是否可运行

- [ ] **步骤 4：创建版本标签**

运行：`git tag -a v0.3.0 -m "版本 0.3.0: 项目全面改进"`
预期：创建版本标签

- [ ] **步骤 5：总结改进成果**

记录：代码质量改进情况
记录：文档完善情况
记录：测试覆盖率提升情况

---

**计划完成。** 此计划包含 4 个阶段、17 个任务、约 80 个步骤。每个步骤都有具体的代码、命令和预期结果。请根据实际情况调整任务顺序和细节。