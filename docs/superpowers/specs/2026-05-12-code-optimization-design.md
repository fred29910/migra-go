# 代码优化设计文档

> 日期：2026-05-12  
> 基于：review.md 审查报告  
> 评审：r1.md 评审报告（已修复所有必须修复和建议修改项）

---

## 1. 背景

根据代码审查报告，发现 README.md 描述与实际代码实现存在差异，主要包括：
- 配置文件大量未使用的配置项
- 环境变量示例与代码不匹配
- mysql:// 协议匹配陷阱
- ALTER TABLE 仅支持 ADD COLUMN（MVP 级别）

---

## 2. 设计目标

1. 使配置文件所有配置项在代码中实际可用
2. 环境变量使用 MIGRA_ 前缀，通过 viper.SetEnvPrefix 自动绑定
3. 移除 mysql:// 匹配陷阱（先添加友好错误提示）
4. 扩展 ALTER TABLE 支持常用子命令
5. 更新相关文档保持一致性

---

## 3. 配置系统设计

### 3.1 配置优先级

```
命令行参数 > 环境变量(MIGRA_*) > 配置文件(~/.migra.yaml 或 ./migra.yaml) > 默认值
```

### 3.2 Viper 环境变量绑定规则说明

使用 `viper.SetEnvPrefix("MIGRA")` 后，Viper 会自动将配置键转为大写并用 `_` 连接：
- `database.url` → `MIGRA_DATABASE_URL`
- `database.pool.max_open_conns` → `MIGRA_DATABASE_POOL_MAX_OPEN_CONNS`
- `diff.schemas` → `MIGRA_DIFF_SCHEMAS`

**注意**：设置前缀后，原有的无前缀环境变量（如 `DATABASE_URL`）将不再自动绑定。这是预期行为，因为我们需要统一使用 `MIGRA_` 前缀。

### 3.3 环境变量映射表

| 配置键 | 环境变量 | 示例值 |
|--------|----------|--------|
| `database.url` | `MIGRA_DATABASE_URL` | `postgres://localhost/mydb` |
| `database.source` | `MIGRA_DATABASE_SOURCE` | `postgres://localhost/db1` |
| `database.target` | `MIGRA_DATABASE_TARGET` | `postgres://localhost/db2` |
| `database.pool.max_open_conns` | `MIGRA_DATABASE_POOL_MAX_OPEN_CONNS` | `10` |
| `database.pool.max_idle_conns` | `MIGRA_DATABASE_POOL_MAX_IDLE_CONNS` | `5` |
| `database.pool.conn_max_lifetime` | `MIGRA_DATABASE_POOL_CONN_MAX_LIFETIME` | `1h` |
| `diff.schemas` | `MIGRA_DIFF_SCHEMAS` | `public,auth` |
| `diff.unsafe_drop` | `MIGRA_DIFF_UNSAFE_DROP` | `false` |
| `diff.strict` | `MIGRA_DIFF_STRICT` | `false` |
| `diff.format` | `MIGRA_DIFF_FORMAT` | `sql` |
| `output.file` | `MIGRA_OUTPUT_FILE` | `/tmp/diff.sql` |
| `output.verbose` | `MIGRA_OUTPUT_VERBOSE` | `false` |
| `logging.level` | `MIGRA_LOGGING_LEVEL` | `info` |
| `logging.format` | `MIGRA_LOGGING_FORMAT` | `text` |

### 3.4 实现方案

#### 3.4.1 main.go 修改

```go
func initConfig() {
    // 设置环境变量前缀为 MIGRA_
    viper.SetEnvPrefix("MIGRA")
    viper.AutomaticEnv()

    if cfgFile := viper.GetString("config"); cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        if err == nil {
            viper.AddConfigPath(home)
            viper.SetConfigName(".migra")
        }
        viper.AddConfigPath(".")
        viper.SetConfigName("migra")
    }

    if err := viper.ReadInConfig(); err == nil {
        if viper.GetBool("verbose") {
            fmt.Println("Using config file:", viper.ConfigFileUsed())
        }
    }
}
```

