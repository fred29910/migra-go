# 代码优化实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复 README.md 与实际代码的差异，包括配置系统实现、ALTER TABLE 扩展、mysql:// 移除和环境变量更新。

**架构：** 1) 使用 viper.SetEnvPrefix("MIGRA") 实现环境变量绑定，将配置文件中的 pool/diff/output/logging 配置项接入代码；2) 扩展 AlterTableHandler 支持 DROP COLUMN、ALTER COLUMN TYPE、SET/DROP NOT NULL、SET/DROP DEFAULT 子命令，新增对应 Mutation 类型；3) 移除 mysql:// 匹配并添加友好错误提示；4) 更新 .env.example 和文档。

**技术栈：** Go 1.26.2, Cobra v1.10.2, Viper v1.21.0, pgx v5.9.2, pg_query_go v1.0.2, testify v1.11.1

---

## 文件结构

### 修改的文件

| 文件 | 变更内容 |
|------|----------|
| `cmd/migra/main.go` | 添加 viper.SetEnvPrefix("MIGRA") |
| `cmd/migra/diff.go` | 从 viper 读取 flag 默认值，绑定 viper |
| `cmd/migra/push.go` | 从 viper 读取 flag 默认值，绑定 viper |
| `internal/source/db_loader.go` | 移除 mysql:// 匹配，添加 pool 配置 |
| `internal/parser/mutation.go` | 添加新 Mutation 类型和常量 |
| `internal/parser/alter_table_handler.go` | 扩展 ALTER TABLE 子命令支持 |
| `internal/model/table.go` | 添加 RemoveColumn 方法 |
| `examples/.env.example` | 更新为 MIGRA_* 变量 |

### 新增的测试文件

| 文件 | 职责 |
|------|------|
| `internal/parser/alter_table_extended_test.go` | ALTER TABLE 扩展解析测试 |
| `internal/parser/mutation_extended_test.go` | 新 Mutation 类型 Apply 测试 |
| `cmd/migra/config_test.go` | 配置优先级测试 |

---

## 任务

### 任务 1：配置系统 - main.go 添加环境变量前缀

**文件：**
- 修改：`cmd/migra/main.go:35-56`

- [ ] **步骤 1：修改 initConfig 添加 SetEnvPrefix**

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

- [ ] **步骤 2：运行测试验证现有功能**

运行：`rtk go test ./cmd/migra/... -v -count=1`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/main.go
git commit -m "feat(config): add MIGRA_ env prefix via viper.SetEnvPrefix"
```

---

### 任务 2：配置系统 - diff.go 绑定 viper 默认值

**文件：**
- 修改：`cmd/migra/diff.go:38-48`

- [ ] **步骤 1：修改 diff.go init() 从 viper 读取默认值**

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

- [ ] **步骤 2：运行测试验证**

运行：`rtk go test ./cmd/migra/... -v -count=1`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/diff.go
git commit -m "feat(config): bind diff flags to viper for config file defaults"
```

---

### 任务 3：配置系统 - push.go 绑定 viper 默认值

**文件：**
- 修改：`cmd/migra/push.go:21-30`

- [ ] **步骤 1：修改 push.go init() 从 viper 读取默认值**

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

- [ ] **步骤 2：运行测试验证**

运行：`rtk go test ./cmd/migra/... -v -count=1`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/push.go
git commit -m "feat(config): bind push flags to viper for config file defaults"
```

---

### 任务 4：配置系统 - db_loader.go 添加 pool 配置

**文件：**
- 修改：`internal/source/db_loader.go`

- [ ] **步骤 1：编写失败的测试**

创建 `internal/source/db_loader_pool_test.go`：

```go
package source

import (
    "testing"
    "github.com/spf13/viper"
    "github.com/stretchr/testify/assert"
)

