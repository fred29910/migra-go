# Testdata 目录重构设计文档

**日期：** 2026-06-17  
**作者：** Sisyphus  
**状态：** 待审批

## 1. 背景与目标

### 1.1 背景

当前项目的 `testdata/` 目录结构较为扁平，测试数据按功能模块分散在不同位置，缺乏统一的组织规范。随着项目功能的不断增加，测试数据的管理和维护变得越来越困难。

### 1.2 目标

重构 `testdata/` 目录结构，实现以下目标：
1. **按复杂度分组**：将测试数据按复杂度（简单、中等、复杂）进行分组
2. **统一命名规范**：采用数字前缀 + 功能描述的命名规范
3. **改善可维护性**：使测试数据更清晰、更易于维护和扩展
4. **保持向后兼容**：确保现有测试用例能够正常运行

## 2. 设计方案

### 2.1 目录结构设计

**新的目录结构：**

```
testdata/
├── README.md                          # 更新文档
├── example_source.sql                 # 保持不变（简单场景）
├── example_target.sql                 # 保持不变（简单场景）
├── alter_operations.sql               # 保持不变（简单场景）
├── complex_ddl.sql                    # 保持不变（简单场景）
├── drop_scenarios.sql                 # 保持不变（简单场景）
├── edge_cases.sql                     # 保持不变（简单场景）
│
└── complex/                           # 新增：复杂场景目录
    ├── README.md                      # 复杂场景说明文档
    │
    ├── evolution/                     # 版本演进序列
    │   ├── v1/                        # 基线版本
    │   │   ├── 01_users.sql          #   用户表
    │   │   ├── 02_posts.sql          #   文章表
    │   │   ├── 03_indexes.sql        #   索引
    │   │   └── 04_enums.sql          #   枚举类型
    │   ├── v2/                        # v1 + 新增
    │   │   └── schema.sql            #   合并快照
    │   ├── v3/                        # v2 - 删除 + 修改
    │   │   ├── schema.sql            #   合并快照
    │   │   └── README.md             #   变更说明
    │   └── v4/                        # v3 + IDENTITY + COLLATE + CASCADE
    │       ├── schema.sql            #   合并快照
    │       └── README.md             #   变更说明
    │
    ├── multi-schema/                  # 多 schema 场景
    │   ├── v1/                        # 版本 v1
    │   │   ├── 01_public.sql         #   public.users
    │   │   └── 02_auth.sql           #   auth.roles + auth.permissions
    │   └── v2/                        # 版本 v2
    │       ├── 01_public.sql         #   public.users（增 email）+ public.profiles
    │       ├── 02_auth.sql           #   auth.roles（增 description）
    │       └── snapshot.sql          #   合并快照
    │
    ├── rename/                        # 重命名场景
    │   ├── basic/                     # 基础重命名
    │   │   ├── v1/
    │   │   │   └── schema.sql        #   users(username)
    │   │   └── v2/
    │   │       └── schema.sql        #   users(login_name)
    │   └── complex/                   # 复杂重命名（组合变更）
    │       ├── v1/
    │       │   └── schema.sql        #   users(username, age) + 索引
    │       └── v2/
    │           └── schema.sql        #   users(login_name, phone) + 新索引
    │
    ├── identity/                      # IDENTITY 列场景
    │   ├── v1/
    │   │   └── schema.sql            #   SERIAL 基线
    │   └── v2/
    │       └── schema.sql            #   GENERATED ALWAYS AS IDENTITY
    │
    ├── collate/                       # COLLATE 场景
    │   ├── v1/
    │   │   └── schema.sql            #   无 COLLATE
    │   └── v2/
    │       └── schema.sql            #   含 COLLATE "en_US"
    │
    ├── objects/                       # 非表对象场景
    │   ├── v1/
    │   │   └── schema.sql            #   仅表
    │   └── v2/
    │       └── schema.sql            #   + EXTENSION + SEQUENCE + VIEW
    │
    └── edge-cases/                    # 边界条件场景
        ├── nested/                    # 嵌套目录
        │   ├── 01_tables/
        │   │   └── users.sql
        │   └── 02_tables/
        │       └── posts.sql
        ├── nested-target/             # 嵌套目录目标
        │   ├── 01_tables/
        │   │   └── users.sql
        │   └── 02_tables/
        │       └── posts.sql
        └── directory/                 # DirectoryLoader 边界情况
            ├── tables/
            │   └── users.sql
            ├── .hidden.sql
            ├── .hidden_dir/
            │   └── secret.sql
            ├── empty/
            ├── empty.sql
            ├── encoding_utf8.sql
            ├── large/
            │   └── generated_schema.sql
            ├── no_extension_file
            └── readme.txt
```

