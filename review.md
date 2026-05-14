# README.md 与实际代码差异审查报告

> 审查时间：2026-05-12  
> 审查范围：README.md 描述 vs 实际代码实现

---

## 一、总体评估

README.md 整体描述与实际代码实现**高度一致**，架构设计、核心功能、技术栈、命令行参数等方面基本准确。但仍存在若干细节差异、描述不准确或过时之处，以下逐项列出。

---

## 二、项目结构差异

### 2.1 缺失的目录/文件

| README 描述 | 实际状态 | 说明 |
|-------------|----------|------|
| `internal/diff/diff_index.go` | ✅ 存在 | 索引差异比较已实现 |
| `internal/introspect/constraints.go` | ✅ 存在 | 约束内省已实现 |
| `internal/introspect/tables.go` | ✅ 存在 | 表内省已实现 |
| `internal/introspect/indexes.go` | ✅ 存在 | 索引内省已实现 |
| `internal/introspect/enums.go` | ✅ 存在 | 枚举内省已实现 |
| `internal/model/index_elem.go` | ✅ 存在 | IndexElem 模型已实现 |
| `internal/model/object_key.go` | ✅ 存在 | ObjectKey 模型已实现 |
| `internal/model/golden_test.go` | ✅ 存在 | 黄金测试文件已实现 |
| `internal/model/testdata/schema_golden.json` | ✅ 存在 | 黄金测试数据已实现 |
| `internal/parser/index_mutation.go` | ✅ 存在 | 索引 Mutation 已实现 |
| `internal/parser/parserutil/util.go` | ✅ 存在 | 解析工具函数已实现 |
| `internal/testutil/testutil.go` | ✅ 存在 | 测试工具集已实现 |

### 2.2 项目结构中未在 README 中提及的部分

| 实际存在的目录/文件 | 说明 |
|---------------------|------|
| `internal/model/golden_test.go` + `testdata/` | 黄金测试（Golden Test）机制，README 未提及 |
| `internal/parser/index_mutation.go` | 独立的索引 Mutation 类型，README 未提及 |
| `internal/parser/mutation_test.go` | Parser Mutation 单元测试 |
| `internal/parser/index_mutation_test.go` | 索引 Mutation 单元测试 |
| `cmd/migra/integration_test.go` | 集成测试文件 |
| `cmd/migra/diff_runner_test.go` | diff_runner 单元测试 |
| `docs/superpowers/` | Superpowers 框架相关计划和规格文档 |
| `.agents/`, `.claude/`, `.codex/`, `.jj/`, `.rtk/` | 开发工具和配置目录 |

**建议**：README 的项目结构部分可以补充测试文件和黄金测试机制的描述。

---

## 三、功能特性差异

### 3.1 README 声称存在且已实现的功能

| 功能 | 实现状态 | 代码位置 |
|------|----------|----------|
| 双向 Diff | ✅ 已实现 | `internal/diff/differ.go` |
| 结构化模型（SchemaModel） | ✅ 已实现 | `internal/model/schema.go`, `table.go`, `column.go` |
| SQL 生成与执行 | ✅ 已实现 | `internal/render/render.go`, `cmd/migra/push_runner.go` |
| 交互式安全执行 | ✅ 已实现 | `push_runner.go:executeWithConfirmation()` |
| 危险操作标记（DROP） | ✅ 已实现 | `internal/diff/operation.go` 中 `IsDestructive()` |
| `--unsafe-drop` 参数 | ✅ 已实现 | `diff.go:44`, `push.go:25` |
| 语义归一化 | ✅ 已实现 | `internal/normalize/normalize.go` |
| Kahn 算法 DAG 拓扑排序 | ✅ 已实现 | `internal/plan/dag.go:GetExecutionOrder()` |
| 三阶段执行计划 | ✅ 已实现 | `internal/plan/plan.go`（pre-deploy/deploy/post-deploy） |
| HandlerRegistry + 访问者模式 OCP | ✅ 已实现 | `internal/parser/registry.go` |
| SQL 和 JSON 输出格式 | ✅ 已实现 | `internal/render/render.go:RenderOutput()` |
| `--timeout` 参数 | ✅ 已实现 | `diff.go:47`, `push.go:29` |