**BindPFlag 兼容性说明**：`viper.BindPFlag` 可以多次调用，不会覆盖已有绑定。每次调用仅建立配置键到命令行 flag 的映射关系。现有 `main.go:26-31` 的 BindPFlag 调用与新添加的调用互不冲突。

#### 3.4.2 diff.go 修改

在 init() 中绑定命令行 flag 到 viper 配置键，使配置文件和环境变量可以作为默认值：

```go
func init() {
    rootCmd.AddCommand(diffCmd)

    // Diff-specific flags（默认值从 viper 读取）
    diffCmd.Flags().StringSliceP("schema", "s", viper.GetStringSlice("diff.schemas"), "schemas to compare (can be multiple)")
    diffCmd.Flags().StringP("format", "f", viper.GetString("diff.format"), "output format: sql or json")
    diffCmd.Flags().Bool("unsafe-drop", viper.GetBool("diff.unsafe_drop"), "allow destructive drop operations")
    diffCmd.Flags().Bool("strict", viper.GetBool("diff.strict"), "fail on unsupported statements")
    diffCmd.Flags().StringP("output", "o", viper.GetString("output.file"), "output file (default: stdout)")
    diffCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading (e.g. 30s, 2m)")

    // 绑定到 viper
    viper.BindPFlag("diff.schemas", diffCmd.Flags().Lookup("schema"))
    viper.BindPFlag("diff.format", diffCmd.Flags().Lookup("format"))
    viper.BindPFlag("diff.unsafe_drop", diffCmd.Flags().Lookup("unsafe-drop"))
    viper.BindPFlag("diff.strict", diffCmd.Flags().Lookup("strict"))
    viper.BindPFlag("output.file", diffCmd.Flags().Lookup("output"))
}
```

#### 3.4.3 push.go 修改

push 命令同样需要绑定 viper 默认值：

```go
func init() {
    rootCmd.AddCommand(pushCmd)

    pushCmd.Flags().StringSliceP("schema", "s", viper.GetStringSlice("diff.schemas"), "schemas to compare (can be multiple)")
    pushCmd.Flags().Bool("unsafe-drop", viper.GetBool("diff.unsafe_drop"), "skip confirmation for destructive DROP operations")
    pushCmd.Flags().Bool("dry-run", true, "show SQL without executing (default: true)")
    pushCmd.Flags().Bool("execute", false, "execute SQL without confirmation (not recommended)")
    pushCmd.Flags().Bool("no-verify", false, "skip post-execution validation")
    pushCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading")

    // 绑定到 viper
    viper.BindPFlag("diff.schemas", pushCmd.Flags().Lookup("schema"))
    viper.BindPFlag("diff.unsafe_drop", pushCmd.Flags().Lookup("unsafe-drop"))
}
```

#### 3.4.4 db_loader.go 修改

使用 pool 配置：

```go
func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    connConfig, err := pgx.ParseConfig(source)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to parse connection string: %w", err)
    }

    // 应用 pool 配置（从 viper 读取）
    if maxOpen := viper.GetInt("database.pool.max_open_conns"); maxOpen > 0 {
        connConfig.MaxConns = int32(maxOpen)
    }
    if maxIdle := viper.GetInt("database.pool.max_idle_conns"); maxIdle > 0 {
        connConfig.MinConns = int32(maxIdle)
    }
    if lifetime := viper.GetDuration("database.pool.conn_max_lifetime"); lifetime > 0 {
        connConfig.MaxConnLifetime = lifetime
    }

    conn, err := pgx.ConnectConfig(ctx, connConfig)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    defer conn.Close(ctx)

    // ... 其余代码不变
}
```

---

## 4. ALTER TABLE 扩展设计

### 4.1 新增 Mutation 类型

在 `internal/parser/mutation.go` 中添加：