### 2.2 命名规范设计

#### 文件命名规范

**规则：数字前缀 + 功能描述 + .sql 后缀**

```
格式：{序号}_{功能描述}.sql
示例：01_users.sql, 02_posts.sql, 03_indexes.sql
```

**具体规范：**

| 类型 | 命名格式 | 示例 |
|------|----------|------|
| 表定义 | `{序号}_{表名}.sql` | `01_users.sql`, `02_posts.sql` |
| 索引定义 | `{序号}_indexes.sql` | `03_indexes.sql` |
| 枚举类型 | `{序号}_enums.sql` | `04_enums.sql` |
| 合并快照 | `schema.sql` | `schema.sql` |
| 版本说明 | `README.md` | `README.md` |

#### 目录命名规范

**规则：小写字母 + 连字符**

```
格式：{功能描述}
示例：evolution, multi-schema, edge-cases
```

**具体规范：**

| 类型 | 命名格式 | 示例 |
|------|----------|------|
| 版本演进 | `evolution` | `evolution/` |
| 多 schema | `multi-schema` | `multi-schema/` |
| 重命名 | `rename` | `rename/` |
| IDENTITY | `identity` | `identity/` |
| COLLATE | `collate` | `collate/` |
| 非表对象 | `objects` | `objects/` |
| 边界条件 | `edge-cases` | `edge-cases/` |

#### 版本目录命名

**规则：`v` + 版本号**

```
格式：v{版本号}
示例：v1, v2, v3, v4
```

### 2.3 迁移策略

#### 迁移范围

**迁移的测试数据：**

| 原路径 | 新路径 | 说明 |
|--------|--------|------|
| `testdata/diff/v1/` | `testdata/complex/evolution/v1/` | 版本演进基线 |
| `testdata/diff/v2/` | `testdata/complex/evolution/v2/` | 版本演进 v2 |
| `testdata/diff/v3/` | `testdata/complex/evolution/v3/` | 版本演进 v3 |
| `testdata/diff/v4/` | `testdata/complex/evolution/v4/` | 版本演进 v4 |
| `testdata/diff/snapshot.sql` | `testdata/complex/evolution/snapshot.sql` | v2 快照 |
| `testdata/diff/multi_schema/` | `testdata/complex/multi-schema/` | 多 schema 场景 |
| `testdata/diff/rename_example/` | `testdata/complex/rename/basic/` | 基础重命名 |
| `testdata/diff/rename_complex/` | `testdata/complex/rename/complex/` | 复杂重命名 |
| `testdata/diff/identity_example/` | `testdata/complex/identity/` | IDENTITY 场景 |
| `testdata/diff/collate_example/` | `testdata/complex/collate/` | COLLATE 场景 |
| `testdata/diff/objects_example/` | `testdata/complex/objects/` | 非表对象场景 |
| `testdata/diff/nested/` | `testdata/complex/edge-cases/nested/` | 嵌套目录 |
| `testdata/diff/nested_target/` | `testdata/complex/edge-cases/nested-target/` | 嵌套目录目标 |
| `testdata/diff/edge/` | `testdata/complex/edge-cases/directory/` | DirectoryLoader 边界 |

**保留的测试数据：**

| 路径 | 说明 |
|------|------|
| `testdata/example_source.sql` | 简单场景：基础 diff 测试源 |
| `testdata/example_target.sql` | 简单场景：基础 diff 测试目标 |
| `testdata/alter_operations.sql` | 简单场景：ALTER TABLE 操作 |
| `testdata/complex_ddl.sql` | 简单场景：复合 DDL |
| `testdata/drop_scenarios.sql` | 简单场景：DROP 语义 |
| `testdata/edge_cases.sql` | 简单场景：边界 SQL 模式 |

#### 迁移步骤

**阶段 1：创建目录结构**
```bash
# 创建 complex/ 目录结构
mkdir -p testdata/complex/{evolution,multi-schema,rename/{basic,complex},identity,collate,objects,edge-cases/{nested,nested-target,directory}}
```

**阶段 2：复制文件**
```bash
# 复制版本演进序列
cp -r testdata/diff/v1/ testdata/complex/evolution/v1/
cp -r testdata/diff/v2/ testdata/complex/evolution/v2/
cp -r testdata/diff/v3/ testdata/complex/evolution/v3/
cp -r testdata/diff/v4/ testdata/complex/evolution/v4/
cp testdata/diff/snapshot.sql testdata/complex/evolution/

# 复制多 schema 场景
cp -r testdata/diff/multi_schema/ testdata/complex/multi-schema/

# 复制重命名场景
cp -r testdata/diff/rename_example/ testdata/complex/rename/basic/
cp -r testdata/diff/rename_complex/ testdata/complex/rename/complex/

# 复制其他复杂场景
cp -r testdata/diff/identity_example/ testdata/complex/identity/
cp -r testdata/diff/collate_example/ testdata/complex/collate/
cp -r testdata/diff/objects_example/ testdata/complex/objects/

# 复制边界场景
cp -r testdata/diff/nested/ testdata/complex/edge-cases/nested/
cp -r testdata/diff/nested_target/ testdata/complex/edge-cases/nested-target/
cp -r testdata/diff/edge/ testdata/complex/edge-cases/directory/
```

