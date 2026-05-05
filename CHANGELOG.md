# 变更日志

本项目的所有显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

### 重构 (Refactor)
- refactor(cli): split diff pipeline into testable stages
- refactor: inject diff/plan/render via interfaces
- refactor(render): remove renderer mutable SQL buffer

### 特性 (Features)
- feat(cli): add timeout flag and context propagation
- feat(diff): report warnings for unsupported changes
- feat(diff): add minimal add/drop constraint pipeline

### 性能 (Performance)
- perf(plan): optimize DAG dedupe and queue traversal

### 新增
- 添加 GitHub Actions CI/CD 配置（test、lint、release）
- 添加开源规范文件（LICENSE、CONTRIBUTING.md、CODE_OF_CONDUCT.md）
- 添加示例配置文件

### 变更
- 修正模块路径为 `github.com/fred29910/migra-go`
- 重命名 CLI 入口为 `cmd/migra`
- 更新 Makefile，添加 `vet`、`ci` target

### 移除
- 移除旧的 `cmd/schemadiff` 目录

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

[未发布]: https://github.com/fred29910/migra-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/fred29910/migra-go/releases/tag/v0.1.0