```go
// DropColumnMutation 删除列
type DropColumnMutation struct {
    Schema string
    Table  string
    Column string
}

func (m DropColumnMutation) Kind() MutationKind { return MutKindDropColumn }
func (m DropColumnMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m DropColumnMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    if _, exists := table.ColumnByName[m.Column]; !exists {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    table.RemoveColumn(m.Column)
    return nil
}

// AlterColumnTypeMutation 修改列类型
type AlterColumnTypeMutation struct {
    Schema   string
    Table    string
    Column   string
    FromType string
    ToType   string
}

func (m AlterColumnTypeMutation) Kind() MutationKind { return MutKindAlterColumnType }
func (m AlterColumnTypeMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m AlterColumnTypeMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    // 类型兼容性检查（可选，PostgreSQL 会在运行时检查）
    col.DataType = m.ToType
    return nil
}

// SetNotNullMutation 设置 NOT NULL
type SetNotNullMutation struct {
    Schema string
    Table  string
    Column string
}

func (m SetNotNullMutation) Kind() MutationKind { return MutKindSetNotNull }
func (m SetNotNullMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m SetNotNullMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    col.IsNullable = false
    return nil
}

// DropNotNullMutation 删除 NOT NULL
type DropNotNullMutation struct {
    Schema string
    Table  string
    Column string
}

func (m DropNotNullMutation) Kind() MutationKind { return MutKindDropNotNull }
func (m DropNotNullMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m DropNotNullMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    col.IsNullable = true
    return nil
}

// SetDefaultMutation 设置默认值
type SetDefaultMutation struct {
    Schema      string
    Table       string
    Column      string
    DefaultExpr string
}

func (m SetDefaultMutation) Kind() MutationKind { return MutKindSetDefault }
func (m SetDefaultMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m SetDefaultMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    col.DefaultExpr = &m.DefaultExpr
    return nil
}

// DropDefaultMutation 删除默认值
type DropDefaultMutation struct {
    Schema string
    Table  string
    Column string
}

func (m DropDefaultMutation) Kind() MutationKind { return MutKindDropDefault }
func (m DropDefaultMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
func (m DropDefaultMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil {
        return fmt.Errorf("schema %s not found", m.Schema)
    }
    table, exists := ns.Tables[m.Table]
    if !exists {
        return fmt.Errorf("table %s.%s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    col.DefaultExpr = nil
    return nil
}
```

### 4.2 MutationKind 常量扩展

在 `internal/parser/mutation.go` 中添加新常量：

```go
const (
    MutKindCreateTable      MutationKind = "create_table"
    MutKindAddColumn        MutationKind = "add_column"
    MutKindCreateEnumType   MutationKind = "create_enum_type"
    MutKindCreateIndex      MutationKind = "create_index"
    // 新增
    MutKindDropColumn       MutationKind = "drop_column"
    MutKindAlterColumnType  MutationKind = "alter_column_type"
    MutKindSetNotNull       MutationKind = "set_not_null"
    MutKindDropNotNull      MutationKind = "drop_not_null"
    MutKindSetDefault       MutationKind = "set_default"
    MutKindDropDefault      MutationKind = "drop_default"
)
```

### 4.3 Table.RemoveColumn 方法

在 `internal/model/table.go` 中添加：

```go
// RemoveColumn removes a column by name from the table
func (t *Table) RemoveColumn(name string) {
    delete(t.ColumnByName, name)
    for i, col := range t.Columns {
        if col.Name == name {
            t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
            return
        }
    }
}
```

### 4.4 AlterTableHandler 完整扩展

在 `internal/parser/alter_table_handler.go` 中扩展 Handle 方法：