**阶段 3：验证复制**
```bash
# 验证目录结构
tree testdata/complex/

# 验证文件完整性
find testdata/complex/ -name "*.sql" | wc -l
```

**阶段 4：删除旧目录**
```bash
# 删除旧的 diff/ 目录
rm -rf testdata/diff/
```

### 2.4 测试用例更新

#### 需要更新的测试文件

**集成测试文件：** `cmd/migra/integration_test.go`

#### 路径映射表

| 原路径 | 新路径 | 测试函数 |
|--------|--------|----------|
| `../../testdata/diff/v1/` | `../../testdata/complex/evolution/v1/` | `TestV3DirectoryDiff` |
| `../../testdata/diff/v2/` | `../../testdata/complex/evolution/v2/` | `TestV3DirectoryDiff` |
| `../../testdata/diff/v2/schema.sql` | `../../testdata/complex/evolution/v2/schema.sql` | `TestV3UnsafeDropDiff_Safe`, `TestV3UnsafeDropDiff_Unsafe` |
| `../../testdata/diff/v3/schema.sql` | `../../testdata/complex/evolution/v3/schema.sql` | `TestV3UnsafeDropDiff_Safe`, `TestV3UnsafeDropDiff_Unsafe`, `TestV3ToV4Diff` |
| `../../testdata/diff/v4/schema.sql` | `../../testdata/complex/evolution/v4/schema.sql` | `TestV3ToV4Diff` |
| `../../testdata/diff/nested/` | `../../testdata/complex/edge-cases/nested/` | `TestNestedDirectoryDiff` |
| `../../testdata/diff/nested_target/` | `../../testdata/complex/edge-cases/nested-target/` | `TestNestedDirectoryDiff` |
| `../../testdata/diff/multi_schema/v1/` | `../../testdata/complex/multi-schema/v1/` | `TestMultiSchemaDirectoryDiffStatic` |
| `../../testdata/diff/multi_schema/v2/` | `../../testdata/complex/multi-schema/v2/` | `TestMultiSchemaDirectoryDiffStatic` |
| `../../testdata/diff/rename_example/v1/schema.sql` | `../../testdata/complex/rename/basic/v1/schema.sql` | `TestRenameColumnDiff` |
| `../../testdata/diff/rename_example/v2/schema.sql` | `../../testdata/complex/rename/basic/v2/schema.sql` | `TestRenameColumnDiff` |
| `../../testdata/diff/rename_complex/v1/schema.sql` | `../../testdata/complex/rename/complex/v1/schema.sql` | `TestRenameColumnComplexDiff` |
| `../../testdata/diff/rename_complex/v2/schema.sql` | `../../testdata/complex/rename/complex/v2/schema.sql` | `TestRenameColumnComplexDiff` |
| `../../testdata/diff/identity_example/v1/schema.sql` | `../../testdata/complex/identity/v1/schema.sql` | `TestIdentityColumnDiff` |
| `../../testdata/diff/identity_example/v2/schema.sql` | `../../testdata/complex/identity/v2/schema.sql` | `TestIdentityColumnDiff` |
| `../../testdata/diff/collate_example/v1/schema.sql` | `../../testdata/complex/collate/v1/schema.sql` | `TestCollateClauseDiff` |
| `../../testdata/diff/collate_example/v2/schema.sql` | `../../testdata/complex/collate/v2/schema.sql` | `TestCollateClauseDiff` |
| `../../testdata/diff/objects_example/v1/schema.sql` | `../../testdata/complex/objects/v1/schema.sql` | `TestObjectsDiff` |
| `../../testdata/diff/objects_example/v2/schema.sql` | `../../testdata/complex/objects/v2/schema.sql` | `TestObjectsDiff` |

#### 更新示例

**更新前：**
```go
{
    Name: "V3 directory diff",
    Source: app.SourceConfig{
        Type: "directory",
        Source: "../../testdata/diff/v1/",
    },
    Target: app.TargetConfig{
        Type: "directory",
        Source: "../../testdata/diff/v2/",
    },
},
```

**更新后：**
```go
{
    Name: "V3 directory diff",
    Source: app.SourceConfig{
        Type: "directory",
        Source: "../../testdata/complex/evolution/v1/",
    },
    Target: app.TargetConfig{
        Type: "directory",
        Source: "../../testdata/complex/evolution/v2/",
    },
},
```

