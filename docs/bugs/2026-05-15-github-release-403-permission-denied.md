# GitHub Release 创建失败 — 403 Permission Denied

## 基本信息

| 字段 | 值 |
|------|-----|
| **日期** | 2026-05-15 |
| **标签** | `v0.1.1` |
| **工作流** | `.github/workflows/release.yml` |
| **Action** | `softprops/action-gh-release@v2` |
| **错误码** | HTTP 403 |
| **严重程度** | 🔴 高 — 阻断发布流程 |

---

## 错误现象

推送标签 `v0.1.1` 后，GitHub Actions 工作流成功执行了编译、SHA256 校验和生成、SBOM 生成等步骤，但在最后一步 **Create Release** 时失败：

```
👩‍🏭 Creating new GitHub release for tag v0.1.1...
⚠️ GitHub release failed with status: 403
{"message":"Resource not accessible by integration",
 "documentation_url":"https://docs.github.com/rest/releases/releases#generate-release-notes-content-for-a-release",
 "status":"403"}
Skip retry — your GitHub token/PAT does not have the required permission to create a release
⚠️ Unexpected error fetching GitHub release for tag refs/tags/v0.1.1:
    HttpError: Resource not accessible by integration
Error: Resource not accessible by integration
```

---

## 根因分析

### 直接原因

工作流使用的 `GITHUB_TOKEN`（即 GitHub Actions 自动生成的临时 token）**缺少创建 Release 所需的权限**。

在 [release.yml:52](.github/workflows/release.yml:52) 中：

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

`${{ secrets.GITHUB_TOKEN }}` 是 GitHub Actions 为每个 job 自动生成的临时 token，其权限受限于仓库的 **Workflow permissions** 设置。

### 深层原因

GitHub 仓库的 **Settings → Actions → General → Workflow permissions** 中，默认权限可能设置为 **Read-only**（只读），或者即使选择了 **Read and write**，也可能因为以下原因导致权限不足：

1. **仓库默认权限为只读**：自 2023 年起，GitHub 将新仓库的默认 workflow 权限从 "Read and write" 改为 "Read-only"。如果仓库是在此之后创建的，`GITHUB_TOKEN` 默认只有读取权限，无法创建 Release。

2. **组织级权限限制**：如果仓库属于某个组织（Organization），组织管理员可能在组织级别设置了更严格的 workflow 权限策略，覆盖了仓库级别的设置。

3. **`generate_release_notes: true` 需要额外权限**：`softprops/action-gh-release` 的 `generate_release_notes` 功能需要调用 GitHub REST API 的 `POST /repos/{owner}/{repo}/releases/generate-notes` 端点，该端点要求 token 具有 **contents: write** 权限。即使基础 Release 创建权限已满足，缺少 contents 写入权限也会导致 403。

### 错误链

```
push tag v0.1.1
  → GitHub Actions 触发 release.yml
    → checkout ✅
    → setup-go ✅
    → build binary ✅
    → generate SHA256 ✅
    → generate SBOM ✅
    → Create Release (softprops/action-gh-release@v2)
      → 调用 POST /repos/fred29910/migra-go/releases
      → 调用 POST /repos/fred29910/migra-go/releases/generate-notes
      → ❌ 403 Resource not accessible by integration
        → GITHUB_TOKEN 缺少 contents:write 权限
```

---

## 解决方案

### 方案一：修改仓库 Workflow 权限（推荐）

在 GitHub 仓库设置中授予 `GITHUB_TOKEN` 写入权限：

1. 进入仓库 **Settings**
2. 左侧导航选择 **Actions → General**
3. 找到 **Workflow permissions** 部分
4. 选择 **Read and write permissions**（读取和写入权限）
5. 点击 **Save**

修改后，`GITHUB_TOKEN` 将自动获得 `contents: write` 权限，足以创建 Release 和生成 release notes。

**优点**：无需额外配置，无需管理 PAT，安全性高（token 是临时的、自动过期的）。

### 方案二：使用 Personal Access Token (PAT)

如果因组织策略无法修改 workflow 权限，可以使用 PAT：

1. 创建 Fine-grained PAT：
   - 访问 **Settings → Developer settings → Personal access tokens → Fine-grained tokens**
   - 设置 **Repository access** 为当前仓库
   - 设置 **Permissions**：
     - **Contents**: Read and write
     - **Metadata**: Read-only（自动）

2. 将 PAT 添加到 **Repository secrets**：
   - 仓库 **Settings → Secrets and variables → Actions → Repository secrets**
   - 点击 **New repository secret**
   - 名称为 `RELEASE_TOKEN`，值为 PAT
   - 点击 **Add secret**

   > **为什么用 Repository secrets 而不是 Environment secrets？**
   > - **Repository secrets** 对仓库内所有 workflow 和所有环境可用，引用方式简单：`${{ secrets.RELEASE_TOKEN }}`
   > - **Environment secrets** 需要先配置 Environment（如 `production`），然后在 job/step 上声明 `environment: production` 才能使用。适用于多环境隔离场景，但当前 `release.yml` 没有使用 environment，引入它会增加不必要的复杂度
   > - 如果未来需要多环境管控，可以迁移到 Environment secrets，届时只需在 job 上添加 `environment: <name>` 即可

3. 修改 [release.yml:52](.github/workflows/release.yml:52)：

```yaml
  env:
    GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}
```

**缺点**：需要管理 PAT 的生命周期，存在过期风险。

### 方案三：在 workflow 文件中显式声明 permissions

在 `release.yml` 的 job 级别显式声明所需权限（GitHub 会以此覆盖默认权限，但前提是仓库/组织设置允许）：

```yaml
jobs:
  build:
    name: Build and Release
    runs-on: ubuntu-latest
    permissions:
      contents: write    # 创建 Release + 上传附件 + generate-notes
    steps:
      # ... 其余步骤不变
```

**注意**：此方案仅在仓库 Workflow permissions 设置为 "Read and write" 时有效。如果仓库权限为 "Read-only"，显式声明 `write` 不会提升权限上限。

---

## 推荐修复步骤

1. **立即**：在仓库 Settings 中将 Workflow permissions 改为 **Read and write**
2. **同时**：在 `release.yml` 中添加显式 `permissions` 声明（最佳实践，使权限自文档化）
3. **重新触发**：重新推送标签或手动触发工作流

修改后的 `release.yml` 关键部分：

```yaml
jobs:
  build:
    name: Build and Release
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      # ... 其余步骤不变
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

---

## 参考链接

- [GitHub Docs: Automatic token authentication](https://docs.github.com/en/actions/security-guides/automatic-token-authentication)
- [GitHub Docs: Workflow permissions](https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs)
- [GitHub Docs: Repository permissions for GitHub Actions](https://docs.github.com/en/repositories/managing-a-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#setting-the-permissions-of-the-github_token-for-your-repository)
- [softprops/action-gh-release: Permissions](https://github.com/softprops/action-gh-release#permissions)
- [GitHub REST API: Generate release notes](https://docs.github.com/en/rest/releases/releases#generate-release-notes-content-for-a-release)
