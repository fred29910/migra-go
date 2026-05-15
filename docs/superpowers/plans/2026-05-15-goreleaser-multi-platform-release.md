# GoReleaser 多平台 Release 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 使用 GoReleaser 实现 5 个平台（linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64）的自动化二进制编译、打包（tar.gz/zip）、校验和与 SBOM 生成，并通过 GitHub Actions 发布到 GitHub Release。

**架构：** 创建 `.goreleaser.yaml` 定义多平台构建配置，改造 GitHub Actions release.yml 为三 job 流程（build-linux-windows → build-darwin → create-release），在 Makefile 中补充 release 相关目标。保留 CGO 依赖，通过交叉编译链支持多平台。

**技术栈：** GoReleaser、GitHub Actions、CGO 交叉编译链（aarch64-linux-gnu-gcc, x86_64-w64-mingw32-gcc）、Syft（SBOM）

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `.goreleaser.yaml` | 创建 | GoReleaser 核心配置：构建目标、交叉编译器、打包格式、ldflags、校验和、SBOM |
| `.goreleaser-darwin.yaml` | 创建 | Darwin 专用 GoReleaser 配置（macos runner 使用） |
| `Makefile` | 修改 | 新增 `release-local`、`release-snapshot`、`install-tools` 目标 |
| `.github/workflows/release.yml` | 修改 | 改造为三 job：build-linux-windows、build-darwin、create-release |
| `scripts/release.sh` | 修改 | 移除手动构建步骤，保留标签和推送逻辑 |

---

### 任务 1：创建 `.goreleaser.yaml`（Linux + Windows 构建配置）

**文件：**
- 创建：`.goreleaser.yaml`

- [ ] **步骤 1：创建 `.goreleaser.yaml`**

```yaml
# .goreleaser.yaml
# GoReleaser 配置 — 用于 ubuntu-latest runner（linux/amd64, linux/arm64, windows/amd64）

project_name: migra

env:
  - CGO_ENABLED=1

builds:
  - binary: migra
    main: ./cmd/migra
    goos:
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    # windows/arm64 排除（不常用）
    ignore:
      - goos: windows
        goarch: arm64
    env:
      - CGO_ENABLED=1
    ldflags:
      - -s -w
      - -X github.com/fred29910/migra-go/internal/version.Version={{ .Tag }}
      - -X github.com/fred29910/migra-go/internal/version.BuildTime={{ .Date }}
      - -X github.com/fred29910/migra-go/internal/version.GitCommit={{ .ShortCommit }}
      - -X github.com/fred29910/migra-go/internal/version.GoVersion={{ .GoVersion }}
    overrides:
      - goos: linux
        goarch: arm64
        env:
          - CC=aarch64-linux-gnu-gcc
          - CGO_ENABLED=1
      - goos: windows
        goarch: amd64
        env:
          - CC=x86_64-w64-mingw32-gcc
          - CGO_ENABLED=1

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: >-
      {{ .ProjectName }}-
      {{- .Os }}-
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

checksum:
  name_template: "checksums-linux-windows.txt"
  algorithm: sha256

sboms:
  - id: spdx
    artifacts: binary
    documents:
      - "{{ .ProjectName }}.sbom.spdx.json"
    args: ["$artifact", "--output", "spdx-json=$document"]

dist: dist
```

- [ ] **步骤 2：Commit**

```bash
git add .goreleaser.yaml
git commit -m "feat: 添加 GoReleaser 配置（linux/amd64, linux/arm64, windows/amd64）"
```

---

### 任务 2：创建 `.goreleaser-darwin.yaml`（Darwin 构建配置）

**文件：**
- 创建：`.goreleaser-darwin.yaml`

- [ ] **步骤 1：创建 `.goreleaser-darwin.yaml`**

