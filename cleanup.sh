#!/bin/bash

# SWE-Agent 项目清理脚本
# 清理冗余文件和编译产物

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           SWE-Agent 项目清理脚本                             ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 统计
TOTAL_SIZE=0
TOTAL_FILES=0

# 函数：安全删除文件
safe_remove() {
    local file=$1
    if [ -f "$file" ]; then
        size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)
        TOTAL_SIZE=$((TOTAL_SIZE + size))
        TOTAL_FILES=$((TOTAL_FILES + 1))
        rm -f "$file"
        echo -e "${GREEN}✓${NC} 删除: $file ($(numfmt --to=iec-i --suffix=B $size 2>/dev/null || echo $size bytes))"
    else
        echo -e "${YELLOW}⊘${NC} 跳过: $file (不存在)"
    fi
}

echo "🗑️  清理旧的编译产物..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
safe_remove "codex-webhook"

echo ""
echo "🗑️  清理测试覆盖率文件（可重新生成）..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
safe_remove "coverage.out"
safe_remove "coverage.html"

echo ""
echo "🗑️  清理过时的开发文档..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
safe_remove "SIMPLIFICATION_PLAN.md"
safe_remove "ARCHITECTURE_COMPARISON.md"
safe_remove "TEST_RESULTS.md"
safe_remove "test_scenarios.md"
safe_remove "AGENTS.md"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ 清理完成！${NC}"
echo ""
echo "📊 清理统计:"
echo "  • 删除文件数: $TOTAL_FILES"
if command -v numfmt &> /dev/null; then
    echo "  • 释放空间: $(numfmt --to=iec-i --suffix=B $TOTAL_SIZE)"
else
    echo "  • 释放空间: $TOTAL_SIZE bytes"
fi
echo ""
echo "💡 提示:"
echo "  • 覆盖率报告可通过 'go test -coverprofile=coverage.out ./...' 重新生成"
echo "  • 二进制文件可通过 'go build -o swe-agent cmd/main.go' 重新构建"
echo ""

# 显示当前未跟踪文件
echo "📁 当前未跟踪文件 (git):"
git status --porcelain 2>/dev/null | grep "^??" | head -10 || echo "  无"