### 3.2 README 声称存在但实现不完整的功能

#### 3.2.1 "约束"支持（部分实现）

README 提到支持"约束"，实际代码中：
- ✅ 约束模型已定义（`model/table.go` 中 `Constraint` 结构体）
- ✅ 约束差异比较已实现（`diff_tables.go:diffTableConstraints()`）
- ✅ 约束渲染已实现（`render.go` 中 `AddConstraintOp` / `DropConstraintOp`）
- ⚠️ **约束内省不完整**：`introspect/constraints.go` 仅支持主键、唯一约束和 CHECK 约束，**外键约束的内省未完全验证**
- ⚠️ **ALTER TABLE 仅支持 ADD COLUMN**：`alter_table_handler.go` 中明确标注 `// MVP: Skip other alter subcommands`，仅实现了 `AT_AddColumn`，其他 ALTER 子命令（如 DROP COLUMN、ALTER TYPE、ADD CONSTRAINT 等）仅输出警告

#### 3.2.2 "push 命令提供逐条确认"

README 描述准确，实际实现中：
- ✅ 逐条确认（y/n/a/s）已实现（`push_runner.go:245-285`）
- ✅ 危险操作中断已实现（`push_runner.go:256-258`）
- ✅ 事务保护已实现（`push_runner.go:201-210`）
- ✅ 自动回滚已实现（`push_runner.go:206-210` defer 回滚）
- ✅ 信号中断处理（SIGINT/SIGTERM）已实现（`push_runner.go:196-217`）

#### 3.2.3 "非事务性 DDL 检测"

README 未明确提及，但代码中实现了非事务性 DDL 检测（`push_runner.go:45-69`）：
- `CREATE INDEX CONCURRENTLY`
- `DROP INDEX CONCURRENTLY`
- `REINDEX INDEX CONCURRENTLY`

**建议**：README 可以补充此安全特性描述。

### 3.3 README 未提及但实际存在的功能

| 功能 | 代码位置 |
|------|----------|
| 非事务性 DDL 检测与拒绝 | `push_runner.go:45-69` |
| 信号中断处理（SIGINT/SIGTERM） | `push_runner.go:196-217` |
| 占位符表机制（处理 ALTER TABLE 在 CREATE TABLE 之前的情况） | `mutation.go:AddColumnMutation.Apply()` |
| 黄金测试（Golden Test）机制 | `model/golden_test.go` |
| 依赖注入模式（RunnerDeps） | `diff_service.go:RunnerDeps` |
| 编译时接口检查 | 多处 `var _ Interface = (*Impl)(nil)` |

---

## 四、命令行参数差异

### 4.1 diff 命令参数

| README 描述 | 实际代码 | 状态 |
|-------------|----------|------|
| `-s, --schema` | `diffCmd.Flags().StringSliceP("schema", "s", ...)` | ✅ 一致 |
| `-f, --format` | `diffCmd.Flags().GetStringP("format", "f", ...)` | ✅ 一致 |
| `--unsafe-drop` | `diffCmd.Flags().Bool("unsafe-drop", false, ...)` | ✅ 一致 |
| `--strict` | `diffCmd.Flags().Bool("strict", false, ...)` | ✅ 一致 |
| `--timeout` | `diffCmd.Flags().Duration("timeout", ...)` | ✅ 一致 |
| `-o, --output` | `diffCmd.Flags().GetStringP("output", "o", ...)` | ✅ 一致 |

### 4.2 push 命令参数

