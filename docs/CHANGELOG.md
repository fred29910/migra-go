# 变更日志

本项目的所有显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

### 杂务 (Chores)
- chore: 更新 release.sh，移除手动构建步骤

## [0.3.1] - 2026-06-15

### 重构 (Refactor)
- refactor(diff/render): 拆分大文件为多个小模块提升可维护性
- refactor(util): 提取公共函数消除代码重复
- refactor(errors): 添加哨兵错误并统一错误处理

### 修复 (Bug Fixes)
- fix: use fmt.Fprintf instead of WriteString(fmt.Sprintf) (QF1012)
- fix: resolve golangci-lint issues (gofmt, errcheck, unused code)

### 测试 (Tests)
- test: add tests for 4 high-risk fixes
- test(cmd/migra): 补充 cmd/migra 包测试
- test(model): 补充 internal/model 包测试
- test(parserutil): 添加 internal/parserutil 包单元测试
- test(introspect): 添加 internal/introspect 包单元测试
- test(diff): 添加 diff 上下文测试

### 文档 (Documentation)
- docs: 更新文档以反映代码实际状态
- docs: 同步架构文档、DDL 矩阵和配置文档
- docs: 统一 bug 文档状态格式为已修复
- docs: 标记 5 个已知 bug 为已修复状态
- docs(CONTRIBUTING): 更新贡献指南，添加代码审查流程和覆盖率要求

## [0.3.0] - 2026-06-15

### 重构 (Refactor)
- refactor: 代码质量改进——重构渲染器从巨型 switch 为多态分发模式
- refactor(parser): 提取 WarningEmitter 回调，消除 os.Stderr 硬编码
- refactor(parser): recover() 捕获 panic 时记录完整 stack trace

### 特性 (Features)
- feat(diff/render): 物化视图作为独立 Op 类型（create_materialized_view / drop_materialized_view），共 34 种 Operation
- feat(errors): 新增结构化 Error 类型（Code/Message/Cause），支持 wrapping

### 修复 (Bug Fixes)
- fix(render/push/source): 修复代码评审中的 3 个问题
- fix: 修复 diff_columns_test.go 编译错误

### 测试 (Tests)
- test: 补充 IDENTITY、COLLATE、VIEW/SEQUENCE/EXTENSION 测试用例
- test: 添加 integration tests for new DDL feature testdata
- test(cmd/migra): 提升测试覆盖率从 41.2% 到 63.1%

### 文档 (Documentation)
- docs: 添加使用示例和 API 参考文档
- docs: 添加项目改进设计文档
- docs: 更新 testdata/README.md with new DDL feature testdata

## [0.2.2] - 2026-06-03

### 修复 (Bug Fixes)
- fix(render): 修复 RenderJSON 输出与文档规范不一致的问题
- fix(render): 修复 View 与 Materialized View 渲染模型不一致的问题
- fix(render): 修复 DROP EXTENSION 生成 schema-qualified 名称导致 SQL 无效的问题
- fix(render): 修复 ALTER SEQUENCE 对 CYCLE 仅能打开不能关闭的问题
- fix(parser): 修复 ALTER TABLE SET DEFAULT 默认值提取逻辑错误
- fix(push): 修复 push 交互流程复用 30s 超时上下文问题
- fix(push): 修复 push 中断处理使用 goroutine + os.Exit(1) 的问题
- fix(source): 修复 strict 行为在不同 source loader 上不一致的问题

### 测试 (Tests)
- test: 补充 IDENTITY、COLLATE、VIEW/SEQUENCE/EXTENSION 测试用例
- test: 补充 render 层 rename、view、sequence、extension 渲染测试

### 文档 (Documentation)
- docs: 同步架构文档与实际代码实现
- docs: 更新 DDL 特性支持矩阵
- docs: 添加代码评审报告

## [0.2.1] - 2026-06-03

> 注：此版本与 v0.2.2 同日发布，包含不同的修复集

### 修复 (Bug Fixes)
- fix(render): 修复 RenderJSON 输出与文档规范不一致的问题
- fix(render): 修复 View 与 Materialized View 渲染模型不一致的问题
- fix(render): 修复 DROP EXTENSION 生成 schema-qualified 名称导致 SQL 无效的问题
- fix(render): 修复 ALTER SEQUENCE 对 CYCLE 仅能打开不能关闭的问题
- fix(parser): 修复 ALTER TABLE SET DEFAULT 默认值提取逻辑错误
- fix(push): 修复 push 交互流程复用 30s 超时上下文问题
- fix(push): 修复 push 中断处理使用 goroutine + os.Exit(1) 的问题
- fix(source): 修复 strict 行为在不同 source loader 上不一致的问题

### 测试 (Tests)
- test: 补充 IDENTITY、COLLATE、VIEW/SEQUENCE/EXTENSION 测试用例
- test: 补充 render 层 rename、view、sequence、extension 渲染测试

