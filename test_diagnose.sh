#!/bin/bash
# Cowork 系统诊断测试脚本
# 用法: ./test_diagnose.sh [场景描述]
# 示例: ./test_diagnose.sh "创建 /tmp/cowork-test/README.md 文件"

set -e

# 配置
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COORDINATOR="$SCRIPT_DIR/bin/coordinator"
WORKER="$SCRIPT_DIR/bin/worker"
API_BASE="http://localhost:8080"
REPORT_FILE="/tmp/cowork-diagnose-$(date +%Y%m%d_%H%M%S).md"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 默认测试场景
SCENARIO="${1:-列出 /tmp 目录下的前5个文件}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Cowork 系统诊断测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "测试场景: ${YELLOW}$SCENARIO${NC}"
echo -e "报告文件: ${BLUE}$REPORT_FILE${NC}"
echo ""

# 初始化报告
cat > "$REPORT_FILE" << EOF
# Cowork 系统诊断报告

**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')
**测试场景**: $SCENARIO

---

## 1. 环境检查

EOF

# 1. 检查二进制文件
echo -e "${YELLOW}[1/8] 检查二进制文件...${NC}"
BIN_STATUS=""
if [ -x "$COORDINATOR" ]; then
    echo "  ✅ coordinator: $(ls -lh $COORDINATOR | awk '{print $5}')"
    BIN_STATUS+="- coordinator: ✅\n"
else
    echo "  ❌ coordinator 不存在或不可执行"
    BIN_STATUS+="- coordinator: ❌ 不存在\n"
fi

if [ -x "$WORKER" ]; then
    echo "  ✅ worker: $(ls -lh $WORKER | awk '{print $5}')"
    BIN_STATUS+="- worker: ✅\n"
else
    echo "  ❌ worker 不存在或不可执行"
    BIN_STATUS+="- worker: ❌ 不存在\n"
fi

cat >> "$REPORT_FILE" << EOF
$BIN_STATUS

## 2. 配置检查

EOF

# 2. 检查配置文件
echo -e "${YELLOW}[2/8] 检查配置文件...${NC}"
CONFIG_STATUS=""
if [ -f ~/.cowork/setting.json ]; then
    echo "  ✅ 配置文件存在"
    AI_BASE_URL=$(jq -r '.coordinator.ai_base_url // "N/A"' ~/.cowork/setting.json)
    AI_MODEL=$(jq -r '.coordinator.ai_model // "N/A"' ~/.cowork/setting.json)
    HAS_API_KEY=$(jq -r '.coordinator.ai_api_key // ""' ~/.cowork/setting.json | wc -c)
    echo "  Base URL: $AI_BASE_URL"
    echo "  Model: $AI_MODEL"
    if [ "$HAS_API_KEY" -gt 10 ]; then
        echo "  ✅ API Key 已配置"
        CONFIG_STATUS+="- API Key: ✅\n"
    else
        echo "  ❌ API Key 未配置或太短"
        CONFIG_STATUS+="- API Key: ❌\n"
    fi
else
    echo "  ❌ 配置文件不存在: ~/.cowork/setting.json"
    CONFIG_STATUS+="- 配置文件: ❌ 不存在\n"
fi

cat >> "$REPORT_FILE" << EOF
$CONFIG_STATUS

## 3. 服务启动

EOF

# 3. 清理并启动服务
echo -e "${YELLOW}[3/8] 清理并启动服务...${NC}"

# 停止旧进程
pkill -f "bin/coordinator" 2>/dev/null || true
pkill -f "bin/worker" 2>/dev/null || true
sleep 2

# 清理旧数据
rm -rf ~/.cowork/coordinator/cowork.db 2>/dev/null || true
echo "  清理旧数据完成"

# 启动 coordinator
COWORK_LOG_LEVEL=info "$COORDINATOR" > /tmp/coordinator.log 2>&1 &
COORD_PID=$!
echo "  启动 coordinator (PID: $COORD_PID)"

# 等待 coordinator 启动
for i in {1..15}; do
    if curl -s "$API_BASE/health" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

if curl -s "$API_BASE/health" > /dev/null 2>&1; then
    echo "  ✅ Coordinator 已就绪"
    SERVICE_STATUS="- Coordinator: ✅ (PID: $COORD_PID)\n"
else
    echo "  ❌ Coordinator 启动失败"
    SERVICE_STATUS="- Coordinator: ❌ 启动失败\n"
    tail -20 /tmp/coordinator.log
fi

# 启动 worker
"$WORKER" --name test-worker > /tmp/worker.log 2>&1 &
WORKER_PID=$!
echo "  启动 worker (PID: $WORKER_PID)"

