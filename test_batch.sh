#!/bin/bash
# Cowork 批量业务场景测试脚本
# 用法: ./test_batch.sh
# 会自动运行预定义的测试场景并生成汇总报告

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPORT_DIR="/tmp/cowork-test-reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
SUMMARY_FILE="$REPORT_DIR/summary_${TIMESTAMP}.md"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 生成测试ID（时间戳+随机字符）
TEST_ID=$(date +%Y%m%d_%H%M%S)_$RANDOM
TEST_DIR="/tmp/cowork-test-$TEST_ID"

# 测试场景列表（使用动态测试目录）
SCENARIOS=(
    "场景1: 在 $TEST_DIR 目录下创建一个 README.md 文件，内容是 'Hello Cowork!'"
    "场景2: 查询系统状态，告诉我有哪些 worker 在线"
    "场景3: 列出 /tmp 目录下的前5个文件"
    "场景4: 读取 $TEST_DIR/README.md 文件内容"
    "场景5: 在 $TEST_DIR 目录下创建一个 test.txt 文件，内容是当前时间戳"
    "场景6: 把 $TEST_DIR 目录下所有 .txt 和 .md 文件合并成一个 all.txt"
)

mkdir -p "$REPORT_DIR"

echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║          Cowork 批量业务场景测试                          ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "测试ID: ${BLUE}$TEST_ID${NC}"
echo -e "测试目录: ${BLUE}$TEST_DIR${NC}"
echo -e "报告目录: ${BLUE}$REPORT_DIR${NC}"
echo ""

# 初始化汇总报告
cat > "$SUMMARY_FILE" << EOF
# Cowork 批量测试汇总报告

**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')
**测试ID**: $TEST_ID
**测试目录**: $TEST_DIR

## 测试场景列表

EOF

# 记录测试结果
declare -a RESULTS
declare -a TOOL_COUNTS
declare -a REPORT_FILES

# 运行每个场景
for i in "${!SCENARIOS[@]}"; do
    SCENARIO_NUM=$((i + 1))
    SCENARIO="${SCENARIOS[$i]}"

    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}场景 $SCENARIO_NUM/${#SCENARIOS[@]}${NC}: $SCENARIO"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # 运行测试脚本
    if "$SCRIPT_DIR/test_scenario.sh" "$SCENARIO" 2>&1 | tee /tmp/test_output_${SCENARIO_NUM}.log; then
        RESULTS[$i]="✅ 成功"
        echo -e "结果: ${GREEN}✅ 成功${NC}"
    else
        RESULTS[$i]="❌ 失败"
        echo -e "结果: ${RED}❌ 失败${NC}"
    fi

    # 获取最新的报告文件
    LATEST_REPORT=$(ls -t "$REPORT_DIR"/report_*.md 2>/dev/null | head -1)
    REPORT_FILES[$i]="$LATEST_REPORT"

    # 提取工具调用次数
    TOOL_COUNT=$(grep "工具调用次数:" "$LATEST_REPORT" 2>/dev/null | awk '{print $2}' || echo "N/A")
    TOOL_COUNTS[$i]="$TOOL_COUNT"

    echo ""

    # 等待一下，避免太快
    sleep 2
done

# 生成汇总报告
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}生成汇总报告...${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

cat > "$SUMMARY_FILE" << EOF
# Cowork 批量测试汇总报告

**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')
**总场景数**: ${#SCENARIOS[@]}

## 测试结果汇总

| 场景 | 任务描述 | 结果 | 工具调用 |
|------|----------|------|----------|
EOF

for i in "${!SCENARIOS[@]}"; do
    SCENARIO_NUM=$((i + 1))
    SHORT_DESC=$(echo "${SCENARIOS[$i]}" | cut -d':' -f2- | xargs | head -c 50)
    echo "| $SCENARIO_NUM | $SHORT_DESC... | ${RESULTS[$i]} | ${TOOL_COUNTS[$i]} |" >> "$SUMMARY_FILE"
done

cat >> "$SUMMARY_FILE" << EOF

## 详细报告文件

EOF

for i in "${!REPORT_FILES[@]}"; do
    SCENARIO_NUM=$((i + 1))
    echo "- 场景 $SCENARIO_NUM: \`${REPORT_FILES[$i]}\`" >> "$SUMMARY_FILE"
done

# 统计成功率
SUCCESS_COUNT=0
for result in "${RESULTS[@]}"; do
    if [[ "$result" == *"成功"* ]]; then
        ((SUCCESS_COUNT++))
    fi
done

cat >> "$SUMMARY_FILE" << EOF

## 测试统计

- **成功**: $SUCCESS_COUNT / ${#SCENARIOS[@]}
- **成功率**: $(( SUCCESS_COUNT * 100 / ${#SCENARIOS[@]} ))%

---
*报告生成时间: $(date '+%Y-%m-%d %H:%M:%S')*
EOF

# 显示汇总
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    测试汇总                               ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

for i in "${!SCENARIOS[@]}"; do
    SCENARIO_NUM=$((i + 1))
    SHORT_DESC=$(echo "${SCENARIOS[$i]}" | cut -d':' -f2- | xargs | head -c 40)
    echo -e "  场景 $SCENARIO_NUM: ${RESULTS[$i]} - ${SHORT_DESC}... (工具: ${TOOL_COUNTS[$i]})"
done

echo ""
echo -e "  成功率: ${GREEN}$SUCCESS_COUNT${NC} / ${#SCENARIOS[@]} ($(( SUCCESS_COUNT * 100 / ${#SCENARIOS[@]} ))%)"
echo -e "  汇总报告: ${BLUE}$SUMMARY_FILE${NC}"
echo ""