### 文档 (Documentation)
- docs: 同步架构文档与实际代码实现
- docs: 更新 DDL 特性支持矩阵
- docs: 添加代码评审报告

## [0.2.0] - 2026-05-21

### 特性 (Features)
- feat(parser): 添加 CREATE VIEW 解析支持（CreateViewHandler）
- feat(parser): 添加 CREATE SEQUENCE 解析支持（CreateSequenceHandler）
- feat(parser): 添加 CREATE EXTENSION 解析支持（CreateExtensionHandler）
- feat(introspect): 添加视图、序列、扩展的数据库内省支持
- feat(diff): 添加视图、序列、扩展的差异比较操作
- feat(render): 添加视图、序列、扩展的 SQL 渲染支持
- feat(diff): 添加 IDENTITY 列差异检测（add/set/drop identity）
- feat(diff): 添加列排序规则差异检测（alter_column_collation）
- feat(render): 添加 IDENTITY 列和列排序规则的 SQL 渲染

### 修复 (Bug Fixes)
- fix(parser): 修复多 Schema diff 中 CreateSchemaStmt 解析失败问题
- fix(directory-diff): 修复目录差异比较评审中的问题

### 文档 (Documentation)
- docs: 添加 DDL 特性扩展支持分析文档
- docs: 添加深度技术评审与优化报告

## [0.1.5] - 2026-05-18

### 特性 (Features)
- feat: 添加 GoReleaser 配置（linux/amd64, linux/arm64, windows/amd64）
- feat: 添加 GoReleaser Darwin 配置（darwin/amd64, darwin/arm64）
- feat(Makefile): 补充 release-local / release-snapshot / install-tools 目标
- feat(ci): 改造 release.yml 为三 job 多平台构建流程

### 修复 (Bug Fixes)
- fix(release): 添加 GitHub Release 的 contents:write 权限并使用 RELEASE_TOKEN

### 文档 (Documentation)
- docs: 同步项目目录结构与实际代码库
- docs: 添加 GitHub Release 403 权限拒绝 Bug 报告
- docs: GoReleaser 多平台 Release 设计方案与实现计划

## [0.1.1] - 2026-05-15

### 特性 (Features)
- feat: 实现目录差异比较（DirectoryLoader 递归扫描与合并）
- feat: 添加 DirectoryLoader.Match() 与 Load() 实现
- feat: 实现目录差异比较集成测试与设计文档
- feat(version): 添加 --version/-v 标志和版本信息包
- feat(release): 添加 checksum 和 SBOM 生成到 Release 工作流
- feat(testdata): 添加测试数据文件

### 修复 (Bug Fixes)
- fix(source): 修复 DirectoryLoader 跨文件 DDL 依赖解析失败问题
- fix(directory-diff): 修复评审中的 5 个问题
- fix(render): 修复 RenderJSON 输出与文档规范不一致的问题
- fix: 修复多 Schema diff 中 CreateSchemaStmt 解析失败及 --schema 过滤无效问题
- fix: 移除 multi_schema/v2 中冲突的 snapshot.sql 文件

### 重构 (Refactor)
- refactor(dir_loader): 提取 stripFileScheme 消除重复代码，补充 file:// 目录测试

### 测试 (Tests)
- test: 添加 testdata 快照、边界场景和 v3 schema

### 文档 (Documentation)
- docs: 添加 PostgreSQL DDL 特性支持矩阵
- docs: 版本信息设计文档与实现计划

## [0.1.0] - 2026-05-04

### 新增
- 初始版本发布
- 实现 SQL 解析（基于 pg_query_go）
- 实现数据库内省（pg_catalog）
- 实现差异比较引擎
- 实现执行计划（DAG 排序）
- 实现 SQL 渲染器
- 添加 CLI 工具（cobra + viper）
- 支持 db-less SQL 文件比较
- 支持 PostgreSQL 数据库连接
- 添加破坏性变更诊断
- 添加单元测试和集成测试框架

[未发布]: https://github.com/fred29910/migra-go/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/fred29910/migra-go/releases/tag/v0.3.1
[0.3.0]: https://github.com/fred29910/migra-go/releases/tag/v0.3.0
[0.2.2]: https://github.com/fred29910/migra-go/releases/tag/v0.2.2
[0.2.1]: https://github.com/fred29910/migra-go/releases/tag/v0.2.1
[0.2.0]: https://github.com/fred29910/migra-go/releases/tag/v0.2.0
[0.1.5]: https://github.com/fred29910/migra-go/releases/tag/v0.1.5
[0.1.1]: https://github.com/fred29910/migra-go/releases/tag/v0.1.1
[0.1.0]: https://github.com/fred29910/migra-go/releases/tag/v0.1.0