| README 描述 | 实际代码 | 状态 |
|-------------|----------|------|
| `-s, --schema` | `pushCmd.Flags().StringSliceP("schema", "s", ...)` | ✅ 一致 |
| `--unsafe-drop` | `pushCmd.Flags().Bool("unsafe-drop", false, ...)` | ✅ 一致 |
| `--dry-run` | `pushCmd.Flags().Bool("dry-run", true, ...)` | ✅ 一致 |
| `--execute` | `pushCmd.Flags().Bool("execute", false, ...)` | ✅ 一致 |
| `--no-verify` | `pushCmd.Flags().Bool("no-verify", false, ...)` | ✅ 一致 |
| `--timeout` | `pushCmd.Flags().Duration("timeout", ...)` | ✅ 一致 |

### 4.3 全局参数

| README 描述 | 实际代码 | 状态 |
|-------------|----------|------|
| `-c, --config` | `rootCmd.PersistentFlags().StringP("config", "c", ...)` | ✅ 一致 |
| `-v, --verbose` | `rootCmd.PersistentFlags().BoolP("verbose", "v", ...)` | ✅ 一致 |

### 4.4 参数差异详情

#### 4.4.1 push 命令的 `--format` 参数

README 的参数表格中 `--format` 仅列在 diff 下，实际代码中 push 命令**确实没有** `--format` 参数（push 内部硬编码为 `"sql"` 格式）。README 描述准确。

#### 4.4.2 push 命令的参数验证

实际代码中 push 命令有**目标地址验证**（`push_runner.go:126-131`）：
```go
if !strings.HasPrefix(lower, "postgres://") &&
    !strings.HasPrefix(lower, "postgresql://") &&
    !strings.HasPrefix(lower, "pg://") {
    return fmt.Errorf("target must be a database connection string (postgres://...)")
}
```
README 未提及此验证逻辑。

---

## 五、架构设计差异

### 5.1 数据流架构

README 描述的数据流：
```
SQL/DB → Parser → Diff → Renderer → Push
                ↘ Planner (DAG)
```

实际代码数据流：
```
SQL/DB → Source Registry → Parser/Introspect → Normalize → Diff → Plan (DAG) → Renderer → Output
                                                                        ↘ Push (交互确认)
```

**差异**：
1. README 未提及 **Source Registry** 和 **Loader** 抽象层（`internal/source/`）
2. README 未提及 **Normalize** 阶段在 Diff 之前执行
3. README 未提及 **依赖注入** 模式（`RunnerDeps`）

### 5.2 解析器架构

README 描述的 OCP 架构：
- ✅ HandlerRegistry + reflect.Type 路由（`registry.go`）
- ✅ Handler 接口（`registry.go:Handler`）
- ✅ Mutation 接口（`mutation.go:SchemaMutation`）
- ✅ MutationApplier（`applier.go`）

**实际额外实现**：
- `CreateIndexMutation` 独立类型（`index_mutation.go`），README 未提及
- `MutationError` 包装类型（`applier.go`），支持错误链
- 占位符表机制处理 DDL 顺序问题

### 5.3 差异比较引擎

README 未提及的实际实现细节：
- `diffContext` 结构体（`context.go`）作为局部状态，避免共享可变状态
- 警告（warnings）机制贯穿 diff 全流程
- 枚举类型仅支持追加式变更（append-only），非追加式变更会产生警告

---

## 六、技术栈差异

### 6.1 版本信息

| 组件 | README 描述 | 实际 go.mod | 状态 |
|------|-------------|-------------|------|
| Go 版本 | Go 1.26+ | `go 1.26.2` | ✅ 一致 |
| pgx | pgx v5 | `github.com/jackc/pgx/v5 v5.9.2` | ✅ 一致 |
| Cobra | 最新版 | `github.com/spf13/cobra v1.10.2` | ✅ 一致 |
| Viper | 最新版 | `github.com/spf13/viper v1.21.0` | ✅ 一致 |
| pg_query_go | 最新版 | `github.com/lfittl/pg_query_go v1.0.2` | ✅ 一致 |
| testify | 最新版 | `github.com/stretchr/testify v1.11.1` | ✅ 一致 |

