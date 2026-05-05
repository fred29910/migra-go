# 贡献指南

感谢您考虑为 migra-go 做出贡献！

## 如何贡献

### 报告 Bug

- 使用 [GitHub Issues](https://github.com/fred29910/migra-go/issues) 提交 Bug 报告
- 请包含：复现步骤、预期行为、实际行为、环境信息

### 提交 Pull Request

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: 添加某个功能'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 开发规范

### 代码风格

- 使用 `make fmt` 格式化代码
- 使用 `make lint` 检查代码质量
- 遵循 Go 官方代码规范

### 提交信息

本项目使用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) 规范：

```
<类型>[可选的作用域]: <描述>

[可选的正文]

[可选的脚注]
```

类型列表：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档变更
- `style`: 代码格式（不影响代码运行的变动）
- `refactor`: 重构（既不是新功能也不是修复）
- `perf`: 性能优化
- `test`: 测试相关
- `chore`: 构建过程或辅助工具变动

示例：
```bash
feat(parser): 添加对 CREATE INDEX 的解析支持
fix(diff): 修复列类型比较时的空指针问题
docs: 更新 README 安装说明
```

### 测试要求

- 新功能必须包含测试
- 运行 `make test` 确保所有测试通过
- 优先使用表驱动测试
- 数据库相关测试使用 `-short` 标志跳过

### 分支策略

- `main`: 稳定版本，对应最新 release
- `develop`: 开发分支，PR 合并目标
- 功能分支: `feature/xxx`, `fix/xxx`, `docs/xxx`

## 开发环境搭建

```bash
git clone https://github.com/fred29910/migra-go.git
cd migra-go
go mod download
make build
```

## 行为准则

请阅读 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) 了解我们的行为规范。

## 问题咨询

如有疑问，请：
- 查看现有 [Issues](https://github.com/fred29910/migra-go/issues)
- 创建新的 Issue 并添加 `question` 标签
