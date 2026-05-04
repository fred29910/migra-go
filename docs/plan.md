# MIGRA-Go 实施方案（Merged）

## 1. 目标与范围

### 1.1 总体目标
构建一个可维护、可测试、可扩展的 Go 工具，实现以下核心链路：

1. `SQL 文件 -> SchemaModel`
2. `PostgreSQL 实例 -> SchemaModel`
3. `SchemaModel A vs B -> 结构化 DiffOp`
4. `DiffOp -> 可执行迁移 SQL`

### 1.2 MVP 范围（必须先做）

MVP 仅覆盖：

1. schema/table/column
2. primary key
3. enum type
4. normal index（含唯一索引）
5. 非外键类常见约束（not null/check/default）

不在 MVP 内（延后）：

1. view/function/trigger/rule
2. rollback 自动生成
3. 跨数据库方言支持
4. 图形化输出（DOT/LSP）

这样能优先保证“结果正确 + 可上线试用”。

## 2. 架构与目录

```text
/cmd/schemadiff
/internal/model
/internal/parser
/internal/introspect
/internal/normalize
/internal/diff
/internal/plan
/internal/render
/internal/testutil
```

模块职责：

1. `model`: 中间结构、对象 ID、序列化
2. `parser`: SQL AST -> model（优先 DDL 子集）
3. `introspect`: pg_catalog/information_schema -> model
4. `normalize`: 语义归一化，减少“假 diff”
5. `diff`: 生成强类型 DiffOp
6. `plan`: 依赖图、拓扑排序、分阶段执行策略
7. `render`: DiffOp -> SQL

## 3. 数据模型设计（关键）

### 3.1 对象标识

禁止仅用 `map[string]*Obj`；统一用可唯一标识对象的 Key：

```go
type ObjectKey struct {
    Schema    string
    Name      string
    Kind      ObjectKind
    Signature string // function 用参数签名；其他对象可为空
}
```

函数重载、跨 schema 同名对象由 `Signature + Schema` 消歧。

### 3.2 SchemaModel（MVP）

```go
type Schema struct {
    Schemas map[string]*Namespace
}

type Namespace struct {
    Name   string
    Tables map[string]*Table
    Types  map[string]*EnumType
}

type Table struct {
    Schema      string
    Name        string
    Columns     []*Column          // 保留顺序，支持列顺序相关策略
    PrimaryKey  *PrimaryKey
    Constraints map[string]*Constraint
    Indexes     map[string]*Index
}

type Column struct {
    Name         string
    DataType     string
    IsNullable   bool
    DefaultExpr  *string
    IsIdentity   bool
    IdentityKind string // ALWAYS/BY DEFAULT
}
```

设计原则：

1. `json` 可稳定序列化（用于 golden）
2. 字段表达“语义”而非“展示文本”
3. 可来自 parser/introspect 两种输入

## 4. 归一化策略（normalize）

先归一化，再 diff。否则误报会非常多。

必须实现：

1. 类型同义归一：`int4 -> integer`，`bool -> boolean`
2. 默认值表达式归一：去除无关括号、schema 前缀噪声
3. 标识符归一：quoted/unquoted 一致化
4. 约束定义归一：空白、大小写、冗余 cast

建议接口：

```go
func CanonicalizeSchema(s *model.Schema) (*model.Schema, error)
```

## 5. Parser 方案

### 5.1 解析路线

优先采用 `pg_query_go`：

1. SQL -> PostgreSQL AST
2. Visitor 提取 DDL 语义
3. 写入 SchemaModel

MVP 支持节点：

1. `CreateStmt`（CREATE TABLE）
2. `AlterTableStmt`（列新增/变更、约束）
3. `IndexStmt`（CREATE INDEX）
4. `CreateEnumStmt`（CREATE TYPE ... AS ENUM）

后续扩展节点：

1. `CreateFunctionStmt`
2. `CreateViewStmt`

### 5.2 工程注意事项

1. 明确 `cgo` 依赖，CI 中验证 Linux/macOS 构建
2. parser 错误需携带 statement 位置，便于定位
3. 对暂不支持语句返回“可读错误”，不要 silent skip

## 6. Introspection 方案

### 6.1 数据来源

组合使用：

1. `pg_catalog`（完整性高）
2. `information_schema`（可读性好）

避免只限定 `public`，支持 `--schema` 参数（可多值）。

### 6.2 关键覆盖项（MVP）

1. 表/列/ordinal/default/nullable
2. PK/unique/check 约束
3. enum labels（按 sort order）
4. index definition（含唯一、表达式索引标记）