```go
func (h *AlterTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
    stmt, ok := node.(pg_nodes.AlterTableStmt)
    if !ok {
        return nil, fmt.Errorf("AlterTableHandler: expected pg_nodes.AlterTableStmt, got %T", node)
    }
    tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

    var mutations []SchemaMutation
    for _, item := range stmt.Cmds.Items {
        cmd, ok := item.(pg_nodes.AlterTableCmd)
        if !ok {
            continue
        }
        switch cmd.Subtype {
        case pg_nodes.AT_AddColumn:
            if cmd.Def != nil {
                if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
                    col := parserutil.ParseColumnDef(colDef)
                    mutations = append(mutations, AddColumnMutation{
                        Schema: schemaName,
                        Table:  tableName,
                        Column: *col,
                    })
                }
            }

        case pg_nodes.AT_DropColumn:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" {
                fmt.Fprintf(os.Stderr, "warning: DROP COLUMN missing column name\n")
                continue
            }
            mutations = append(mutations, DropColumnMutation{
                Schema: schemaName,
                Table:  tableName,
                Column: colName,
            })

        case pg_nodes.AT_AlterColumnType:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" || cmd.Def == nil {
                fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN TYPE missing column name or type\n")
                continue
            }
            if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
                col := parserutil.ParseColumnDef(colDef)
                mutations = append(mutations, AlterColumnTypeMutation{
                    Schema:   schemaName,
                    Table:    tableName,
                    Column:   colName,
                    ToType:   col.DataType,
                })
            }

        case pg_nodes.AT_SetNotNull:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" {
                fmt.Fprintf(os.Stderr, "warning: SET NOT NULL missing column name\n")
                continue
            }
            mutations = append(mutations, SetNotNullMutation{
                Schema: schemaName,
                Table:  tableName,
                Column: colName,
            })

        case pg_nodes.AT_DropNotNull:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" {
                fmt.Fprintf(os.Stderr, "warning: DROP NOT NULL missing column name\n")
                continue
            }
            mutations = append(mutations, DropNotNullMutation{
                Schema: schemaName,
                Table:  tableName,
                Column: colName,
            })

        case pg_nodes.AT_SetDefault:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" || cmd.Def == nil {
                fmt.Fprintf(os.Stderr, "warning: SET DEFAULT missing column name or expression\n")
                continue
            }
            // 从 Def 中提取默认值表达式
            defaultExpr := extractDefaultExpr(cmd.Def)
            mutations = append(mutations, SetDefaultMutation{
                Schema:      schemaName,
                Table:       tableName,
                Column:      colName,
                DefaultExpr: defaultExpr,
            })

        case pg_nodes.AT_DropDefault:
            colName := ""
            if cmd.Name != nil {
                colName = *cmd.Name
            }
            if colName == "" {
                fmt.Fprintf(os.Stderr, "warning: DROP DEFAULT missing column name\n")
                continue
            }
            mutations = append(mutations, DropDefaultMutation{
                Schema: schemaName,
                Table:  tableName,
                Column: colName,
            })

        default:
            fmt.Fprintf(os.Stderr, "warning: unsupported ALTER TABLE subcommand: %v\n", cmd.Subtype)
        }
    }
    return mutations, nil
}

// extractDefaultExpr 从节点中提取默认值表达式
func extractDefaultExpr(node pg_nodes.Node) string {
    // 使用 pg_query 的 Deparse 功能
    if d, ok := node.(interface{ Deparse() string }); ok {
        defer func() {
            if r := recover(); r != nil {
                // Deparse not implemented
            }
        }()
        return d.Deparse()
    }
    return fmt.Sprintf("%v", node)
}
```

### 4.5 ALTER COLUMN TYPE 的 USING 子句说明

**性能考虑**：当修改列类型时，如果 PostgreSQL 无法隐式转换类型，需要使用 `USING` 子句。例如：
```sql
ALTER TABLE t ALTER COLUMN c TYPE integer USING (c::integer);
```

**设计决策**：
- 解析阶段：仅提取目标类型，不处理 USING 子句（USING 是运行时行为）
- 渲染阶段：生成 `ALTER TABLE ... ALTER COLUMN ... TYPE ...;`，不包含 USING
- 用户需要在迁移脚本中手动添加 USING 子句（如果需要）

---

## 5. 其他修复

### 5.1 移除 mysql:// 匹配（分两步）

