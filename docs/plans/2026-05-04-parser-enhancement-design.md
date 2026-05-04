# Parser 完善设计（修正版）：基于 pg_query_go 的 AST 解析

## 目标

完整实现 planv2.md 要求的 parser MVP：
1. 使用 `pg_query_go` 生成 PostgreSQL AST
2. 通过类型断言提取 DDL 语义
3. 写入 SchemaModel
4. 支持错误定位和报告

## 修正：实际 API 分析

### pg_query_go 真实 API

1. **解析入口**：
   ```go
   // pg_query.Parse() 返回 (ParseTreeList, error)
   tree, err := pg_query.Parse(sql)
   // ParseTreeList 结构：
   type ParseTreeList struct {
       Statements []nodes.Node  // 注意：是 []nodes.Node，不是 []RawStmt
   }
   ```

2. **节点类型断言**：
   - `pg_query_go` 使用 `nodes` 包定义所有节点类型
   - 节点是**值类型**（如 `nodes.CreateStmt`），不是指针类型
   - 类型断言方式：
     ```go
     switch n := stmt.(type) {
     case nodes.CreateStmt:
         // 处理 CREATE TABLE
     case nodes.AlterTableStmt:
         // 处理 ALTER TABLE
     case nodes.IndexStmt:
         // 处理 CREATE INDEX
     default:
         return fmt.Errorf("unsupported statement type: %T", stmt)
     }
     ```

3. **重要**：`pg_query.Parse()` 返回的是 `[]nodes.Node`，每个元素需要类型断言为具体节点类型。

## 修正后的架构设计

### 整体流程
```
SQL → pg_query.Parse() → []nodes.Node → 类型断言 → 提取语义 → model.Schema
```

### 解析入口（修正）
```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    // 1. 重置状态
    p.schema = model.NewSchema()
    p.errors = p.errors[:0]

    // 2. 使用 pg_query 解析 SQL 为 AST
    tree, err := pg_query.Parse(sql)
    if err != nil {
        return nil, fmt.Errorf("pg_query parse failed: %w", err)
    }

    // 3. 遍历 AST 语句（注意：是 []nodes.Node）
    for _, stmt := range tree.Statements {
        if err := p.visitNode(stmt); err != nil {
            p.errors = append(p.errors, err)
        }
    }

    if len(p.errors) > 0 {
        return p.schema, fmt.Errorf("parsing completed with %d errors", len(p.errors))
    }
    return p.schema, nil
}

// visitNode 分发节点到具体处理函数（修正：使用值类型断言）
func (p *Parser) visitNode(stmt nodes.Node) error {
    switch n := stmt.(type) {
    case nodes.CreateStmt:
        return p.handleCreateTable(n)
    case nodes.AlterTableStmt:
        return p.handleAlterTable(n)
    case nodes.IndexStmt:
        return p.handleCreateIndex(n)
    case nodes.CreateEnumStmt:
        return p.handleCreateEnum(n)
    default:
        // 获取语句位置（如果节点支持）
        pos := -1
        if rl, ok := stmt.(interface{ GetStmtLocation() int }); ok {
            pos = rl.GetStmtLocation()
        }
        return &ParseError{
            Message:   fmt.Sprintf("unsupported statement type: %T", stmt),
            Position:  pos,
            Statement: truncateSQL(sql, stmt, 100), // 需要从原始 SQL 提取
        }
    }
}
```

### 节点处理细节（修正返回值）

#### CreateStmt（CREATE TABLE）
```go
func (p *Parser) handleCreateTable(stmt nodes.CreateStmt) error {
    // 1. 提取表名和 schema
    tableName, schemaName := p.parseRelation(stmt.Relation)
    
    // 2. 创建表对象
    table := model.NewTable(schemaName, tableName)
    
    // 3. 遍历 TableElts
    // 注意：TableElts 是 nodes.List，需要迭代
    for _, item := range stmt.TableElts.Items {
        switch elt := item.(type) {
        case nodes.ColumnDef:
            col := p.parseColumnDef(elt)
            if col != nil {
                table.AddColumn(col)
            }
        case nodes.Constraint:
            p.parseTableConstraint(elt, table)
        }
    }
    
    // 4. 存储到 schema
    ns := p.schema.GetOrCreateNamespace(schemaName)
    ns.Tables[tableName] = table
    return nil
}
```

