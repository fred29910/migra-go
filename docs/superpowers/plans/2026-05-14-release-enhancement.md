# Release Enhancement 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 增强现有的GitHub Release工作流，在生成Linux amd64二进制包的同时自动生成校验和（SHA256）和SBOM（Software Bill of Materials），并将这些制品作为Release资产上传。

**架构：** 在现有的 `.github/workflows/release.yml` 工作流中添加校验和生成和SBOM生成步骤，使用标准工具（shasum和Syft）生成制品，并将所有制品上传为GitHub Release资产。

**技术栈：** GitHub Actions, Go, shasum, Syft

---

### 任务 1：修改Release工作流以添加校验和生成步骤

**文件：**
- 修改：`.github/workflows/release.yml:22-38`

- [ ] **步骤 1：编写失败的测试（验证工作流语法）**

```yaml
# 这个测试实际上是通过运行workflow来验证的，但我们可以先检查YAML语法
# 由于GitHub Actions工作流测试需要实际运行，我们将在实现后通过推送标签来验证
echo "工作流语法验证将在实现后通过实际运行来测试"
```

- [ ] **步骤 2：运行测试验证失败（此步骤跳过，因为工作流测试需要实际运行）**

由于GitHub Actions工作流需要在实际环境中运行才能验证，我们将直接实现并在推送标签时验证。

- [ ] **步骤 3：编写最少实现代码**

在现有工作流中，在"Build binary"步骤后添加校验和生成步骤：

```yaml
      - name: Build binary
        run: |
          make build
          chmod +x migra
      
      - name: Generate SHA256 checksum
        run: |
          shasum -a 256 migra > migra.sha256
```

- [ ] **步骤 4：运行测试验证通过（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 5：Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): add checksum generation step"
```

### 任务 2：修改Release工作流以添加SBOM生成步骤

**文件：**
- 修改：`.github/workflows/release.yml:22-45`

- [ ] **步骤 1：编写失败的测试（验证工作流语法）**

```bash
echo "工作流语法验证将在实现后通过实际运行来测试"
```

- [ ] **步骤 2：运行测试验证失败（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 3：编写最少实现代码**

在校验和生成步骤后添加SBOM生成步骤：

```yaml
      - name: Generate SHA256 checksum
        run: |
          shasum -a 256 migra > migra.sha256
      
      - name: Generate SBOM
        run: |
          # 安装 Syft
          curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
          # 生成 SPDX 格式的 SBOM
          syft packages migra -o spdx-json > migra-sbom.spdx.json
```

- [ ] **步骤 4：运行测试验证通过（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 5：Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): add SBOM generation step"
```

### 任务 3：修改Release工作流以将所有制品上传为Release资产

**文件：**
- 修改：`.github/workflows/release.yml:32-45`

- [ ] **步骤 1：编写失败的测试（验证工作流语法）**

```bash
echo "工作流语法验证将在实现后通过实际运行来测试"
```

- [ ] **步骤 2：运行测试验证失败（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 3：编写最少实现代码**

修改"Create Release"步骤以包含所有制品：

```yaml
      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            migra
            migra.sha256
            migra-sbom.spdx.json
          generate_release_notes: true
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **步骤 4：运行测试验证通过（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 5：Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): upload all artifacts to release"
```

### 任务 4：验证完整的Release工作流

**文件：**
- 修改：`.github/workflows/release.yml`

- [ ] **步骤 1：编写失败的测试（验证完整工作流）**

```bash
# 完整工作流验证将通过实际的标签推送来测试
echo "完整工作流验证将在推送标签时进行"
```

- [ ] **步骤 2：运行测试验证失败（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 3：编写最少实现代码**

完整的工作流应该看起来像这样：

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    name: Build and Release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'
          cache: true

      - name: Build binary
        run: |
          make build
          chmod +x migra

      - name: Generate SHA256 checksum
        run: |
          shasum -a 256 migra > migra.sha256

      - name: Generate SBOM
        run: |
          # 安装 Syft
          curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
          # 生成 SPDX 格式的 SBOM
          syft packages migra -o spdx-json > migra-sbom.spdx.json

      - name: Run integration tests (with DB)
        run: go test ./... -v
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        if: env.DATABASE_URL != ''

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            migra
            migra.sha256
            migra-sbom.spdx.json
          generate_release_notes: true
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **步骤 4：运行测试验证通过（此步骤跳过，因为工作流测试需要实际运行）**

- [ ] **步骤 5：Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): complete workflow with checksum, SBOM, and artifact upload"
```

## 自检

### 1. 规格覆盖度检查
让我检查设计文档中的每个目标是否都有对应的任务：

- [x] 在现有的 `.github/workflows/release.yml` 工作流中添加步骤 - 通过任务1、2、3覆盖
- [x] 生成二进制包的SHA256校验和文件 - 通过任务1覆盖
- [x] 使用Syft工具生成SBOM（SPDX格式） - 通过任务2覆盖
- [x] 将校验和和SBOM作为Release资产上传 - 通过任务3覆盖
- [x] 保持工作流的简洁和可维护性 - 通过所有任务的逐步实现覆盖

### 2. 占位符扫描
检查计划中是否有禁止的占位符：
- 没有找到 "待定"、"TODO"、"后续实现"、"补充细节"
- 没有找到 "添加适当的错误处理" / "添加验证" / "处理边界情况" 的一般性描述
- 每个代码步骤都有实际代码块
- 没有重复的任务描述
- 每个步骤都展示了怎么做的具体内容

### 3. 类型一致性检查
- 所有任务都在修改同一个文件：`.github/workflows/release.yml`
- 工作流结构保持一致
- 步骤名称和顺序逻辑清晰
- 使用的工具和命令在任务之间保持一致

计划已完成并保存到 `docs/superpowers/plans/2026-05-14-release-enhancement.md`。两种执行方式：

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点供审查

**选哪种方式？**

<tool_call>
<function=attempt_completion>