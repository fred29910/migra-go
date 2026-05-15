# Bug: JSON 格式输出与期望不一致

## 元信息

| 字段 | 值 |
|---|---|
| **发现日期** | 2026-05-15 |
| **严重程度** | Medium |
| **组件** | `internal/render/render.go` → `RenderJSON()` |
| **相关文档** | `testdata/README.md` → 场景 2：JSON 格式输出 |
| **状态** | Fixed |

---

## 问题描述

`migra diff --format json` 的实际输出与 `testdata/README.md` 中"场景 2：JSON 格式输出"描述的期望输出存在 **3 处结构性不一致**。

---

## 复现步骤

```bash
make build
./migra diff --format json testdata/example_source.sql testdata/example_target.sql
```

---

## 实际输出 vs 期望输出

### 实际输出（当前代码生成）

```json
[
  {
    "kind": "add_table",
    "object": {
      "Schema": "public",
      "Name": "comments",
      "Kind": "table",
      "Signature": ""
    },
    "destructive": false
  },
  {
    "kind": "add_column",
    "object": {
      "Schema": "public",
      "Name": "users.age",
      "Kind": "column",
      "Signature": ""
    },
    "destructive": false
  },
  {
    "kind": "add_enum_label",
    "object": {
      "Schema": "public",
      "Name": "user_role.guest",
      "Kind": "type",
      "Signature": ""
    },
    "destructive": false
  }
]
```

### 期望输出（README.md 定义）

```json
[
  {
    "kind": "add_table",
    "object_key": "public.comments",
    "destructive": false,
    "sql": "CREATE TABLE \"public\".\"comments\" ..."
  },
  {
    "kind": "add_column",
    "object_key": "public.users.age",
    "destructive": false,
    "sql": "ALTER TABLE \"public\".\"users\" ADD COLUMN \"age\" integer"
  },
  {
    "kind": "add_enum_label",
    "object_key": "public.user_role.guest",
    "destructive": false,
    "sql": "ALTER TYPE \"public\".\"user_role\" ADD VALUE 'guest'"
  }
]
```

---

## 差异分析

### 差异 1：字段名 `object` vs `object_key`

| | 代码实际 | 文档期望 |
|---|---|---|
| **JSON 字段名** | `"object"` | `"object_key"` |

**源码位置**：[`internal/render/render.go:348`](internal/render/render.go:348)

```go
type OpInfo struct {
    Kind        string          `json:"kind"`
    Object      model.ObjectKey `json:"object"`   // ← 期望 "object_key"
    Destructive bool            `json:"destructive"`
}
```

### 差异 2：`object` 字段值格式（嵌套对象 vs 扁平字符串）

| | 代码实际 | 文档期望 |
|---|---|---|
| **值类型** | 嵌套 Object 结构 | 扁平字符串 |
| **示例** | `{"Schema": "public", "Name": "comments", "Kind": "table", "Signature": ""}` | `"public.comments"` |

**源码位置**：[`internal/render/render.go:349`](internal/render/render.go:349)

`RenderJSON` 直接将 `model.ObjectKey` 结构体序列化为 JSON 对象。`ObjectKey` 结构体定义在 [`internal/model/object_key.go:17-22`](internal/model/object_key.go:17)：

```go
type ObjectKey struct {
    Schema    string
    Name      string
    Kind      ObjectKind
    Signature string
}
```

由于 Go 的 `encoding/json` 默认导出所有公开字段，序列化结果为嵌套对象。期望格式是 `"schema.name"` 形式的扁平字符串。

### 差异 3：缺少 `sql` 字段

| | 代码实际 | 文档期望 |
|---|---|---|
| **sql 字段** | ❌ 不存在 | ✅ 包含渲染后的 SQL 语句 |

**源码位置**：[`internal/render/render.go:347-351`](internal/render/render.go:347)

`OpInfo` 结构体中完全没有 `sql` 字段。`RenderJSON` 函数只提取了 `kind`、`object`、`destructive` 三个属性，没有调用 `Renderer.Render()` 或 `Renderer.RenderSingle()` 来生成 SQL 文本。

---

## 根因分析

`RenderJSON` 函数在设计时未与文档规范对齐：

1. **字段命名不一致**：`OpInfo` 的 JSON tag 使用了 `"object"`，而文档规范要求 `"object_key"`
2. **序列化策略不当**：直接将 `model.ObjectKey` 结构体作为 JSON 值序列化，导致输出嵌套对象而非扁平字符串
3. **功能缺失**：未将 SQL 渲染结果包含在 JSON 输出中，导致 JSON 格式丢失了关键的 DDL 语句信息

这三个问题都源于同一个根因：**`RenderJSON` 的实现是早期版本，未随文档规范更新**。

---

## 修复建议

修改 `internal/render/render.go` 中的 `RenderJSON` 函数：

1. 将 `OpInfo` 结构体的 `Object` 字段改为字符串类型的 `ObjectKey`，JSON tag 改为 `"object_key"`
2. 在构造 `OpInfo` 时，将 `model.ObjectKey` 格式化为 `"schema.name"` 字符串
3. 添加 `SQL string` 字段（JSON tag: `"sql"`），调用 `Renderer.RenderSingle(op)` 生成 SQL

```go
type OpInfo struct {
    Kind        string `json:"kind"`
    ObjectKey   string `json:"object_key"`
    Destructive bool   `json:"destructive"`
    SQL         string `json:"sql"`
}
```

---

## 影响范围

- **受影响命令**：`migra diff --format json`
- **受影响测试**：`internal/render/render_test.go:TestRenderOutput_SupportsSQLAndJSON`（当前测试只检查 `"kind"` 字段存在，未验证完整结构）
- **下游消费者**：任何解析 migra JSON 输出的自动化工具或 CI/CD 流水线

---

## 相关文件

| 文件 | 说明 |
|---|---|
| [`internal/render/render.go`](../internal/render/render.go) | `RenderJSON` 实现 |
| [`internal/render/render_test.go`](../internal/render/render_test.go) | 渲染测试 |
| [`internal/model/object_key.go`](../internal/model/object_key.go) | `ObjectKey` 结构体定义 |
| [`testdata/README.md`](../../testdata/README.md) | 场景 2 期望输出定义 |

---

## 修复记录

### 修复时间

2026-05-15

### 修改文件

| 文件 | 变更说明 |
|------|----------|
| [`internal/render/render.go`](../internal/render/render.go) | `OpInfo.Object` → `OpInfo.ObjectKey`（`string` 类型，JSON tag `"object_key"`）；新增 `SQL string` 字段（JSON tag `"sql"`）；`RenderJSON` 创建 Renderer 实例调用 `RenderSingle(op)` 生成 SQL |
| [`internal/render/render_test.go`](../internal/render/render_test.go) | 扩展 `TestRenderOutput_SupportsSQLAndJSON`，验证 `"object_key"` 和 `"sql"` 字段存在，验证 `"public.idx_a"` 扁平值 |
| [`docs/bugs/2026-05-15-json-format-output-mismatch.md`](2026-05-15-json-format-output-mismatch.md) | 状态更新为 Fixed，添加修复记录 |

### 改动要点

1. `OpInfo.ObjectKey` 改为 `string` 类型，值为 `schema.name` 格式的扁平字符串
2. 添加 `OpInfo.SQL` 字段，值为对应操作的 SQL DDL 语句
3. `RenderJSON` 内部创建 `Renderer` 实例以复用已实现的渲染逻辑