sleep 5

# 检查 worker 注册
WORKER_COUNT=$(curl -s "$API_BASE/api/workers" | jq -r '.data | length' 2>/dev/null || echo "0")
if [ "$WORKER_COUNT" -gt 0 ]; then
    echo "  ✅ Worker 已注册"
    SERVICE_STATUS+="- Worker: ✅ (PID: $WORKER_PID)\n"
else
    echo "  ❌ Worker 注册失败"
    SERVICE_STATUS+="- Worker: ❌ 注册失败\n"
fi

cat >> "$REPORT_FILE" << EOF
$SERVICE_STATUS

## 4. Agent Session 测试

EOF

# 4. 创建 Agent Session
echo -e "${YELLOW}[4/8] 创建 Agent Session...${NC}"
SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/agent/sessions" \
    -H "Content-Type: application/json" \
    -d '{"model": "glm", "system_prompt": "你是一个助手，可以使用工具帮助用户完成任务。"}')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.data.id // empty')
if [ -n "$SESSION_ID" ]; then
    echo "  ✅ Session 创建成功: $SESSION_ID"
    SESSION_STATUS="- Session ID: $SESSION_ID\n"
else
    echo "  ❌ Session 创建失败"
    echo "  响应: $SESSION_RESPONSE"
    SESSION_STATUS="- Session: ❌ 创建失败\n"
fi

cat >> "$REPORT_FILE" << EOF
$SESSION_STATUS

## 5. 工具检查

EOF

# 5. 检查可用工具
echo -e "${YELLOW}[5/8] 检查可用工具...${NC}"
TOOLS_RESPONSE=$(curl -s "$API_BASE/api/agent/tools")
TOOL_COUNT=$(echo "$TOOLS_RESPONSE" | jq -r '.data | length' 2>/dev/null || echo "0")
TOOL_NAMES=$(echo "$TOOLS_RESPONSE" | jq -r '.data[].function.name' 2>/dev/null | tr '\n' ', ' | sed 's/,$//')

echo "  可用工具数量: $TOOL_COUNT"
echo "  工具列表: $TOOL_NAMES"

cat >> "$REPORT_FILE" << EOF
- 工具数量: $TOOL_COUNT
- 工具列表: $TOOL_NAMES

## 6. 任务执行测试

EOF

# 6. 发送测试消息
echo -e "${YELLOW}[6/8] 发送测试消息...${NC}"
echo "  场景: $SCENARIO"

timeout 90 curl -s -X POST "$API_BASE/api/agent/sessions/$SESSION_ID/messages/tools" \
    -H "Content-Type: application/json" \
    -d "{\"content\": \"$SCENARIO\"}" \
    -o /tmp/ai_response.txt 2>/dev/null

# 分析响应
echo ""
echo -e "${YELLOW}分析响应...${NC}"

# 检查是否是模拟响应
if grep -q "simulated AI response" /tmp/ai_response.txt 2>/dev/null; then
    echo "  ⚠️  返回模拟响应 - API Key 可能未正确配置"
    RESPONSE_STATUS="- 响应类型: ⚠️ 模拟响应\n"
else
    echo "  ✅ 返回真实 AI 响应"
    RESPONSE_STATUS="- 响应类型: ✅ 真实响应\n"
fi

# 检查工具调用
TOOL_CALL_COUNT=$(grep -c '"type":"tool_calls"' /tmp/ai_response.txt 2>/dev/null || echo "0")
echo "  工具调用次数: $TOOL_CALL_COUNT"
RESPONSE_STATUS+="- 工具调用次数: $TOOL_CALL_COUNT\n"

# 检查错误
ERROR_COUNT=$(grep -c '"type":"error"' /tmp/ai_response.txt 2>/dev/null || echo "0")
# 确保是数字
ERROR_COUNT=$(echo "$ERROR_COUNT" | head -1 | tr -d '[:space:]')
if [ "${ERROR_COUNT:-0}" -gt 0 ]; then
    echo "  ❌ 发现 $ERROR_COUNT 个错误"
    ERROR_MSG=$(grep '"type":"error"' /tmp/ai_response.txt | head -3)
    echo "  错误信息: $ERROR_MSG"
    RESPONSE_STATUS+="- 错误数: $ERROR_COUNT\n"
else
    echo "  ✅ 无错误"
    RESPONSE_STATUS+="- 错误数: 0\n"
fi

# 提取 AI 回复摘要
AI_REPLY=$(grep '"type":"done"' /tmp/ai_response.txt | head -1 | sed 's/.*"content":"//; s/".*//' | head -c 200)
echo ""
echo "  AI 回复摘要:"
echo "  ${GREEN}$AI_REPLY${NC}"