#### 新增测试用例

**为 complex/ 目录添加新的测试用例：**

```go
// TestComplexEvolutionDiff 测试复杂版本演进序列
func TestComplexEvolutionDiff(t *testing.T) {
    testCases := []struct {
        Name   string
        Source app.SourceConfig
        Target app.TargetConfig
    }{
        {
            Name: "v1 to v2 evolution",
            Source: app.SourceConfig{
                Type: "directory",
                Source: "../../testdata/complex/evolution/v1/",
            },
            Target: app.TargetConfig{
                Type: "directory",
                Source: "../../testdata/complex/evolution/v2/",
            },
        },
        {
            Name: "v2 to v3 evolution",
            Source: app.SourceConfig{
                Type: "file",
                Source: "../../testdata/complex/evolution/v2/schema.sql",
            },
            Target: app.TargetConfig{
                Type: "file",
                Source: "../../testdata/complex/evolution/v3/schema.sql",
            },
        },
        {
            Name: "v3 to v4 evolution",
            Source: app.SourceConfig{
                Type: "file",
                Source: "../../testdata/complex/evolution/v3/schema.sql",
            },
            Target: app.TargetConfig{
                Type: "file",
                Source: "../../testdata/complex/evolution/v4/schema.sql",
            },
        },
    }
    // ... 测试逻辑
}
```

### 2.5 文档更新

#### 需要更新的文档

**主文档：** `testdata/README.md`

#### 文档更新内容

1. **更新目录结构说明**：反映新的目录结构
2. **更新测试数据映射表**：更新测试数据与测试用例的映射关系
3. **新增复杂场景说明**：添加 complex/ 目录的详细说明
4. **更新版本演进图谱**：更新版本演进的路径引用
5. **更新验证命令速查表**：更新验证命令中的路径引用

#### 新增 complex/README.md

**新增文件：** `testdata/complex/README.md`

包含以下内容：
- 复杂场景测试数据的目录结构说明
- 各功能模块的详细说明
- 如何运行测试的指南

## 3. 验证计划

### 3.1 验证步骤

1. **目录结构验证**
   ```bash
   # 验证目录结构
   tree testdata/complex/
   
   # 验证文件完整性
   find testdata/complex/ -name "*.sql" | wc -l
   ```

2. **测试用例验证**
   ```bash
   # 运行所有测试
   make test
   
   # 运行复杂场景测试
   go test ./cmd/migra/... -v -run TestComplex
   
   # 运行版本演进测试
   go test ./cmd/migra/... -v -run TestV3DirectoryDiff
   ```

3. **文档验证**
   ```bash
   # 验证文档格式
   grep -r "testdata/complex/" docs/
   grep -r "testdata/diff/" docs/  # 应该没有结果
   ```

### 3.2 验证清单

- [ ] 目录结构正确
- [ ] 所有测试数据已迁移
- [ ] 所有测试用例路径已更新
- [ ] 所有测试用例通过
- [ ] 文档已更新
- [ ] 旧目录已删除

## 4. 风险与缓解措施

### 4.1 风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 测试用例路径更新遗漏 | 测试失败 | 逐个检查每个测试用例的路径引用 |
| 文件复制不完整 | 测试数据缺失 | 验证文件数量和内容 |
| 文档更新不完整 | 文档与实际不符 | 逐个检查文档中的路径引用 |

### 4.2 回滚计划

如果重构过程中出现问题，可以使用以下回滚步骤：

```bash
# 恢复旧的 testdata 目录
git checkout HEAD -- testdata/

# 删除新的 complex/ 目录
rm -rf testdata/complex/
```

## 5. 时间估算

| 任务 | 时间估算 |
|------|----------|
| 创建目录结构 | 5 分钟 |
| 复制文件 | 10 分钟 |
| 验证复制 | 5 分钟 |
| 更新测试用例路径 | 30 分钟 |
| 新增测试用例 | 20 分钟 |
| 更新文档 | 20 分钟 |
| 验证测试 | 15 分钟 |
| **总计** | **约 105 分钟** |

## 6. 总结

本设计文档详细描述了 `testdata/` 目录的重构方案，包括：

1. **目录结构设计**：按复杂度分组，创建 `complex/` 目录
2. **命名规范设计**：统一使用数字前缀 + 功能描述的命名规范
3. **迁移策略**：分阶段迁移测试数据
4. **测试用例更新**：更新所有测试用例的路径引用
5. **文档更新**：更新所有相关文档

通过本次重构，项目的测试数据将更加清晰、易于维护和扩展，为后续的功能开发提供更好的支持。
