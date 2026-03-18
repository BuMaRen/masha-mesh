#!/bin/bash

# 文档整理脚本 - 将所有 markdown 文档移动到 docs 目录中的对应子目录

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}文档整理脚本 - 将文档移动到分类目录${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}\n"

# 检查 docs 目录是否存在
if [ ! -d "$PROJECT_ROOT/docs" ]; then
    echo -e "${RED}❌ docs 目录不存在，请先运行分类创建脚本${NC}"
    exit 1
fi

# 定义要移动的文件映射
declare -A FILE_MAP=(
    # 快速开始
    ["$PROJECT_ROOT/README_FIRST.txt"]="$PROJECT_ROOT/docs/getting-started/"
    ["$PROJECT_ROOT/QUICKSTART.md"]="$PROJECT_ROOT/docs/getting-started/"
    ["$PROJECT_ROOT/START_HERE.md"]="$PROJECT_ROOT/docs/getting-started/"
    ["$PROJECT_ROOT/README.md"]="$PROJECT_ROOT/docs/getting-started/"

    # 部署
    ["$PROJECT_ROOT/README_DEPLOYMENT.md"]="$PROJECT_ROOT/docs/deployment/"
    ["$PROJECT_ROOT/DEPLOYMENT_SUMMARY.md"]="$PROJECT_ROOT/docs/deployment/"
    ["$PROJECT_ROOT/DEPLOYMENT_SUCCESS.md"]="$PROJECT_ROOT/docs/deployment/"
    ["$PROJECT_ROOT/deployment.log"]="$PROJECT_ROOT/docs/deployment/"

    # 调试
    ["$PROJECT_ROOT/DEBUG_REPORT.md"]="$PROJECT_ROOT/docs/debugging/"

    # 项目
    ["$PROJECT_ROOT/项目清单.md"]="$PROJECT_ROOT/docs/project/"
    ["$PROJECT_ROOT/SUMMARY.md"]="$PROJECT_ROOT/docs/project/"
)

# 显示待移动的文件
echo -e "${YELLOW}📋 待整理的文件：${NC}"
count=0
for file in "${!FILE_MAP[@]}"; do
    if [ -f "$file" ]; then
        ((count++))
        echo -e "  $count. $(basename "$file")"
    fi
done
echo ""

# 询问用户确认
read -p "是否继续？(y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}已取消${NC}"
    exit 0
fi

# 执行移动
echo -e "\n${BLUE}开始整理文件...${NC}\n"

moved_count=0
skipped_count=0

for file in "${!FILE_MAP[@]}"; do
    dest_dir="${FILE_MAP[$file]}"
    
    if [ ! -f "$file" ]; then
        echo -e "${YELLOW}⊘ 跳过${NC}: $(basename "$file") (文件不存在)"
        ((skipped_count++))
        continue
    fi
    
    filename=$(basename "$file")
    
    # 移动文件
    if mv "$file" "$dest_dir/$filename"; then
        echo -e "${GREEN}✓ 移动${NC}: $filename → $(basename "$dest_dir")/"
        ((moved_count++))
    else
        echo -e "${RED}❌ 失败${NC}: $filename"
    fi
done

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ 整理完成！${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

echo -e "📊 统计："
echo -e "  ${GREEN}已移动${NC}: $moved_count 个文件"
echo -e "  ${YELLOW}已跳过${NC}: $skipped_count 个文件"
echo ""

echo -e "📖 查看整理结果："
echo -e "  tree docs/ -L 2"
echo -e "  或者："
echo -e "  ls -la docs/*/"
echo ""

echo -e "🚀 后续步骤："
echo -e "  1. 查看 ${BLUE}DOCS_INDEX.md${NC} 了解完整导航"
echo -e "  2. 进入 ${BLUE}docs/getting-started/${NC} 开始阅读"
echo -e "  3. 按照文档指示进行操作"
echo ""