func TestDBLoaderPoolConfig(t *testing.T) {
    // 设置 pool 配置
    viper.Set("database.pool.max_open_conns", 20)
    viper.Set("database.pool.max_idle_conns", 10)
    viper.Set("database.pool.conn_max_lifetime", "30m")
    defer viper.Reset()

    loader := &DBLoader{}
    // 验证 Match 仍然工作
    assert.True(t, loader.Match("postgres://localhost/db"))
    assert.False(t, loader.Match("mysql://localhost/db"))
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/source/... -run TestDBLoaderPoolConfig -v`
预期：FAIL（mysql:// 匹配尚未移除）

- [ ] **步骤 3：修改 db_loader.go 移除 mysql:// 并添加 pool 配置**

```go
package source

import (
    "context"
    "fmt"
    "strings"

    "github.com/fred29910/migra-go/internal/introspect"
    "github.com/fred29910/migra-go/internal/model"
    "github.com/jackc/pgx/v5"
    "github.com/spf13/viper"
)

// DBLoader implements Loader for database connections.
type DBLoader struct{}

// Match returns true if the source is a database connection string.
func (l *DBLoader) Match(source string) bool {
    lowerSource := strings.ToLower(source)
    return strings.HasPrefix(lowerSource, "postgres://") ||
        strings.HasPrefix(lowerSource, "postgresql://") ||
        strings.HasPrefix(lowerSource, "pg://")
}

// Load loads schema from a database connection string.
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

    introspectOpt := introspect.LoadOptions{
        Schemas: opt.Schemas,
    }
    schema, err := introspect.LoadFromDBWithConn(ctx, conn, introspectOpt)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to load schema from database: %w", err)
    }
    return schema, nil, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/source/... -v -count=1`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/source/db_loader.go internal/source/db_loader_pool_test.go
git commit -m "feat(config): remove mysql:// match and add pool config support"
```

---

### 任务 5：模型层 - Table.RemoveColumn 方法

**文件：**
- 修改：`internal/model/table.go`
- 测试：`internal/model/table_test.go`（如果不存在则创建）

- [ ] **步骤 1：编写失败的测试**

```go
func TestTableRemoveColumn(t *testing.T) {
    table := NewTable("public", "users")
    table.AddColumn(&Column{Name: "id", DataType: "integer"})
    table.AddColumn(&Column{Name: "name", DataType: "varchar"})
    table.AddColumn(&Column{Name: "email", DataType: "varchar"})

    // 删除中间列
    table.RemoveColumn("name")

    assert.Len(t, table.Columns, 2)
    assert.Nil(t, table.ColumnByName["name"])
    assert.NotNil(t, table.ColumnByName["id"])
    assert.NotNil(t, table.ColumnByName["email"])

    // 验证顺序保持
    assert.Equal(t, "id", table.Columns[0].Name)
    assert.Equal(t, "email", table.Columns[1].Name)
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/model/... -run TestTableRemoveColumn -v`
预期：FAIL（RemoveColumn 方法不存在）

- [ ] **步骤 3：实现 RemoveColumn 方法**

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

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/model/... -run TestTableRemoveColumn -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/model/table.go internal/model/table_test.go
git commit -m "feat(model): add Table.RemoveColumn method"
```

---

### 任务 6：解析器 - 新增 Mutation 类型和常量

**文件：**
- 修改：`internal/parser/mutation.go`
- 测试：`internal/parser/mutation_extended_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

