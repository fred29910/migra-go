#!/bin/bash
# 发布脚本 - 创建新版本标签

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
make test

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
git push origin develop
git push origin "$VERSION"

echo ""
echo "✅ 发布 $VERSION 完成！"
echo "   GitHub Actions 将自动构建并创建 Release"
echo "   查看: https://github.com/fred29910/migra-go/actions"