### 6.2 间接依赖

README 未提及但实际存在的间接依赖：
- `github.com/jackc/pgpassfile` — .pgpass 文件支持
- `github.com/jackc/pgservicefile` — pg_service.conf 支持
- `github.com/pelletier/go-toml/v2` — TOML 配置支持
- `github.com/go-viper/mapstructure/v2` — 配置映射

---

## 七、配置文件差异

### 7.1 配置文件结构

README 和 `examples/config.yaml` 描述的配置结构与实际代码的对应关系：

| 配置项 | config.yaml | 实际使用 | 状态 |
|--------|-------------|----------|------|
| `database.url` | ✅ | `diff_runner.go:27` `viper.GetString("database.url")` | ✅ 已使用 |
| `database.source` | ❌ 未在示例中 | `diff_runner.go:32` `viper.GetString("database.source")` | ⚠️ 示例缺失 |
| `database.target` | ❌ 未在示例中 | `diff_runner.go:33` `viper.GetString("database.target")` | ⚠️ 示例缺失 |
| `database.pool.*` | ✅ | ❌ **未在代码中使用** | ⚠️ 未实现 |
| `diff.schemas` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `diff.unsafe_drop` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `diff.strict` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `diff.format` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `output.file` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `output.verbose` | ✅ | ❌ **未通过 viper 绑定** | ⚠️ 未实现 |
| `logging.*` | ✅ | ❌ **未在代码中使用** | ⚠️ 未实现 |

### 7.2 环境变量

| .env.example | 实际代码 | 状态 |
|--------------|----------|------|
| `DATABASE_URL` | ❌ 未使用 | ⚠️ 代码使用 `database.url` 而非 `DATABASE_URL` |
| `MIGRA_SCHEMAS` | ❌ 未使用 | ⚠️ 代码使用命令行 `--schema` 而非环境变量 |
| `MIGRA_UNSAFE_DROP` | ❌ 未使用 | ⚠️ 同上 |
| `MIGRA_STRICT` | ❌ 未使用 | ⚠️ 同上 |
| `MIGRA_FORMAT` | ❌ 未使用 | ⚠️ 同上 |
| `MIGRA_OUTPUT` | ❌ 未使用 | ⚠️ 同上 |
| `MIGRA_VERBOSE` | ❌ 未使用 | ⚠️ 同上 |
| `LOG_LEVEL` | ❌ 未使用 | ⚠️ 日志级别未实现 |

**重大发现**：`examples/.env.example` 中定义的环境变量**均未在代码中实际使用**。代码仅通过 Viper 读取 `database.url`、`database.source`、`database.target` 三个配置项，且仅用于 diff 命令的默认参数。

### 7.3 Viper 自动环境变量绑定

代码中使用了 `viper.AutomaticEnv()`（`main.go:49`），但未设置环境变量前缀。这意味着 Viper 会自动将配置键（如 `database.url`）映射为 `DATABASE_URL` 环境变量。但实际测试需要验证此行为是否与 `.env.example` 中的 `MIGRA_` 前缀变量冲突。

---

## 八、数据库连接方式差异

### 8.1 支持的连接格式

| README 描述 | 实际代码 | 状态 |
|-------------|----------|------|
| `postgres://` | `db_loader.go:18` | ✅ |
| `postgresql://` | `db_loader.go:19` | ✅ |
| `pg://` | `db_loader.go:20` | ✅ |
| `mysql://` | `db_loader.go:21` | ⚠️ **匹配了但未实现 MySQL 加载器** |

**问题**：`DBLoader.Match()` 中包含 `mysql://` 前缀匹配，但实际加载逻辑调用的是 `introspect.LoadFromDB()`，该函数使用 pgx 连接 PostgreSQL。这意味着传入 `mysql://` 会导致运行时错误而非友好的"不支持"提示。