```yaml
# .goreleaser-darwin.yaml
# GoReleaser 配置 — 用于 macos-latest runner（darwin/amd64, darwin/arm64）

project_name: migra

env:
  - CGO_ENABLED=1

builds:
  - binary: migra
    main: ./cmd/migra
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    env:
      - CGO_ENABLED=1
    ldflags:
      - -s -w
      - -X github.com/fred29910/migra-go/internal/version.Version={{ .Tag }}
      - -X github.com/fred29910/migra-go/internal/version.BuildTime={{ .Date }}
      - -X github.com/fred29910/migra-go/internal/version.GitCommit={{ .ShortCommit }}
      - -X github.com/fred29910/migra-go/internal/version.GoVersion={{ .GoVersion }}

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}-
      {{- .Os }}-
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

checksum:
  name_template: "checksums-darwin.txt"
  algorithm: sha256

# Darwin 不生成 SBOM（由 linux job 生成）

dist: dist
```

- [ ] **步骤 2：Commit**

```bash
git add .goreleaser-darwin.yaml
git commit -m "feat: 添加 GoReleaser Darwin 配置（darwin/amd64, darwin/arm64）"
```

---

### 任务 3：修改 Makefile — 补充 release 目标

**文件：**
- 修改：`Makefile`

- [ ] **步骤 1：在 Makefile 末尾（help 目标前）添加 release 目标**

在 `help` 目标之前插入以下内容：

```makefile
# Install GoReleaser CLI
install-tools:
	go install github.com/goreleaser/goreleaser@latest

# Local full release (requires GITHUB_TOKEN)
release-local:
	goreleaser release --clean

# Local snapshot build (no publish, for testing)
release-snapshot:
	goreleaser release --snapshot --clean
```

- [ ] **步骤 2：更新 help 目标，添加新目标说明**

修改 `help` 目标，在 `@echo "  help        - Show this help"` 前添加：

```makefile
	@echo "  install-tools - Install GoReleaser CLI"
	@echo "  release-local - Run goreleaser release (full, requires GITHUB_TOKEN)"
	@echo "  release-snapshot - Run goreleaser release --snapshot (local test)"
```

- [ ] **步骤 3：Commit**

```bash
git add Makefile
git commit -m "feat: Makefile 补充 release-local / release-snapshot / install-tools 目标"
```

---

### 任务 4：改造 `.github/workflows/release.yml`

**文件：**
- 修改：`.github/workflows/release.yml`

