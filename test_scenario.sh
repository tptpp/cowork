#!/bin/bash
# Cowork 业务场景测试脚本
# 用法: ./test_scenario.sh "用户任务描述"
# 示例: ./test_scenario.sh "在 /tmp/cowork-test/ 目录下创建一个 README.md 文件"

set -e

# 配置
COWORK_DIR="$(cd "$(dirname "$0")" && pwd)"
COORDINATOR="$COWORK_DIR/bin/coordinator"
WORKER="$COWORK_DIR/bin/worker"
API_BASE="http://localhost:8080"
REPORT_DIR="/tmp/cowork-test-reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查参数
if [ -z "$1" ]; then
    echo -e "${RED}错误: 请提供用户任务描述${NC}"
    echo "用法: $0 \"用户任务描述\""
    echo "示例: $0 \"在 /tmp/cowork-test/ 目录下创建一个 README.md 文件\""
    exit 1
fi

USER_TASK="$1"
REPORT_FILE="$REPORT_DIR/report_${TIMESTAMP}.md"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Cowork 业务场景测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 创建报告目录
mkdir -p "$REPORT_DIR"

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}清理环境...${NC}"
    pkill -f "bin/coordinator" 2>/dev/null || true
    pkill -f "bin/worker" 2>/dev/null || true
    sleep 1
}

# 设置退出时清理
trap cleanup EXIT

