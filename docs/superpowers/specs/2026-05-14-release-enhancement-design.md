# Release Enhancement Design

## 概述

增强现有的GitHub Release工作流，在生成Linux amd64二进制包的同时自动生成校验和（SHA256）和SBOM（Software Bill of Materials），并将这些制品作为Release资产上传。

## 目标

- 在现有的 `.github/workflows/release.yml` 工作流中添加步骤
- 生成二进制包的SHA256校验和文件
- 使用Syft工具生成SBOM（SPDX格式）
- 将校验和和SBOM作为Release资产上传
- 保持工作流的简洁和可维护性

## 架构

### 工作流增强

现有工作流步骤：
1. Checkout代码
2. 设置Go环境
3. 构建二进制包
4. 运行集成测试（可选）
5. 创建Release并上传二进制包

增强后工作流步骤：
1. Checkout代码
2. 设置Go环境
3. 构建二进制包
4. 生成SHA256校验和
5. 安装Syft并生成SBOM
6. 运行集成测试（可选）
7. 创建Release并上传所有制品（二进制包、校验和、SBOM）

### 制品说明

- `migra`：Linux amd64二进制包
- `migra.sha256`：二进制包的SHA256校验和文件
- `migra-sbom.spdx.json`：SBOM文件（SPDX JSON格式）

## 组件

### 工作流修改（.github/workflows/release.yml）

在现有工作流中添加以下步骤：

1. **校验和生成步骤**：在构建二进制包后，使用`shasum -a 256`生成校验和文件
2. **SBOM生成步骤**：安装Syft工具，扫描二进制包生成SPDX格式的SBOM
3. **制品上传**：将所有制品（二进制包、校验和、SBOM）传递给`softprops/action-gh-release`操作

### 数据流

```
源码 -> [Go构建] -> 二进制包
                         -> [校验和生成] -> 校验和文件
                         -> [Syft扫描] -> SBOM文件
所有制品 -> [Release创建] -> GitHub Release资产
```

## 错误处理

- 如果校验和生成失败，工作流终止并报告错误
- 如果SBOM生成失败（Syft安装或扫描失败），工作流终止并报告错误
- 集成测试失败不会阻止Release创建（保持现有行为）
- 所有步骤使用`set -e`确保任何命令失败时立即退出

## 测试策略

1. **单元测试**：现有的Go单元测试保持不变
2. **集成测试**：工作流中的集成测试步骤保持不变
3. **端到端测试**：通过在标签推送时触发工作流来验证：
   - 二进制包正确构建
   - 校验和文件存在且内容正确
   - SBOM文件存在且为有效的SPDX JSON
   - 所有三个文件都被上传为Release资产

## 实现注意事项

- 使用官方的Syft安装脚本确保工具版本一致
- SBOM生成使用`syft packages -o spdx-json`命令
- 校验和文件命名遵循`<binary-name>.sha256`的惯例
- 所有制品在同一步骤中上传以避免工作流复杂性
- 工作流继续使用`actions/checkout@v4`和`actions/setup-go@v5`

## 未来扩展性

此设计为未来扩展留有空间：
- 可以轻松添加其他平台的构建（通过矩阵策略）
- 可以添加更多制品类型（如签名、安装脚本）
- 可以替换为GoReleaser而不改变工作流的整体结构
- SBOM格式可以扩展为支持多种格式（CycloneDX等）

## 成功标准

- 在标签推送触发的工作流运行中：
  - 成功构建Linux amd64二进制包
  - 成功生成SHA256校验和文件
  - 成功生成SPDX格式的SBOM文件
  - 所有三个文件都被上传为GitHub Release资产
  - 工作流在所有步骤成功完成时结束