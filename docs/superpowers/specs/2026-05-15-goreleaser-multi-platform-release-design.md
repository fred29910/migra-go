# GoReleaser Multi-Platform Release Design

## 概述

使用 GoReleaser 替换现有的手动构建流程，实现 5 个平台的自动化二进制编译、打包（tar.gz/zip）、校验和与 SBOM 生成，并通过 GitHub Actions 发布到 GitHub Release。

## 目标

- 支持 5 个平台：`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Windows 打包为 zip，其他平台打包为 tar.gz
- 自动生成 SHA256 校验和文件
- 自动生成 SPDX 格式 SBOM
- 保留 CGO 依赖（`pg_query_go`），通过交叉编译链支持多平台
- 本地可通过 `make release-snapshot` 测试构建

## 架构

### 整体流程

```
git push --tags v*
       │
       ▼
GitHub Actions (release.yml)
       │
       ├─ job: build-linux-windows (ubuntu-latest)
       │     └─ GoReleaser ──→ linux/amd64  ──→ tar.gz
       │                       linux/arm64  ──→ tar.gz
       │                       windows/amd64 ──→ zip
       │                       → checksums.txt (partial)
       │                       → sbom.spdx.json
       │
       ├─ job: build-darwin (macos-latest)
       │     └─ GoReleaser ──→ darwin/amd64 ──→ tar.gz
       │                       darwin/arm64 ──→ tar.gz
       │                       → checksums.txt (partial)
       │                       → sbom.spdx.json
       │
       └─ job: create-release
             └─ 合并所有制品 → GitHub Release
                   ├─ migra-linux-amd64.tar.gz
                   ├─ migra-linux-arm64.tar.gz
                   ├─ migra-darwin-amd64.tar.gz
                   ├─ migra-darwin-arm64.tar.gz
                   ├─ migra-windows-amd64.zip
                   ├─ migra_checksums.txt
                   └─ migra.sbom.spdx.json
```

### 为什么需要两个 runner

Darwin 交叉编译在 Linux 上需要 osxcross + macOS SDK，配置复杂且不稳定。最可靠的方式是在 `macos-latest` runner 上原生编译 Darwin 二进制。因此拆分为两个 job：

- `build-linux-windows`：在 ubuntu 上编译 linux/amd64、linux/arm64、windows/amd64
- `build-darwin`：在 macos 上编译 darwin/amd64、darwin/arm64

### 为什么不用 GoReleaser 自带的 Release 功能

GoReleaser 的 `release` 配置只能在单个 runner 上运行。两个 runner 各自产出的制品需要在最后合并后统一上传，因此使用 `softprops/action-gh-release` 在 `create-release` job 中统一上传。

## 组件

### 1. `.goreleaser.yaml`（新增）

GoReleaser 核心配置，定义：

- **项目名**：`migra`
- **构建目标**：5 个平台，CGO_ENABLED=1
- **CC 交叉编译器**：
  - `linux/arm64` → `aarch64-linux-gnu-gcc`
  - `windows/amd64` → `x86_64-w64-mingw32-gcc`
  - `linux/amd64` → 默认 gcc（ubuntu 自带）
  - `darwin/*` → 默认 clang（macOS 自带）
- **打包格式**：默认 tar.gz，Windows 覆盖为 zip
- **ldflags**：注入版本信息（与现有 Makefile 对齐）
- **校验和**：生成 `migra_checksums.txt`（SHA256）
- **SBOM**：使用 syft 生成 `migra.sbom.spdx.json`（SPDX 格式）
- **dist 目录**：`./dist`

### 2. `Makefile`（修改）

新增目标：

- `release-local`：调用 `goreleaser release --clean`（完整发布，需 GITHUB_TOKEN）
- `release-snapshot`：调用 `goreleaser release --snapshot --clean`（本地测试，不发布）
- `install-tools`：安装 GoReleaser CLI（`go install`）

保留现有目标不变（`build`, `test`, `clean`, `lint`, `fmt`, `vet`, `ci`, `help`）。

### 3. `.github/workflows/release.yml`（改造）

从单 job 改造为三 job：

1. **build-linux-windows**：
   - runner: `ubuntu-latest`
   - 安装交叉编译链：`gcc-aarch64-linux-gnu`, `gcc-mingw-w64`
   - 运行 `goreleaser release --clean --skip=validate,publish`（跳过 publish，由最后 job 处理）
   - 上传制品为 artifact

2. **build-darwin**：
   - runner: `macos-latest`
   - 运行 `goreleaser release --clean --skip=validate,publish`
   - 上传制品为 artifact

3. **create-release**：
   - needs: `[build-linux-windows, build-darwin]`
   - 下载两个 artifact
   - 合并校验和文件
   - 使用 `softprops/action-gh-release@v2` 上传所有制品
   - token: `secrets.RELEASE_TOKEN`

### 4. `scripts/release.sh`（修改）

保留：
- 版本号格式验证
- 工作区干净检查
- 运行测试
- CHANGELOG 更新提示
- 创建标签并推送

移除：
- 手动构建步骤（由 CI 处理）

## 版本信息注入

GoReleaser 自动通过 `-ldflags` 注入以下变量（与现有 `internal/version` 包对齐）：

```
-X github.com/fred29910/migra-go/internal/version.Version={{ .Tag }}
-X github.com/fred29910/migra-go/internal/version.BuildTime={{ .Date }}
-X github.com/fred29910/migra-go/internal/version.GitCommit={{ .ShortCommit }}
-X github.com/fred29910/migra-go/internal/version.GoVersion={{ .GoVersion }}
```

## 错误处理

- GoReleaser 任一平台构建失败 → 整个 job 失败，不产生制品
- 任一 build job 失败 → create-release job 跳过，Release 不创建
- 交叉编译器缺失 → GoReleaser 报错，job 失败
- `snapshot` 模式支持本地测试，不创建 Release，不验证 token

## 测试策略

1. **本地测试**：运行 `make release-snapshot`，验证：
   - 所有平台二进制正确编译
   - 打包格式正确（Windows=zip，其他=tar.gz）
   - 校验和文件存在
   - SBOM 文件存在且为有效 SPDX JSON
2. **CI 测试**：推送标签触发完整流程，验证：
   - 所有 5 个平台制品上传
   - 校验和与二进制匹配
   - Release 创建成功

## 依赖

- GoReleaser CLI（`github.com/goreleaser/goreleaser`）
- ubuntu 交叉编译链：`gcc-aarch64-linux-gnu`, `gcc-mingw-w64`
- Syft（GoReleaser 自动安装用于 SBOM 生成）

## 未来扩展

- 轻松添加新平台（在 `.goreleaser.yaml` 的 `builds` 中添加条目）
- 支持签名（GoReleaser 内置 GPG 签名支持）
- 支持 Homebrew tap（GoReleaser 内置 brew 支持）
- 支持 Docker 镜像（GoReleaser 内置 docker 支持）

## 成功标准

- `git tag v0.X.Y && git push --tags` 触发完整流程
- GitHub Release 包含 5 个平台的打包二进制
- 每个二进制可在对应平台独立运行
- 校验和文件与二进制匹配
- SBOM 文件为有效 SPDX JSON
- `make release-snapshot` 可在本地成功执行