cat >> "$REPORT_FILE" << EOF
$RESPONSE_STATUS
- AI 回复摘要: $AI_REPLY

## 7. 任务执行检查

EOF

# 7. 检查任务执行
echo ""
echo -e "${YELLOW}[7/8] 检查任务执行...${NC}"

# 等待任务完成
sleep 10

# 检查任务状态
TASKS_RESPONSE=$(curl -s "$API_BASE/api/tasks")
TOTAL_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '.data | length' 2>/dev/null || echo "0")
PENDING_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '[.data[] | select(.status=="pending")] | length' 2>/dev/null || echo "0")
RUNNING_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '[.data[] | select(.status=="running")] | length' 2>/dev/null || echo "0")
COMPLETED_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '[.data[] | select(.status=="completed")] | length' 2>/dev/null || echo "0")
FAILED_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '[.data[] | select(.status=="failed")] | length' 2>/dev/null || echo "0")

echo "  总任务数: $TOTAL_TASKS"
echo "  待处理: $PENDING_TASKS, 运行中: $RUNNING_TASKS, 完成: $COMPLETED_TASKS, 失败: $FAILED_TASKS"

cat >> "$REPORT_FILE" << EOF
- 总任务数: $TOTAL_TASKS
- 待处理: $PENDING_TASKS
- 运行中: $RUNNING_TASKS
- 完成: $COMPLETED_TASKS
- 失败: $FAILED_TASKS

### 任务详情

\`\`\`json
$TASKS_RESPONSE
\`\`\`

## 8. 工具执行记录

EOF

# 8. 检查工具执行记录
echo ""
echo -e "${YELLOW}[8/8] 检查工具执行记录...${NC}"
TOOL_EXECS=$(curl -s "$API_BASE/api/agent/sessions/$SESSION_ID/tools/executions")
EXECS_COUNT=$(echo "$TOOL_EXECS" | jq -r '.data | length' 2>/dev/null || echo "0")
echo "  工具执行记录数: $EXECS_COUNT"

if [ "$EXECS_COUNT" -gt 0 ]; then
    echo ""
    echo "  工具执行详情:"
    echo "$TOOL_EXECS" | jq -r '.data[] | "  - \(.tool_name): \(.status)"' 2>/dev/null | head -10
fi

cat >> "$REPORT_FILE" << EOF
- 工具执行记录数: $EXECS_COUNT

\`\`\`json
$TOOL_EXECS
\`\`\`

---

## 9. 日志摘要

### Coordinator 日志 (最后 30 行)

\`\`\`
$(tail -30 /tmp/coordinator.log 2>/dev/null || echo "无日志")
\`\`\`

### Worker 日志 (最后 20 行)

\`\`\`
$(tail -20 /tmp/worker.log 2>/dev/null || echo "无日志")
\`\`\`

---

## 10. 诊断结论

EOF

# 诊断结论
CONCLUSION=""
ISSUES=0

if [ "$TOOL_CALL_COUNT" -eq 0 ]; then
    CONCLUSION+="- ⚠️ AI 未调用任何工具，可能是模型不支持 Function Calling 或工具定义有问题\n"
    ((ISSUES++))
fi

if [ "$PENDING_TASKS" -gt 0 ]; then
    CONCLUSION+="- ⚠️ 有 $PENDING_TASKS 个任务处于 pending 状态，可能是调度器或 Worker 心跳问题\n"
    ((ISSUES++))
fi

if [ "$FAILED_TASKS" -gt 0 ]; then
    CONCLUSION+="- ⚠️ 有 $FAILED_TASKS 个任务失败\n"
    ((ISSUES++))
fi

if grep -q "simulated AI response" /tmp/ai_response.txt 2>/dev/null; then
    CONCLUSION+="- ❌ API Key 未正确配置，返回模拟响应\n"
    ((ISSUES++))
fi

if [ "$ISSUES" -eq 0 ]; then
    CONCLUSION+="- ✅ 系统运行正常，未发现明显问题\n"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}诊断结论${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "$CONCLUSION"

cat >> "$REPORT_FILE" << EOF
$CONCLUSION

---

*报告生成时间: $(date '+%Y-%m-%d %H:%M:%S')*
EOF

echo ""
echo -e "${GREEN}✅ 诊断报告已生成: $REPORT_FILE${NC}"
echo ""

# 清理（可选）
# pkill -f "bin/coordinator" 2>/dev/null || true
# pkill -f "bin/worker" 2>/dev/null || true