#### 步骤 1：先添加友好错误提示

修改 `internal/source/db_loader.go`：

```go
func (l *DBLoader) Match(source string) bool {
    lowerSource := strings.ToLower(source)
    // 检测 MySQL 连接字符串并给出友好提示
    if strings.HasPrefix(lowerSource, "mysql://") {
        return true // 匹配但会在 Load 中返回错误
    }
    return strings.HasPrefix(lowerSource, "postgres://") ||
        strings.HasPrefix(lowerSource, "postgresql://") ||
        strings.HasPrefix(lowerSource, "pg://")
}

func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    // 检查 MySQL 并返回友好错误
    if strings.HasPrefix(strings.ToLower(source), "mysql://") {
        return nil, nil, fmt.Errorf("MySQL is not supported, only PostgreSQL. Please use a postgres:// connection string")
    }
    // ... 其余代码不变
}
```

#### 步骤 2：后续版本移除 mysql:// 匹配

在确认没有用户使用后，从 Match 中移除 mysql:// 前缀。

### 5.2 更新 .env.example

更新 `examples/.env.example` 为实际可用的环境变量：

```bash
# migra 环境变量配置示例
# 使用 MIGRA_ 前缀（参考 https://pkg.go.dev/github.com/spf13/viper#SetEnvPrefix）
# 注意：.env 文件中的值不需要引号包裹，除非值中包含空格

# 数据库连接
MIGRA_DATABASE_URL=postgres://user:password@localhost:5432/dbname?sslmode=disable
MIGRA_DATABASE_SOURCE=postgres://localhost/db1
MIGRA_DATABASE_TARGET=postgres://localhost/db2

# 连接池配置
MIGRA_DATABASE_POOL_MAX_OPEN_CONNS=10
MIGRA_DATABASE_POOL_MAX_IDLE_CONNS=5
MIGRA_DATABASE_POOL_CONN_MAX_LIFETIME=1h

# 比较选项
MIGRA_DIFF_SCHEMAS=public
MIGRA_DIFF_UNSAFE_DROP=false
MIGRA_DIFF_STRICT=false
MIGRA_DIFF_FORMAT=sql

# 输出选项
MIGRA_OUTPUT_FILE=
MIGRA_OUTPUT_VERBOSE=false

# 日志
MIGRA_LOGGING_LEVEL=info
MIGRA_LOGGING_FORMAT=text
```

---

## 6. 测试策略

### 6.1 配置测试（table-driven）

```go
func TestConfigPriority(t *testing.T) {
    tests := []struct {
        name     string
        env      map[string]string
        config   map[string]string
        flag     map[string]string
        expected string
    }{
        {
            name:     "default value",
            env:      nil,
            config:   nil,
            flag:     nil,
            expected: "public",
        },
        {
            name:     "config file overrides default",
            env:      nil,
            config:   map[string]string{"diff.schemas": "auth"},
            flag:     nil,
            expected: "auth",
        },
        {
            name:     "env overrides config",
            env:      map[string]string{"MIGRA_DIFF_SCHEMAS": "api"},
            config:   map[string]string{"diff.schemas": "auth"},
            flag:     nil,
            expected: "api",
        },
        {
            name:     "flag overrides env",
            env:      map[string]string{"MIGRA_DIFF_SCHEMAS": "api"},
            config:   map[string]string{"diff.schemas": "auth"},
            flag:     map[string]string{"schema": "custom"},
            expected: "custom",
        },
    }
    // ...
}
```

### 6.2 ALTER TABLE 测试

#### 6.2.1 单元测试（每个 Mutation 类型）

```go
func TestDropColumnMutation_Apply(t *testing.T) {
    // 创建测试表
    schema := model.NewSchema()
    ns := schema.GetOrCreateNamespace("public")
    table := model.NewTable("public", "users")
    table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
    table.AddColumn(&model.Column{Name: "name", DataType: "varchar"})
    ns.Tables["users"] = table

    // 执行删除
    mut := DropColumnMutation{Schema: "public", Table: "users", Column: "name"}
    err := mut.Apply(schema)

    // 验证
    assert.NoError(t, err)
    assert.Nil(t, table.ColumnByName["name"])
    assert.Len(t, table.Columns, 1)
}
```