- [ ] **步骤 1：完整替换 release.yml**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build-linux-windows:
    name: Build (Linux + Windows)
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'
          cache: true

      - name: Install cross-compilation toolchains
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y -qq gcc-aarch64-linux-gnu gcc-mingw-w64

      - name: Install GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          install-only: true

      - name: Run GoReleaser (skip publish)
        run: goreleaser release --clean --skip=publish
        env:
          GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: dist-linux-windows
          path: dist/*
          retention-days: 1

  build-darwin:
    name: Build (Darwin)
    runs-on: macos-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'
          cache: true

      - name: Install GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          install-only: true

      - name: Run GoReleaser (Darwin, skip publish)
        run: goreleaser release --clean --skip=publish -f .goreleaser-darwin.yaml
        env:
          GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: dist-darwin
          path: dist/*
          retention-days: 1

  create-release:
    name: Create GitHub Release
    needs: [build-linux-windows, build-darwin]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4

      - name: Download Linux/Windows artifacts
        uses: actions/download-artifact@v4
        with:
          name: dist-linux-windows
          path: dist-linux-windows

      - name: Download Darwin artifacts
        uses: actions/download-artifact@v4
        with:
          name: dist-darwin
          path: dist-darwin

      - name: Merge checksums
        run: |
          echo "# migra ${{ github.ref_name }} checksums" > migra_checksums.txt
          echo "" >> migra_checksums.txt
          cat dist-linux-windows/checksums-linux-windows.txt >> migra_checksums.txt
          cat dist-darwin/checksums-darwin.txt >> migra_checksums.txt

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            dist-linux-windows/*.tar.gz
            dist-linux-windows/*.zip
            dist-linux-windows/*.sbom.spdx.json
            dist-darwin/*.tar.gz
            migra_checksums.txt
          generate_release_notes: true
        env:
          GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}
```

- [ ] **步骤 2：Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat: 改造 release.yml 为三 job 多平台构建流程"
```

---

### 任务 5：修改 `scripts/release.sh`

**文件：**
- 修改：`scripts/release.sh`

- [ ] **步骤 1：替换 release.sh 内容**

```bash
#!/bin/bash
# 发布脚本 - 创建新版本标签并推送，由 CI 自动构建发布

set -e

if [ -z "$1" ]; then
    echo "用法: $0 <版本号>"
    echo "示例: $0 v0.2.0"
    exit 1
fi

VERSION=$1

# 验证版本号格式
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ 版本号格式错误，应使用 vX.Y.Z 格式"
    exit 1
fi

echo "🚀 准备发布 $VERSION..."

# 检查工作区是否干净
if [ -n "$(git status --porcelain)" ]; then
    echo "❌ 工作区不干净，请先提交或暂存变更"
    git status --short
    exit 1
fi

# 运行测试
echo "🧪 运行测试..."
go test ./...

# 更新 CHANGELOG.md (手动步骤提示)
echo ""
echo "⚠️  请手动更新 CHANGELOG.md，添加 $VERSION 的变更记录"
echo "按 Enter 继续，或 Ctrl+C 取消..."
read

# 提交 CHANGELOG 更新（如果有）
if [ -n "$(git status --porcelain CHANGELOG.md)" ]; then
    git add CHANGELOG.md
    git commit -m "docs: 更新 CHANGELOG for $VERSION"
fi

# 创建标签
echo "🏷️  创建标签 $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

# 推送
echo "📤 推送到远程..."
git push origin "$(git branch --show-current)"
git push origin "$VERSION"

echo ""
echo "✅ 发布 $VERSION 完成！"
echo "   GitHub Actions 将自动构建多平台二进制并创建 Release"
echo "   查看: https://github.com/fred29910/migra-go/actions"
```

- [ ] **步骤 2：Commit**

```bash
git add scripts/release.sh
git commit -m "chore: 更新 release.sh，移除手动构建步骤"
```

---

### 任务 6：本地验证 — `make release-snapshot`

**文件：**
- 无（验证步骤）

- [ ] **步骤 1：安装 GoReleaser**

```bash
make install-tools
```

- [ ] **步骤 2：运行 snapshot 构建（仅验证 linux/amd64，因为本地是 Linux）**

```bash
make release-snapshot
```

- [ ] **步骤 3：验证 dist 目录产物**

```bash
ls -la dist/
# 预期看到：
#   migra-linux-amd64.tar.gz
#   checksums-linux-windows.txt
#   migra.sbom.spdx.json
```

- [ ] **步骤 4：验证打包内容**

```bash
tar tzf dist/migra-linux-amd64.tar.gz
# 预期看到：migra, LICENSE, README.md
```

- [ ] **步骤 5：验证校验和**

```bash
cd dist && sha256sum -c checksums-linux-windows.txt
# 预期：OK
```

- [ ] **步骤 6：验证 SBOM**

```bash
cat dist/migra.sbom.spdx.json | python3 -m json.tool > /dev/null
# 预期：无错误，有效 JSON
```

---

## 自检

**规格覆盖度：**
- ✅ 5 个平台构建 → 任务 1 + 任务 2（.goreleaser.yaml + .goreleaser-darwin.yaml）
- ✅ Windows=zip, 其他=tar.gz → 任务 1（archives.format_overrides）
- ✅ SHA256 校验和 → 任务 1 + 任务 2（checksum），任务 4（合并）
- ✅ SPDX SBOM → 任务 1（sboms）
- ✅ CGO 交叉编译 → 任务 1（overrides 中的 CC）
- ✅ Makefile release 目标 → 任务 3
- ✅ CI 三 job 流程 → 任务 4
- ✅ release.sh 更新 → 任务 5
- ✅ 本地 snapshot 测试 → 任务 6

**占位符扫描：** 无 "待定"、"TODO"、"适当处理" 等占位符。

**类型一致性：** 所有文件名、路径、变量名在任务间一致。