#### AlterTableStmt（ALTER TABLE）
```go
func (p *Parser) handleAlterTable(stmt nodes.AlterTableStmt) error {
    tableName, schemaName := p.parseRelation(stmt.Relation)
    
    // 遍历 Cmds（子命令）
    for _, item := range stmt.Cmds.Items {
        cmd, ok := item.(nodes.AlterTableCmd)
        if !ok {
            continue
        }
        
        switch cmd.Subtype {
        case nodes.AT_AddColumn:
            if cmd.Def != nil {
                col := p.parseColumnDef(*cmd.Def.(nodes.ColumnDef))
                if col != nil {
                    table.AddColumn(col)
                }
            }
        case nodes.AT_AlterColumnType:
            // 修改列类型
        case nodes.AT_DropColumn:
            // 删除列
        }
    }
    return nil
}
```

## 修正：错误处理与位置信息

### ParseError 结构（修正）
```go
type ParseError struct {
    Message   string
    Position  int    // 语句起始位置（来自 stmt_location）
    StmtLen  int    // 语句长度（来自 stmt_len）
    Statement string // 原始 SQL 语句片段
}

// 获取位置信息的通用方法
func getStmtLocation(stmt nodes.Node) int {
    if rl, ok := stmt.(interface{ GetStmtLocation() int }); ok {
        return rl.GetStmtLocation()
    }
    return -1
}
```

### 位置信息提取
- `pg_query_go` 的 `RawStmt` 包含 `StmtLocation` 和 `StmtLen`
- 但 `Parse()` 返回的是 `[]nodes.Node`，需要先判断是否为 `RawStmt`
- 或者，直接用 `pg_query.ParseToJSON()` 然后手动解析 JSON 获取位置信息

## 修正：测试策略

### 问题：当前集成测试是占位
修正：创建真正的集成测试
```go
func TestIntegrationParserToDiff(t *testing.T) {
    // 1. 解析源 SQL
    p1 := parser.NewParser()
    sourceSchema, err := p1.ParseSQL(sourceSQL)
    if err != nil {
        t.Fatalf("Parse source failed: %v", err)
    }

    // 2. 解析目标 SQL
    p2 := parser.NewParser()
    targetSchema, err := p2.ParseSQL(targetSQL)
    if err != nil {
        t.Fatalf("Parse target failed: %v", err)
    }

    // 3. 运行 diff
    d := diff.NewDiffer()
    operations := d.Diff(sourceSchema, targetSchema)

    // 4. 渲染 SQL
    r := render.NewRenderer()
    sql := r.RenderAll(operations)

    // 5. 验证 SQL 包含预期内容
    if !strings.Contains(sql, "ALTER TABLE") {
        t.Error("expected ALTER TABLE statement")
    }
}
```

## 修正：约束处理与下游对齐

### 问题：parser 解析了约束，但 diff/render 未完整支持
解决方案（选择一种）：

**选项 A**：降低 parser MVP 范围，暂不支持约束解析
- 文档明确：MVP 仅支持 CREATE TABLE + ALTER TABLE ADD COLUMN
- 约束解析延后到 Phase 5

**选项 B**：补齐 diff/render 的约束处理
- diff 模块添加 `AddConstraintOp`、`DropConstraintOp`
- render 模块添加约束渲染分支

**推荐**：选项 A（降低范围，确保链路完整）

## 实施计划（修正版）

1. **重写 parser.go**：
   - 使用 `pg_query.Parse()` 获取 `ParseTreeList`
   - 实现 `visitNode()` 用值类型断言分发
   - 实现 `handleCreateTable()`（仅处理 ColumnDef）
   - 实现 `handleAlterTable()`（仅处理 AT_AddColumn）
   - 其他节点返回 `ParseError`

2. **修正错误处理**：
   - `ParseError` 包含 `Position` 和 `StmtLen`
   - 从 `pg_query.ParseToJSON()` 结果提取位置信息

3. **更新测试**：
   - 创建真正的集成测试（parser → diff → render）
   - 更新 `TestIntegrationParserToDiff` 非空实现

4. **降低范围声明**：
   - 在文档/代码中明确 MVP 范围
   - 避免"解析了但用不上"的断层

## 成功标准（修正）

1. ✅ 使用 `pg_query_go` 正确解析 MVP 节点（CreateStmt、AlterTableStmt）
2. ✅ 不支持的语句返回可读错误（含位置信息）
3. ✅ 所有 parser 测试通过
4. ✅ **真正的**集成测试通过（parser → diff → render 链路）
5. ✅ 明确 MVP 范围，与下游（diff/render）能力对齐