建议接口：

```go
type LoadOptions struct {
    Schemas []string
}

func LoadFromDB(ctx context.Context, conn *pgx.Conn, opt LoadOptions) (*model.Schema, error)
```

## 7. Diff 引擎（强类型）

禁止 `map[string]string` 弱类型细节。

建议：

```go
type Operation interface {
    Kind() Kind
    ObjectKey() model.ObjectKey
}

type AddTableOp struct { Table model.Table }
type DropTableOp struct { Schema, Name string }
type AddColumnOp struct { Schema, Table string; Column model.Column }
type AlterColumnTypeOp struct { Schema, Table, Column, From, To string }
type SetNotNullOp struct { Schema, Table, Column string }
type DropNotNullOp struct { Schema, Table, Column string }
type CreateIndexOp struct { Index model.Index }
type DropIndexOp struct { Schema, Name string }
```

Diff 原则：

1. 显式区分破坏性与非破坏性操作
2. 避免把 rename 误判为 drop+add（MVP 可先不做 rename 自动识别，但要标注风险）
3. 同一对象只产生最小必要操作集

## 8. 迁移计划与排序（plan）

不是简单线性规则，而是依赖图。

执行策略：

1. 构建 `Operation DAG`
2. 拓扑排序
3. 对可能循环依赖的约束使用延迟策略（先建对象后补约束）

分阶段输出建议：

1. pre-deploy（新增对象）
2. deploy（结构修改）
3. post-deploy（高风险 drop，可人工确认）

## 9. SQL 渲染（render）

要求：

1. 输出幂等友好 SQL（尽可能使用 `IF EXISTS/IF NOT EXISTS`，视语义而定）
2. 标准化引用方式（schema-qualified）
3. 注释中标识操作类型与风险等级

示例输出：

```sql
-- op: add_column risk:low
ALTER TABLE public.users ADD COLUMN age integer;
```

## 10. CLI 设计

命令形态：

1. `schemadiff file.sql postgres://...`
2. `schemadiff postgres://a postgres://b`
3. `schemadiff file_a.sql file_b.sql`

建议参数：

1. `--schema public,app`
2. `--format sql|json`
3. `--unsafe-drop`（默认 false）
4. `--strict`（遇到未支持语句即失败）

## 11. 测试与验收

### 11.1 测试层次

1. 单元测试：normalize/diff/render
2. Golden 测试：`SQL -> model.json`
3. 集成测试：Docker PG introspection
4. Roundtrip：`A -> diff -> apply -> B`，二次 diff 必须为空

### 11.2 MVP 验收标准

1. 在至少 20 组 DDL 用例下，roundtrip 通过率 100%
2. 同义 SQL 场景误报率为 0（基于基准用例集）
3. CLI 对未支持语句能明确报错，不 silent ignore
4. 生成 SQL 可在 PostgreSQL 15/16/17 上执行

## 12. 里程碑计划（建议）

### Phase 1（4-6 天）

1. model + normalize 基础
2. introspect MVP
3. golden 序列化框架

### Phase 2（4-6 天）

1. diff MVP（table/column/pk/index/enum）
2. render MVP
3. plan DAG 排序

### Phase 3（3-5 天）

1. parser MVP（CREATE/ALTER/INDEX/ENUM）
2. file->db、file->file 链路打通

### Phase 4（3-4 天）

1. 破坏性变更诊断
2. CLI 参数完善
3. 文档与示例

## 13. 风险清单与缓解

1. `pg_query_go` 构建链复杂
   - 缓解：锁定版本、在 CI 做 cgo 构建矩阵
2. SQL 语义同义表达太多导致误报
   - 缓解：先做 normalize 基线 + 回归用例集
3. drop 类操作误伤风险
   - 缓解：默认不输出危险 drop，需 `--unsafe-drop`

## 14. 下一步落地建议

先实现以下最小切片并合并到主分支：

1. `internal/model` + JSON snapshot
2. `internal/introspect`（仅 public + 可扩 schema 参数）
3. `internal/diff`（table/column/index）
4. `internal/render`（add/alter 基础 SQL）
5. 首个 roundtrip 集成测试

完成后再扩 parser，能显著降低返工概率。

## 15. 后续扩展（超越 Migra）

1. 覆盖 view/function/trigger/rule 的 diff 与渲染
2. YAML/JSON schema 输入支持
3. rollback SQL 生成（结合风险分级）
4. 输出 schema 依赖图（DOT）
5. LSP/IDE 辅助能力