```go
package parser

import (
    "testing"
    "github.com/fred29910/migra-go/internal/model"
    "github.com/stretchr/testify/assert"
)

func TestDropColumnMutation_Apply(t *testing.T) {
    schema := model.NewSchema()
    ns := schema.GetOrCreateNamespace("public")
    table := model.NewTable("public", "users")
    table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
    table.AddColumn(&model.Column{Name: "name", DataType: "varchar"})
    ns.Tables["users"] = table

    mut := DropColumnMutation{Schema: "public", Table: "users", Column: "name"}
    err := mut.Apply(schema)

    assert.NoError(t, err)
    assert.Nil(t, table.ColumnByName["name"])
    assert.Len(t, table.Columns, 1)
}

func TestSetNotNullMutation_Apply(t *testing.T) {
    schema := model.NewSchema()
    ns := schema.GetOrCreateNamespace("public")
    table := model.NewTable("public", "users")
    table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: true})
    ns.Tables["users"] = table

    mut := SetNotNullMutation{Schema: "public", Table: "users", Column: "id"}
    err := mut.Apply(schema)

    assert.NoError(t, err)
    assert.False(t, table.ColumnByName["id"].IsNullable)
}

func TestDropNotNullMutation_Apply(t *testing.T) {
    schema := model.NewSchema()
    ns := schema.GetOrCreateNamespace("public")
    table := model.NewTable("public", "users")
    table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
    ns.Tables["users"] = table

    mut := DropNotNullMutation{Schema: "public", Table: "users", Column: "id"}
    err := mut.Apply(schema)

    assert.NoError(t, err)
    assert.True(t, table.ColumnByName["id"].IsNullable)
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/parser/... -run "TestDropColumnMutation|TestSetNotNullMutation|TestDropNotNullMutation" -v`
预期：FAIL（类型未定义）

- [ ] **步骤 3：在 mutation.go 中添加新常量**

```go
const (
    MutKindCreateTable     MutationKind = "create_table"
    MutKindAddColumn       MutationKind = "add_column"
    MutKindCreateEnumType  MutationKind = "create_enum_type"
    MutKindCreateIndex     MutationKind = "create_index"
    // 新增
    MutKindDropColumn      MutationKind = "drop_column"
    MutKindAlterColumnType MutationKind = "alter_column_type"
    MutKindSetNotNull      MutationKind = "set_not_null"
    MutKindDropNotNull     MutationKind = "drop_not_null"
    MutKindSetDefault      MutationKind = "set_default"
    MutKindDropDefault     MutationKind = "drop_default"
)
```

- [ ] **步骤 4：在 mutation.go 末尾添加新 Mutation 类型**

```go
// DropColumnMutation describes dropping a column from a table.
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

// AlterColumnTypeMutation describes changing a column's data type.
type AlterColumnTypeMutation struct {
    Schema   string
    Table    string
    Column   string
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
    col.DataType = m.ToType
    return nil
}

// SetNotNullMutation describes setting a column to NOT NULL.
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

// DropNotNullMutation describes dropping NOT NULL from a column.
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

// SetDefaultMutation describes setting a default expression on a column.
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

// DropDefaultMutation describes dropping a default expression from a column.
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
        return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
    }
    col := table.ColumnByName[m.Column]
    if col == nil {
        return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.Column)
    }
    col.DefaultExpr = nil
    return nil
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`rtk go test ./internal/parser/... -v -count=1`
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/parser/mutation.go internal/parser/mutation_extended_test.go
git commit -m "feat(parser): add new Mutation types for ALTER TABLE support"
```

---

### 任务 7：解析器 - 扩展 AlterTableHandler

**文件：**
- 修改：`internal/parser/alter_table_handler.go`
- 测试：`internal/parser/alter_table_extended_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

```go
package parser

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestAlterTableDropColumn(t *testing.T) {
    p := NewParser()
    schema, err := p.ParseSQL(`
        CREATE TABLE users (id integer, name varchar);
        ALTER TABLE users DROP COLUMN name;
    `)
    assert.NoError(t, err)
    assert.Nil(t, schema.Schemas["public"].Tables["users"].ColumnByName["name"])
}

func TestAlterTableSetNotNull(t *testing.T) {
    p := NewParser()
    schema, err := p.ParseSQL(`
        CREATE TABLE users (id integer, name varchar);
        ALTER TABLE users ALTER COLUMN name SET NOT NULL;
    `)
    assert.NoError(t, err)
    assert.False(t, schema.Schemas["public"].Tables["users"].ColumnByName["name"].IsNullable)
}