# Step 1: 清理旧数据
echo -e "${YELLOW}[Step 1/7] 清理旧数据...${NC}"
rm -rf ~/.cowork/coordinator/cowork.db 2>/dev/null || true
rm -rf ~/.cowork/workers/*/workspace/* 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} 旧数据已清理"

# Step 2: 启动 Coordinator
echo -e "${YELLOW}[Step 2/7] 启动 Coordinator...${NC}"
COWORK_LOG_LEVEL=info "$COORDINATOR" > /tmp/coordinator.log 2>&1 &
COORD_PID=$!

# 等待 Coordinator 启动 (最多等待 10 秒)
for i in {1..10}; do
    if curl -s "$API_BASE/health" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

# 检查是否启动成功
if ! curl -s "$API_BASE/health" > /dev/null 2>&1; then
    echo -e "  ${RED}✗ Coordinator 启动失败${NC}"
    cat /tmp/coordinator.log
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Coordinator 已启动 (PID: $COORD_PID)"

# Step 3: 启动 Worker
echo -e "${YELLOW}[Step 3/7] 启动 Worker...${NC}"
"$WORKER" --name test-worker > /tmp/worker.log 2>&1 &
WORKER_PID=$!

# 等待 Worker 注册 (最多等待 10 秒)
for i in {1..10}; do
    WORKER_COUNT=$(curl -s "$API_BASE/api/workers" | jq -r '.data | length' 2>/dev/null || echo "0")
    if [ "$WORKER_COUNT" -gt 0 ]; then
        break
    fi
    sleep 1
done

# 检查 Worker 是否注册
WORKER_COUNT=$(curl -s "$API_BASE/api/workers" | jq -r '.data | length')
if [ "$WORKER_COUNT" -eq 0 ]; then
    echo -e "  ${RED}✗ Worker 注册失败${NC}"
    cat /tmp/worker.log
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Worker 已注册 (PID: $WORKER_PID)"

# Step 4: 创建 Agent Session
echo -e "${YELLOW}[Step 4/7] 创建 Agent Session...${NC}"
SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/agent/sessions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "glm",
        "system_prompt": "你是一个助手，可以帮助用户操作文件、执行命令、管理任务。请根据用户需求调用合适的工具。回复用中文。"
    }')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.data.id')
if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    echo -e "  ${RED}✗ 创建 Session 失败${NC}"
    echo "$SESSION_RESPONSE"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Session 已创建: $SESSION_ID"

# Step 5: 发送用户任务
echo -e "${YELLOW}[Step 5/7] 发送用户任务...${NC}"
echo -e "  任务: ${BLUE}$USER_TASK${NC}"

# 使用 jq 构建 JSON 以正确转义特殊字符
JSON_PAYLOAD=$(echo -n "$USER_TASK" | jq -Rs '{"content": .}')
if [ -z "$JSON_PAYLOAD" ] || [ "$JSON_PAYLOAD" = "null" ]; then
    JSON_PAYLOAD="{\"content\": \"$USER_TASK\"}"
fi

# 发送消息并获取 SSE 响应
MESSAGE_RESPONSE=$(curl -s -X POST "$API_BASE/api/agent/sessions/$SESSION_ID/messages/tools" \
    -H "Content-Type: application/json" \
    -d "$JSON_PAYLOAD" \
    --max-time 180)

echo -e "  ${GREEN}✓${NC} 消息已发送"

# 解析 SSE 响应，提取 AI 回复
AI_RESPONSE=$(echo "$MESSAGE_RESPONSE" | grep -o '"type":"done"[^}]*"content":"[^"]*"' | head -1 | sed 's/.*"content":"//; s/"$//' | head -c 200)
TOOL_CALLS=$(echo "$MESSAGE_RESPONSE" | grep -c '"type":"tool_calls"' || echo "0")

echo -e "  AI 回复摘要: ${GREEN}${AI_RESPONSE}...${NC}"

# Step 6: 获取工具执行记录
echo -e "${YELLOW}[Step 6/7] 获取工具执行记录...${NC}"
sleep 2
TOOL_EXECUTIONS=$(curl -s "$API_BASE/api/agent/sessions/$SESSION_ID/tools/executions")
TOOL_COUNT=$(echo "$TOOL_EXECUTIONS" | jq -r '.data | length')
echo -e "  工具调用次数: ${BLUE}$TOOL_COUNT${NC}"

# Step 7: 获取消息历史
echo -e "${YELLOW}[Step 7/7] 获取消息历史...${NC}"
MESSAGES=$(curl -s "$API_BASE/api/agent/sessions/$SESSION_ID/messages")
MESSAGE_COUNT=$(echo "$MESSAGES" | jq -r '.data | length')

# 获取最后一条 AI 回复
LAST_AI_MESSAGE=$(echo "$MESSAGES" | jq -r '.data[-1].content // "无回复"')
echo -e "  AI 回复: ${GREEN}$LAST_AI_MESSAGE${NC}"

# 获取系统状态
SYSTEM_STATS=$(curl -s "$API_BASE/api/system/stats")
WORKERS_STATUS=$(curl -s "$API_BASE/api/workers")
TASKS_STATUS=$(curl -s "$API_BASE/api/tasks")

# 生成报告
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}生成测试报告...${NC}"
echo -e "${BLUE}========================================${NC}"

cat > "$REPORT_FILE" << EOF
# Cowork 业务场景测试报告

## 测试信息

| 项目 | 值 |
|------|-----|
| 测试时间 | $(date '+%Y-%m-%d %H:%M:%S') |
| Session ID | $SESSION_ID |
| 用户任务 | $USER_TASK |

## 测试结果摘要

| 指标 | 值 |
|------|-----|
| Worker 状态 | $(echo "$WORKERS_STATUS" | jq -r '.data[0].status // "N/A"') |
| 工具调用次数 | $TOOL_COUNT |
| 消息数量 | $MESSAGE_COUNT |
| AI 最终回复 | $LAST_AI_MESSAGE |

## 工具执行详情

\`\`\`json
$TOOL_EXECUTIONS
\`\`\`

## 消息历史

\`\`\`json
$MESSAGES
\`\`\`

## 系统状态

\`\`\`json
$SYSTEM_STATS
\`\`\`

## Worker 状态

\`\`\`json
$WORKERS_STATUS
\`\`\`

## 任务状态

\`\`\`json
$TASKS_STATUS
\`\`\`

## Coordinator 日志 (最后 50 行)

\`\`\`
$(tail -50 /tmp/coordinator.log 2>/dev/null || echo "无日志")
\`\`\`

## Worker 日志 (最后 20 行)

\`\`\`
$(tail -20 /tmp/worker.log 2>/dev/null || echo "无日志")
\`\`\`

---
*报告生成时间: $(date '+%Y-%m-%d %H:%M:%S')*
EOF

echo -e "${GREEN}✓ 测试报告已生成: $REPORT_FILE${NC}"
echo ""

# 显示报告摘要
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}测试摘要${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  用户任务: ${YELLOW}$USER_TASK${NC}"
echo -e "  工具调用: ${BLUE}$TOOL_COUNT${NC} 次"
echo -e "  AI 回复: ${GREEN}$LAST_AI_MESSAGE${NC}"
echo -e "  报告文件: ${BLUE}$REPORT_FILE${NC}"
echo ""

# 打开报告（可选）
if command -v bat &> /dev/null; then
    echo -e "${YELLOW}报告预览:${NC}"
    bat --style=plain "$REPORT_FILE" | head -60
else
    echo -e "${YELLOW}报告预览:${NC}"
    head -60 "$REPORT_FILE"
fi