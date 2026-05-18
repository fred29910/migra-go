# 变更日志

本项目的所有显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

### 杂务 (Chores)
- chore: 更新 release.sh，移除手动构建步骤

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

[未发布]: https://github.com/fred29910/migra-go/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/fred29910/migra-go/releases/tag/v0.1.5
[0.1.1]: https://github.com/fred29910/migra-go/releases/tag/v0.1.1
[0.1.0]: https://github.com/fred29910/migra-go/releases/tag/v0.1.0
