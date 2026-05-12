# Push 子命令设计规格

## 概述

实现 `migra push` 子命令，基于 diff 功能将生成的 SQL 应用到目标数据库。

## 背景

当前 migra-go 仅有 `diff` 子命令用于比较 schema 并输出 SQL 差异。用户需要手动将输出的 SQL 复制到数据库执行。实现 push 子命令可简化工作流，直接在 CLI 中完成 diff + apply。

## 目标

1. 在 diff 基础上增加 push 子命令
2. 支持交互式确认执行每条 SQL
3. 事务保护，失败自动回滚
4. 强制 dry-run 模式，必须确认后才真正执行
5. 危险操作（DROP）需要额外确认

## 设计

### 架构

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Source &   │────▶│   Diff       │────▶│   Plan       │────▶│   Push       │
│   Target DB │     │  (Compute)   │     │  (DAG Sort)  │     │  (Execute)   │
└─────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
                                                                   │
                                                            ┌──────▼───────┐
                                                            │ 事务管理     │
                                                            │ - 预览模式   │
                                                            │ - 交互确认   │
                                                            │ - 错误回滚   │
                                                            └──────────────┘
```

### CLI 设计

```bash
# 基本用法
migra push [source] [target]

# 参数说明
# source: SQL 文件或数据库连接串
# target: 必须是数据库连接串

# 标志
migra push [source] [target] --dry-run    # 默认，显示SQL但不执行
migra push [source] [target] --execute    # 执行确认的SQL
migra push [source] [target] --schema public  # 指定schema
migra push [source] [target] --unsafe-drop     # 允许危险DROP操作
```

### 交互流程

1. 计算 diff，生成 SQL 操作列表
2. 显示预览模式：列出所有将要执行的 SQL 及序号
3. 循环询问用户确认：
   - `y` + 回车：执行当前这条
   - `a` + 回车：切换到自动执行模式，后续 SQL 不再逐条询问，直接执行剩余所有
   - `n` + 回车：取消并退出
   - `s` + 回车：跳过当前这条，继续下一条
4. 确认后进入执行阶段

### 执行阶段

1. 开启数据库事务
2. 逐条执行用户确认的 SQL
3. 任意 SQL 执行失败：
   - 打印错误信息
   - 执行 ROLLBACK
   - 退出并提示用户
4. 全部成功则 COMMIT
5. 显示执行结果摘要

### 危险操作检测

- 检测 DROP 操作（drop_table, drop_column, drop_index, drop_constraint, drop_enum_type）
- 遇到 DROP 时，显示警告：
  ```
  ⚠️  危险操作检测到：
  3: DROP TABLE users;
  
  确认执行此危险操作? (y/n)
  ```

### 错误处理

| 场景 | 处理 |
|------|------|
| target 不是数据库连接 | 报错退出，提示 target 必须是数据库 |
| 数据库连接失败 | 报错退出，显示连接错误 |
| SQL 执行失败 | 事务回滚，退出并显示失败 SQL |
| 用户取消 | 退出，不执行任何 SQL |

### 实现位置

```
cmd/migra/
├── main.go           # 注册 push 子命令
├── push.go          # push 子命令定义
└── push_runner.go   # push 执行逻辑
```

## 依赖

- 复用 `diff` 的计算逻辑（`app.ComputeDiff`）
- 复用 `app.DiffService` 的加载逻辑
- 使用 `pgx` 的事务支持

## 边界情况处理

### 1. 非事务性 DDL 处理

PostgreSQL 绝大多数 DDL 支持事务，但以下操作严禁在事务块中运行：
- `CREATE INDEX CONCURRENTLY`
- `DROP INDEX CONCURRENTLY`
- `REINDEX INDEX CONCURRENTLY`
- `CREATE INDEX USING CONCURRENTLY`

**处理策略**：
- 检测到此类操作时，报错提示用户：
  ```
  ⚠️  检测到非事务性 DDL：
  CREATE INDEX CONCURRENTLY idx_name ON table(col);
  
  此操作无法在事务中执行。请使用以下方式单独执行：
  - 在事务外单独执行该 SQL
  - 或使用 pg_restore 等工具
  ```
- 暂时不支持分段执行，保持事务一致性

### 2. 自动模式下的 DROP 安全性

**处理策略**：
- 用户输入 `a`（自动执行模式）后，遇到 `IsDestructive() = true` 的操作时，强制中断自动状态
- 再次提示用户确认该危险操作：
  ```
  ⚠️  危险操作检测到（自动模式中断）：
  7: DROP TABLE old_users;
  
  确认执行此危险操作? (y/n)
  ```
- 如果用户启动了 `--unsafe-drop` 标志，则跳过此额外确认

### 3. 执行后的状态校验

**处理策略**：
- 所有 SQL 执行完毕并 COMMIT 后
- 自动重新执行一次 in-memory diff（复用已加载的 source 和重新加载 target）
- 如果返回结果不为空，说明迁移不完全符合预期
- 显示警告：
  ```
  ⚠️  迁移后校验：检测到残留差异
  以下操作未能完全应用：
  <diff 输出>
  ```
- 此步骤为可选，可通过 `--no-verify` 标志跳过

### 4. 信号处理 (Ctrl+C)

**处理策略**：
- 监听 `SIGINT` (Ctrl+C) 和 `SIGTERM` 信号
- 捕获信号时：
  - 执行 ROLLBACK 回滚未提交的事务
  - 关闭数据库连接
  - 显示友好退出信息：
    ```
    ⚠️  操作已中断，事务已回滚
    ```

### 5. 渲染一致性

**处理策略**：
- 在 `internal/render` 包中暴露 `RenderSingle(op diff.Operation) string` 方法
- 交互确认时显示的单条 SQL 与最终执行的 SQL 完全一致
- 该方法可被 push 模块调用，确保 UI 与实际执行一致

## 测试

1. 单元测试：交互确认逻辑、自动模式中断
2. 集成测试：与真实数据库的 push 执行
3. 边界测试：DROP 操作检测、事务回滚、非事务性 DDL 报错
4. 信号测试：Ctrl+C 时的回滚行为