func TestAlterTableDropNotNull(t *testing.T) {
    p := NewParser()
    schema, err := p.ParseSQL(`
        CREATE TABLE users (id integer, name varchar NOT NULL);
        ALTER TABLE users ALTER COLUMN name DROP NOT NULL;
    `)
    assert.NoError(t, err)
    assert.True(t, schema.Schemas["public"].Tables["users"].ColumnByName["name"].IsNullable)
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/parser/... -run "TestAlterTableDropColumn|TestAlterTableSetNotNull|TestAlterTableDropNotNull" -v`
预期：FAIL（子命令未处理）

- [ ] **步骤 3：扩展 alter_table_handler.go Handle 方法**

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
                    Schema: schemaName,
                    Table:  tableName,
                    Column: colName,
                    ToType: col.DataType,
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

// extractDefaultExpr extracts default expression from a node
func extractDefaultExpr(node pg_nodes.Node) string {
    if d, ok := node.(interface{ Deparse() string }); ok {
        var result string
        func() {
            defer func() {
                if r := recover(); r != nil {
                    result = fmt.Sprintf("%v", node)
                }
            }()
            result = d.Deparse()
        }()
        return result
    }
    return fmt.Sprintf("%v", node)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/parser/... -v -count=1`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/alter_table_handler.go internal/parser/alter_table_extended_test.go
git commit -m "feat(parser): extend AlterTableHandler with DROP COLUMN, SET/DROP NOT NULL, ALTER TYPE, SET/DROP DEFAULT"
```

---

### 任务 8：更新 .env.example

**文件：**
- 修改：`examples/.env.example`

- [ ] **步骤 1：更新 .env.example 为 MIGRA_ 前缀变量**

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

- [ ] **步骤 2：Commit**

```bash
git add examples/.env.example
git commit -m "docs(env): update .env.example to use MIGRA_ prefixed variables"
```

---

### 任务 9：运行完整测试验证

- [ ] **步骤 1：运行所有测试**

运行：`rtk go test ./... -v -count=1`
预期：PASS

- [ ] **步骤 2：运行 lint 检查**

运行：`rtk make lint`
预期：PASS（或仅有不影响功能的警告）

- [ ] **步骤 3：运行完整 CI**

运行：`rtk make ci`
预期：PASS

- [ ] **步骤 4：如有失败，修复并重新运行**

---

### 任务 10：更新文档

**文件：**
- 修改：`docs/configuration.md`
- 修改：`README.md`

- [ ] **步骤 1：更新 docs/configuration.md**

在配置说明中添加环境变量映射表和 Viper 绑定规则说明。

- [ ] **步骤 2：更新 README.md**

1. 更新环境变量说明为 MIGRA_ 前缀
2. 补充 ALTER TABLE 支持的子命令列表
3. 补充非事务性 DDL 检测说明

- [ ] **步骤 3：Commit**

```bash
git add docs/configuration.md README.md
git commit -m "docs: update configuration and README for new features"
```

---

## 自检清单

### 规格覆盖度

| 规格需求 | 对应任务 |
|----------|----------|
| 配置系统 SetEnvPrefix | 任务 1 |
| diff.go viper 绑定 | 任务 2 |
| push.go viper 绑定 | 任务 3 |
| db_loader pool 配置 + mysql:// 移除 | 任务 4 |
| Table.RemoveColumn | 任务 5 |
| 新 Mutation 类型 | 任务 6 |
| AlterTableHandler 扩展 | 任务 7 |
| .env.example 更新 | 任务 8 |
| 测试验证 | 任务 9 |
| 文档更新 | 任务 10 |

### 占位符扫描

- 无"待定"、"TODO"、"后续实现"
- 无"添加适当的错误处理"
- 无"为上述代码编写测试"（无实际测试代码）
- 无"类似任务 N"

### 类型一致性

- `DropColumnMutation` 在任务 6 定义，在任务 7 使用 ✅
- `SetNotNullMutation` 在任务 6 定义，在任务 7 使用 ✅
- `RemoveColumn` 在任务 5 定义，在任务 6 使用 ✅
- `extractDefaultExpr` 在任务 7 定义并使用 ✅