### 8.2 pg:// 短连接格式

README 示例中使用了 `pg://localhost/dbname` 格式。实际代码中 `DBLoader.Match()` 确实匹配 `pg://` 前缀，pgx 驱动也支持此格式。✅ 一致。

---

## 九、其他差异

### 9.1 diff 命令的参数数量

README 描述：
```bash
migra diff file.sql  # target from config database.url
migra diff          # both from config database.source and database.target
```

实际代码（`diff.go:34`）：`cobra.RangeArgs(0, 2)` ✅ 支持 0、1、2 个参数。

### 9.2 push 命令的参数数量

README 描述：
```bash
migra push file.sql postgres://localhost/db
```

实际代码（`push.go:17`）：`cobra.ExactArgs(2)` ✅ 必须提供 2 个参数。

### 9.3 Makefile 命令

| README 描述 | 实际 Makefile | 状态 |
|-------------|---------------|------|
| `make build` | `go build -o migra ./cmd/migra` | ✅ |
| `make test` | `go test ./... -v` | ✅ |
| `make lint` | `golangci-lint run ./...` | ✅ |
| `make fmt` | `gofmt -w . && go fmt ./...` | ✅ |
| `make vet` | `go vet ./...` | ✅ |
| `make ci` | `fmt vet lint test` | ✅ |

Makefile 还包含 `make test-coverage` 和 `make clean`，README 未提及。

---

## 十、建议与改进

### 10.1 高优先级

1. **修复配置文件与代码的不一致**：
   - 实现 `database.pool.*` 配置的实际使用
   - 实现 `diff.*` 配置项通过 Viper 绑定到命令行默认值
   - 实现 `logging.*` 配置的实际使用
   - 或者从 `config.yaml` 和 `docs/configuration.md` 中移除未实现的配置项

2. **修复环境变量示例**：
   - `examples/.env.example` 中的 `MIGRA_*` 环境变量未被代码使用
   - 建议要么在代码中实现这些环境变量的绑定，要么更新 `.env.example` 为实际可用的配置

3. **移除 mysql:// 匹配或添加友好错误提示**：
   - `DBLoader.Match()` 中的 `mysql://` 匹配会导致误导性的运行时错误

### 10.2 中优先级

4. **补充 README 中缺失的功能描述**：
   - 非事务性 DDL 检测
   - 信号中断处理
   - 黄金测试机制
   - 依赖注入模式

5. **更新项目结构说明**：
   - 补充 `internal/testutil/`、黄金测试、集成测试等条目

6. **补充 ALTER TABLE 的限制说明**：
   - 当前仅支持 `ADD COLUMN`，README 应明确说明此限制

### 10.3 低优先级

7. **补充 Makefile 的 `test-coverage` 和 `clean` 目标说明**

8. **考虑添加配置验证**：
   - 启动时验证配置文件中的未知键并给出警告

---

## 十一、总结

| 评估维度 | 一致性评分 | 说明 |
|----------|-----------|------|
| 项目结构 | 95% | 基本一致，少量未提及的测试文件 |
| 功能特性 | 90% | 核心功能完整，部分功能（约束、ALTER）为 MVP 级别 |
| 命令行参数 | 100% | 完全一致 |
| 架构设计 | 85% | 核心架构一致，但缺少对 Source Registry、Normalize、依赖注入的描述 |
| 技术栈 | 100% | 完全一致 |
| 配置文件 | 60% | 存在较多未实现的配置项，环境变量示例与代码不匹配 |
| 数据库连接 | 90% | 基本一致，mysql:// 匹配存在问题 |

**总体评分：87/100**

README 作为项目概览文档质量较高，核心描述准确。主要问题集中在配置文件与代码实现的不一致，以及部分高级特性未在文档中体现。