#### 6.2.2 集成测试（Golden Test）

使用 Golden Test 验证生成的 SQL：

```go
func TestAlterTableSQLGeneration(t *testing.T) {
    tests := []struct {
        name     string
        source   string
        target   string
        expected string // 从 golden 文件读取
    }{
        {
            name:   "drop column",
            source: testdataFile("alter_source.sql"),
            target: testdataFile("alter_target.sql"),
            expected: testdataFile("alter_drop_column_golden.sql"),
        },
        {
            name:   "alter column type",
            source: testdataFile("alter_source.sql"),
            target: testdataFile("alter_target.sql"),
            expected: testdataFile("alter_type_golden.sql"),
        },
    }
    // ...
}
```

### 6.3 回归测试

- 运行 `make test` 确保所有现有测试通过
- 验证 mysql:// 连接字符串返回友好错误
- 验证现有配置文件仍然有效

---

## 7. 影响范围

| 文件 | 修改类型 |
|------|----------|
| `cmd/migra/main.go` | 添加 SetEnvPrefix |
| `cmd/migra/diff.go` | 绑定 viper 默认值，从 viper 读取 flag 默认值 |
| `cmd/migra/push.go` | 绑定 viper 默认值，从 viper 读取 flag 默认值 |
| `cmd/migra/diff_runner.go` | 绑定 viper 默认值 |
| `cmd/migra/push_runner.go` | 绑定 viper 默认值 |
| `internal/source/db_loader.go` | 移除 mysql:// 匹配，添加 pool 配置 |
| `internal/parser/alter_table_handler.go` | 扩展 ALTER TABLE 支持 |
| `internal/parser/mutation.go` | 添加新 Mutation 类型和常量 |
| `internal/model/table.go` | 添加 RemoveColumn 方法 |
| `examples/.env.example` | 更新为 MIGRA_* 变量 |
| `examples/config.yaml` | 添加注释说明（可选） |
| `docs/configuration.md` | 同步更新配置文档 |
| `README.md` | 更新配置文件说明 |

---

## 8. 风险评估与缓解措施

| 风险 | 等级 | 缓解措施 |
|------|------|----------|
| 配置系统修改导致向后兼容问题 | 中 | 1. 添加配置优先级测试用例<br>2. 保留原有默认值行为<br>3. 在 CHANGELOG 中说明变更 |
| ALTER TABLE 扩展引入解析错误 | 中 | 1. 使用 Golden Test 验证 SQL 生成<br>2. 添加边界测试（如删除被引用的列）<br>3. 保持原有警告输出行为 |
| mysql:// 移除影响现有用户 | 低 | 1. 先添加友好错误提示<br>2. 在后续版本中移除<br>3. 在 release notes 中说明 |
| 环境变量前缀变更 | 低 | 1. 原有无前缀变量本就不工作<br>2. 更新 .env.example 和文档 |

---

## 9. 文档更新计划

| 文档 | 更新内容 |
|------|----------|
| `README.md` | 1. 更新环境变量说明为 MIGRA_ 前缀<br>2. 补充 ALTER TABLE 支持的子命令列表<br>3. 补充非事务性 DDL 检测说明 |
| `docs/configuration.md` | 1. 同步环境变量映射表<br>2. 补充 Viper 绑定规则说明 |
| `examples/config.yaml` | 添加注释说明各配置项用途 |
| `CHANGELOG.md` | 记录本次变更 |

---

## 10. 后续工作

1. 执行实现计划
2. 运行 `make ci` 确保所有检查通过
3. 更新 README.md 补充缺失的功能描述
4. 考虑添加更多 ALTER TABLE 子命令（如 RENAME COLUMN、ADD CONSTRAINT）
