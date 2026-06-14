# MIGRA-Go API 参考文档

> 最后更新：2026-06-05

本文档是 MIGRA-Go 内部包的 API 参考文档。

## 包结构

### 核心包

| 包名 | 说明 | 主要功能 |
|------|------|----------|
| `internal/model` | 数据模型 | Schema、Table、Column、View、Sequence、Extension 等核心数据结构 |
| `internal/diff` | 差异比较引擎 | 比较两个 Schema 的差异，生成 33 种 Operation |
| `internal/plan` | 执行计划 | 三阶段执行计划（Pre-deploy/Deploy/Post-deploy），DAG 拓扑排序 |
| `internal/render` | SQL/JSON 渲染器 | 将 Operation 渲染为可执行的 SQL 或 JSON 格式 |

### 解析器包

| 包名 | 说明 | 主要功能 |
|------|------|----------|
| `internal/parser` | SQL 解析器 | 基于 pg_query_go 解析 AST，通过 Handler 模式转换 DDL |
| `internal/parser/parserutil` | 解析器工具 | 类型映射、表达式解析、列解析等辅助函数 |

### 数据源包

| 包名 | 说明 | 主要功能 |
|------|------|----------|
| `internal/source` | 数据源加载 | 统一加载接口，支持 SQL 文件、目录、数据库三种来源 |
| `internal/introspect` | 数据库内省 | 从 PostgreSQL 实例读取 Schema 结构 |

### 工具包

| 包名 | 说明 | 主要功能 |
|------|------|----------|
| `internal/normalize` | 语义归一化 | 同义类型映射（如 int4 → integer），减少误报 |
| `internal/errors` | 错误定义 | 哨兵错误定义，统一错误处理 |
| `internal/util` | 工具函数 | 通用工具函数（QuoteIdentifier、NormalizeDataType 等） |
| `internal/indexdef` | 索引定义解析 | 解析索引定义字符串 |
| `internal/version` | 版本信息 | 版本号、构建时间等元信息 |

### 应用层包

| 包名 | 说明 | 主要功能 |
|------|------|----------|
| `internal/app` | 应用服务 | 依赖注入编排，DiffService 接口 |
| `internal/testutil` | 测试工具 | Schema 构建辅助函数 |

## 核心类型

### model.Schema

```go
type Schema struct {
    Schemas    map[string]*Namespace
    // ... 其他字段
}
```

表示完整的数据库 Schema 结构，包含多个命名空间。

### diff.Operation

```go
type Operation interface {
    Kind() string
    ObjectKey() string
    IsDestructive() bool
}
```

差异操作接口，所有 33 种操作类型都实现此接口。

### plan.Engine

```go
type Engine interface {
    Plan(ops []Operation) map[Stage][]Operation
}
```

执行计划引擎接口，将 Operation 分组为三个阶段。

### render.SQLEngine

```go
type SQLEngine interface {
    RenderOutput(ops []Operation, format string) (string, error)
}
```

SQL 渲染引擎接口，将 Operation 渲染为 SQL 或 JSON。

## 主要函数

### diff.Differ

```go
func NewDiffer() *Differ
func (d *Differ) Diff(source, target *model.Schema) ([]Operation, []string)
```

差异比较器，比较两个 Schema 并返回 Operation 列表。

### plan.Planner

```go
func NewPlanner(unsafeDrop bool) *Planner
func (p *Planner) Plan(ops []Operation) map[Stage][]Operation
```

执行计划器，将 Operation 分组为三个阶段。

### render.Renderer

```go
func NewRenderer() *Renderer
func (r *Renderer) RenderOutput(ops []Operation, format string) (string, error)
```

渲染器，将 Operation 渲染为 SQL 或 JSON。

### source.Loader

```go
type Loader interface {
    Match(source string) bool
    Load(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
}
```

数据源加载器接口，支持 SQL 文件、目录、数据库三种来源。

### normalize.CanonicalizeSchema

```go
func CanonicalizeSchema(schema *model.Schema) error
```

语义归一化，将同义类型映射为标准形式。

## 错误类型

### errors 包

```go
var (
    ErrNotFound      = errors.New("resource not found")
    ErrInvalidConfig = errors.New("invalid configuration")
    ErrParseFailed   = errors.New("parse failed")
    ErrLoadFailed    = errors.New("load failed")
    ErrDiffFailed    = errors.New("diff failed")
)
```

哨兵错误定义，用于统一错误处理。

## 配置项

### app.Config

```go
type Config struct {
    Source     string
    Target     string
    Schemas    []string
    Format     string
    UnsafeDrop bool
    Strict     bool
    Timeout    time.Duration
}
```

应用配置结构体，包含所有配置项。

## 使用示例

### 基本差异比较

```go
package main

import (
    "github.com/fred29910/migra-go/internal/diff"
    "github.com/fred29910/migra-go/internal/model"
)

func main() {
    // 创建 Schema 对象
    source := &model.Schema{}
    target := &model.Schema{}
    
    // 比较差异
    differ := diff.NewDiffer()
    operations, warnings := differ.Diff(source, target)
    
    // 处理结果
    for _, op := range operations {
        fmt.Printf("Operation: %s\n", op.Kind())
    }
}
```

### 渲染 SQL

```go
package main

import (
    "github.com/fred29910/migra-go/internal/diff"
    "github.com/fred29910/migra-go/internal/render"
)

func main() {
    // 假设已获取 operations
    var operations []diff.Operation
    
    // 渲染为 SQL
    renderer := render.NewRenderer()
    sql, err := renderer.RenderOutput(operations, "sql")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(sql)
}
```

## 相关文档

- [README.md](../README.md) - 项目概述和完整功能列表
- [arch.md](./arch.md) - 架构设计文档
- [DDL.md](./DDL.md) - 支持的 DDL 类型矩阵