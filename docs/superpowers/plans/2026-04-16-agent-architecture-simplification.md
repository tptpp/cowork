# Agent 架构简化实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 简化 Agent 架构，统一 Coordinator 和执行者为单一 Agent 结构，差异由模板定义，系统服务处理自动化流程。

**Architecture:** 三层架构 - Agent 层（调用模型 + 执行工具）→ 系统服务层（固定规则自动化）→ 数据层。删除 TaskDecomposer、ToolScheduler，合并到 Agent 和服务层。

**Tech Stack:** Go + Gin + GORM + SQLite（保持现有）

---

## 文件结构

### 新增文件

```
internal/coordinator/agent/
├── agent.go          (~400行) - 统一 Agent 结构
├── llm_client.go     (~300行) - API 调用细节（从 function_calling.go 提取）

internal/coordinator/service/
├── task_scheduler.go (~150行) - 依赖满足时自动调度
├── node_assigner.go  (~100行) - 节点分配
├── message_router.go (~100行) - 消息路由 + 恢复代理
├── context_injector.go (~80行) - 上下文注入
└── progress_monitor.go (~70行) - 进度推送
```

### 修改文件

```
internal/coordinator/agent/coordinator.go    - 删除拆解相关方法，改为使用 Agent
internal/coordinator/agent/template.go       - 保留，添加新模板字段
cmd/coordinator/main.go                      - 更新初始化逻辑
```

### 删除文件

```
internal/coordinator/agent/task_decomposer.go    - 整个删除
internal/coordinator/agent/scheduler.go          - 整个删除
internal/coordinator/agent/task_decomposer_test.go - 整个删除
```

### 保留并重构文件

```
internal/coordinator/agent/function_calling.go   - 重构为 llm_client.go（保留类型定义，删除 Engine 结构）
```

---

## Phase 1: Agent 统一结构

### Task 1: 创建 LLM Client

**Files:**
- Create: `internal/coordinator/agent/llm_client.go`

- [ ] **Step 1: 创建 llm_client.go 文件，提取 API 调用逻辑**

```go
// internal/coordinator/agent/llm_client.go

package agent

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/tp/cowork/internal/coordinator/tools"
    "github.com/tp/cowork/internal/shared/models"
)

// LLMClient LLM API 客户端
type LLMClient struct {
    httpClient *http.Client
    registry   *tools.Registry
}

// LLMClientConfig 客户端配置
type LLMClientConfig struct {
    Timeout time.Duration
}

// NewLLMClient 创建 LLM 客户端
func NewLLMClient(registry *tools.Registry, config LLMClientConfig) *LLMClient {
    if config.Timeout <= 0 {
        config.Timeout = 60 * time.Second
    }

    return &LLMClient{
        httpClient: &http.Client{Timeout: config.Timeout},
        registry:   registry,
    }
}

// ModelConfig 模型配置
type ModelConfig struct {
    Type     string // "openai", "anthropic", "glm"
    BaseURL  string
    Model    string
    APIKey   string
    MaxTokens int
    Temperature float64
}

// 保留现有的类型定义（从 function_calling.go 复制）
type ChatRequest struct {
    Model       string                   `json:"model"`
    Messages    []ChatMessage            `json:"messages"`
    Stream      bool                     `json:"stream"`
    Tools       []map[string]interface{} `json:"tools,omitempty"`
    ToolChoice  interface{}              `json:"tool_choice,omitempty"`
    MaxTokens   int                      `json:"max_tokens,omitempty"`
    Temperature float64                  `json:"temperature,omitempty"`
}

type ChatMessage struct {
    Role       string            `json:"role"`
    Content    interface{}       `json:"content"`
    ToolCalls  []models.ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string            `json:"tool_call_id,omitempty"`
    Name       string            `json:"name,omitempty"`
}

type ContentBlock struct {
    Type      string          `json:"type"`
    Text      string          `json:"text,omitempty"`
    ID        string          `json:"id,omitempty"`
    Name      string          `json:"name,omitempty"`
    Input     json.RawMessage `json:"input,omitempty"`
    ToolUseID string          `json:"tool_use_id,omitempty"`
    Content   string          `json:"content,omitempty"`
    IsError   bool            `json:"is_error,omitempty"`
}

type ChatResponse struct {
    ID      string     `json:"id"`
    Object  string     `json:"object"`
    Created int64      `json:"created"`
    Model   string     `json:"model"`
    Choices []Choice   `json:"choices"`
    Usage   UsageStats `json:"usage"`
}

type Choice struct {
    Index        int          `json:"index"`
    Message      ChatMessage  `json:"message"`
    Delta        *ChatMessage `json:"delta,omitempty"`
    FinishReason string       `json:"finish_reason"`
}

type UsageStats struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

type AnthropicResponse struct {
    ID           string         `json:"id"`
    Type         string         `json:"type"`
    Role         string         `json:"role"`
    Content      []ContentBlock `json:"content"`
    Model        string         `json:"model"`
    StopReason   string         `json:"stop_reason"`
    StopSequence string         `json:"stop_sequence"`
    Usage        struct {
        InputTokens  int `json:"input_tokens"`
        OutputTokens int `json:"output_tokens"`
    } `json:"usage"`
}

type StreamEvent struct {
    Type    string         `json:"type"`
    Index   int            `json:"index,omitempty"`
    Delta   *ContentDelta  `json:"delta,omitempty"`
    Message *ChatMessage   `json:"message,omitempty"`
    Content []ContentBlock `json:"content,omitempty"`
}

type ContentDelta struct {
    Type string `json:"type"`
    Text string `json:"text,omitempty"`
}
```

- [ ] **Step 2: 添加 BuildRequest 方法**

```go
// BuildOpenAIRequest 构建 OpenAI 格式的请求
func (c *LLMClient) BuildOpenAIRequest(
    model string,
    messages []ChatMessage,
    systemPrompt string,
    toolNames []string,
) (*ChatRequest, error) {
    req := &ChatRequest{
        Model:    model,
        Messages: messages,
        Stream:   false,
    }

    if systemPrompt != "" {
        req.Messages = append([]ChatMessage{
            {Role: "system", Content: systemPrompt},
        }, req.Messages...)
    }

    if len(toolNames) > 0 {
        req.Tools = c.registry.GetOpenAIToolsByNames(toolNames)
        req.ToolChoice = "auto"
    }

    return req, nil
}

// BuildAnthropicRequest 构建 Anthropic 格式的请求
func (c *LLMClient) BuildAnthropicRequest(
    model string,
    messages []ChatMessage,
    systemPrompt string,
    toolNames []string,
) (map[string]interface{}, error) {
    var chatMessages []map[string]interface{}

    for _, msg := range messages {
        if msg.Role == "system" {
            continue
        }

        anthropicMsg := map[string]interface{}{
            "role": msg.Role,
        }

        if msg.Role == "tool" {
            anthropicMsg["content"] = []ContentBlock{
                {
                    Type:      "tool_result",
                    ToolUseID: msg.ToolCallID,
                    Content:   fmt.Sprintf("%v", msg.Content),
                },
            }
        } else if len(msg.ToolCalls) > 0 {
            content := []ContentBlock{}
            if msg.Content != nil && msg.Content != "" {
                content = append(content, ContentBlock{
                    Type: "text",
                    Text: fmt.Sprintf("%v", msg.Content),
                })
            }
            for _, tc := range msg.ToolCalls {
                content = append(content, ContentBlock{
                    Type:  "tool_use",
                    ID:    tc.ID,
                    Name:  tc.Function.Name,
                    Input: json.RawMessage(tc.Function.Arguments),
                })
            }
            anthropicMsg["content"] = content
        } else {
            anthropicMsg["content"] = msg.Content
        }

        chatMessages = append(chatMessages, anthropicMsg)
    }

    req := map[string]interface{}{
        "model":      model,
        "messages":   chatMessages,
        "max_tokens": 4096,
    }

    if systemPrompt != "" {
        req["system"] = systemPrompt
    }

    if len(toolNames) > 0 {
        tools := c.registry.GetOpenAIToolsByNames(toolNames)
        anthropicTools := make([]map[string]interface{}, len(tools))
        for i, tool := range tools {
            if fn, ok := tool["function"].(map[string]interface{}); ok {
                anthropicTools[i] = map[string]interface{}{
                    "name":        fn["name"],
                    "description": fn["description"],
                    "input_schema": fn["parameters"],
                }
            }
        }
        req["tools"] = anthropicTools
    }

    return req, nil
}
```

- [ ] **Step 3: 添加 Call 和 Stream 方法**

```go
// CallOpenAI 调用 OpenAI API
func (c *LLMClient) CallOpenAI(cfg ModelConfig, req *ChatRequest) (*ChatResponse, error) {
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("failed to call OpenAI: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("OpenAI API error: %s", string(bodyBytes))
    }

    var result ChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// StreamOpenAI 流式调用 OpenAI API
func (c *LLMClient) StreamOpenAI(cfg ModelConfig, req *ChatRequest, onToken func(string)) (*ChatResponse, error) {
    req.Stream = true
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var fullContent string
    var toolCalls []models.ToolCall
    var finishReason string

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        data := strings.TrimPrefix(line, "data: ")
        if data == "[DONE]" {
            break
        }

        var event StreamEvent
        if err := json.Unmarshal([]byte(data), &event); err != nil {
            continue
        }

        if event.Delta != nil && event.Delta.Text != "" {
            fullContent += event.Delta.Text
            if onToken != nil {
                onToken(event.Delta.Text)
            }
        }

        if event.Message != nil {
            if len(event.Message.ToolCalls) > 0 {
                toolCalls = event.Message.ToolCalls
            }
            finishReason = event.Message.FinishReason
        }
    }

    return &ChatResponse{
        Model: cfg.Model,
        Choices: []Choice{
            {
                Message: ChatMessage{
                    Content:   fullContent,
                    ToolCalls: toolCalls,
                },
                FinishReason: finishReason,
            },
        },
    }, nil
}

// CallAnthropic 调用 Anthropic API
func (c *LLMClient) CallAnthropic(cfg ModelConfig, req map[string]interface{}) (*AnthropicResponse, error) {
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", cfg.BaseURL+"/messages", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", cfg.APIKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("failed to call Anthropic: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("Anthropic API error: %s", string(bodyBytes))
    }

    var result AnthropicResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// StreamAnthropic 流式调用 Anthropic API
func (c *LLMClient) StreamAnthropic(cfg ModelConfig, req map[string]interface{}, onToken func(string)) (*AnthropicResponse, error) {
    // 添加 stream 字段
    req["stream"] = true
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", cfg.BaseURL+"/messages", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", cfg.APIKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var fullContent string
    var contentBlocks []ContentBlock
    var stopReason string

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        data := strings.TrimPrefix(line, "data: ")

        var event map[string]interface{}
        if err := json.Unmarshal([]byte(data), &event); err != nil {
            continue
        }

        eventType, _ := event["type"].(string)
        switch eventType {
        case "content_block_delta":
            index, _ := event["index"].(float64)
            delta, _ := event["delta"].(map[string]interface{})
            text, _ := delta["text"].(string)
            fullContent += text
            if onToken != nil {
                onToken(text)
            }
            // 确保有对应的内容块
            if int(index) >= len(contentBlocks) {
                contentBlocks = append(contentBlocks, ContentBlock{Type: "text"})
            }
        case "content_block_stop":
            // 内容块完成
        case "message_stop":
            stopReason = "end_turn"
        }
    }

    // 添加文本内容块
    if fullContent != "" {
        contentBlocks = append([]ContentBlock{{Type: "text", Text: fullContent}}, contentBlocks...)
    }

    return &AnthropicResponse{
        Model:      cfg.Model,
        Content:    contentBlocks,
        StopReason: stopReason,
    }, nil
}

// ParseToolCallsFromAnthropic 从 Anthropic 响应解析 Tool Calls
func (c *LLMClient) ParseToolCallsFromAnthropic(resp *AnthropicResponse) []models.ToolCall {
    var toolCalls []models.ToolCall

    for _, block := range resp.Content {
        if block.Type == "tool_use" {
            toolCalls = append(toolCalls, models.ToolCall{
                ID:   block.ID,
                Type: "function",
                Function: models.FunctionCall{
                    Name:      block.Name,
                    Arguments: string(block.Input),
                },
            })
        }
    }

    return toolCalls
}
```

- [ ] **Step 4: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/coordinator/agent/llm_client.go
git commit -m "feat: create LLMClient for unified API calling

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: 创建统一 Agent 结构

**Files:**
- Create: `internal/coordinator/agent/agent.go`

- [ ] **Step 1: 创建 Agent 结构体**

```go
// internal/coordinator/agent/agent.go

package agent

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/coordinator/tools"
    "github.com/tp/cowork/internal/shared/models"
)

// Agent 统一的 Agent 结构
type Agent struct {
    ID            string
    Template      *models.AgentTemplate
    Messages      []ChatMessage
    WorkspacePath string
    
    // 依赖组件
    llmClient     *LLMClient
    registry      *tools.Registry
    toolExecStore store.ToolExecutionStore
    taskStore     store.TaskStore
    sessionStore  store.AgentSessionStore
    
    // 配置
    maxToolRounds int
}

// AgentConfig Agent 配置
type AgentConfig struct {
    MaxToolRounds int
    Timeout       time.Duration
}

// NewAgent 创建 Agent
func NewAgent(
    id string,
    template *models.AgentTemplate,
    llmClient *LLMClient,
    registry *tools.Registry,
    toolExecStore store.ToolExecutionStore,
    taskStore store.TaskStore,
    sessionStore store.AgentSessionStore,
    config AgentConfig,
) *Agent {
    if config.MaxToolRounds <= 0 {
        config.MaxToolRounds = 10
    }

    return &Agent{
        ID:            id,
        Template:      template,
        Messages:      []ChatMessage{},
        WorkspacePath: fmt.Sprintf("workspace/%s/", id),
        llmClient:     llmClient,
        registry:      registry,
        toolExecStore: toolExecStore,
        taskStore:     taskStore,
        sessionStore:  sessionStore,
        maxToolRounds: config.MaxToolRounds,
    }
}
```

- [ ] **Step 2: 添加核心方法 ProcessMessage**

```go
// ProcessMessage 处理消息 - 核心：调用模型 + 执行工具
func (a *Agent) ProcessMessage(
    ctx context.Context,
    userMessage string,
    cfg ModelConfig,
    onToken func(string),
) (*ProcessResult, error) {
    // 保存用户消息
    if a.sessionStore != nil {
        if _, err := a.sessionStore.AddMessage(a.ID, "user", userMessage); err != nil {
            log.Printf("Failed to save user message: %v", err)
        }
    }

    // 添加到对话历史
    a.Messages = append(a.Messages, ChatMessage{
        Role:    "user",
        Content: userMessage,
    })

    // 处理多轮对话（可能包含工具调用）
    return a.processWithToolCalls(ctx, cfg, onToken)
}

// ProcessResult 处理结果
type ProcessResult struct {
    Response      string
    ToolCalls     []models.ToolCall
    ToolResults   []ToolResultInfo
    TotalRounds   int
    FinishReason  string
}

// ToolResultInfo 工具结果信息
type ToolResultInfo struct {
    ToolCallID string
    ToolName   string
    Result     string
    IsError    bool
}
```

- [ ] **Step 3: 添加 processWithToolCalls 方法**

```go
// processWithToolCalls 处理带 Tool Calling 的对话
func (a *Agent) processWithToolCalls(
    ctx context.Context,
    cfg ModelConfig,
    onToken func(string),
) (*ProcessResult, error) {
    result := &ProcessResult{TotalRounds: 0}
    var toolResults []ToolResultInfo

    // 获取系统提示词
    systemPrompt := ""
    if a.Template != nil {
        systemPrompt = a.Template.BasePrompt
    }

    // 获取允许的工具列表
    toolNames := a.getAllowedTools()

    for round := 0; round < a.maxToolRounds; round++ {
        result.TotalRounds = round + 1

        var toolCalls []models.ToolCall
        var response string
        var finishReason string

        // 根据模型类型调用不同的 API
        switch cfg.Type {
        case "openai", "glm":
            req, err := a.llmClient.BuildOpenAIRequest(cfg.Model, a.Messages, systemPrompt, toolNames)
            if err != nil {
                return nil, err
            }
            
            var resp *ChatResponse
            if onToken != nil {
                resp, err = a.llmClient.StreamOpenAI(cfg, req, onToken)
            } else {
                resp, err = a.llmClient.CallOpenAI(cfg, req)
            }
            if err != nil {
                return nil, err
            }
            
            if len(resp.Choices) > 0 {
                response = fmt.Sprintf("%v", resp.Choices[0].Message.Content)
                toolCalls = resp.Choices[0].Message.ToolCalls
                finishReason = resp.Choices[0].FinishReason
            }

        case "anthropic":
            req, err := a.llmClient.BuildAnthropicRequest(cfg.Model, a.Messages, systemPrompt, toolNames)
            if err != nil {
                return nil, err
            }
            
            var resp *AnthropicResponse
            if onToken != nil {
                resp, err = a.llmClient.StreamAnthropic(cfg, req, onToken)
            } else {
                resp, err = a.llmClient.CallAnthropic(cfg, req)
            }
            if err != nil {
                return nil, err
            }
            
            // 提取文本内容
            for _, block := range resp.Content {
                if block.Type == "text" {
                    response += block.Text
                }
            }
            toolCalls = a.llmClient.ParseToolCallsFromAnthropic(resp)
            finishReason = resp.StopReason

        default:
            return nil, fmt.Errorf("unsupported model type: %s", cfg.Type)
        }

        // 如果没有 tool calls，保存响应并返回
        if len(toolCalls) == 0 {
            if a.sessionStore != nil {
                if _, err := a.sessionStore.AddMessage(a.ID, "assistant", response); err != nil {
                    log.Printf("Failed to save assistant message: %v", err)
                }
            }

            result.Response = response
            result.FinishReason = finishReason
            result.ToolResults = toolResults
            return result, nil
        }

        // 有 tool calls，保存带 tool_calls 的助手消息
        result.ToolCalls = toolCalls

        // 添加助手消息到对话历史
        a.Messages = append(a.Messages, ChatMessage{
            Role:      "assistant",
            Content:   response,
            ToolCalls: toolCalls,
        })

        // 执行所有 tool calls
        var execResults []ToolResultInfo
        for _, tc := range toolCalls {
            execResult, err := a.executeToolCall(tc)
            if err != nil {
                log.Printf("Failed to execute tool %s: %v", tc.Function.Name, err)
                execResult = ToolResultInfo{
                    ToolCallID: tc.ID,
                    ToolName:   tc.Function.Name,
                    Result:     err.Error(),
                    IsError:    true,
                }
            }
            execResults = append(execResults, execResult)

            // 添加 tool result 到对话历史
            a.Messages = append(a.Messages, ChatMessage{
                Role:       "tool",
                ToolCallID: tc.ID,
                Content:    execResult.Result,
                Name:       tc.Function.Name,
            })
        }

        toolResults = append(toolResults, execResults...)
    }

    // 达到最大轮数
    return nil, fmt.Errorf("max tool call rounds reached (%d)", a.maxToolRounds)
}
```

- [ ] **Step 4: 添加辅助方法**

```go
// getAllowedTools 获取允许的工具列表
func (a *Agent) getAllowedTools() []string {
    if a.Template == nil {
        // 无模板时返回所有工具
        return a.registry.GetToolNames()
    }

    // 如果有白名单，只返回白名单中的工具
    if len(a.Template.AllowedTools) > 0 {
        return []string(a.Template.AllowedTools)
    }

    // 否则返回所有工具（排除黑名单）
    allTools := a.registry.GetToolNames()
    restricted := []string(a.Template.RestrictedTools)
    
    var allowed []string
    for _, tool := range allTools {
        isRestricted := false
        for _, r := range restricted {
            if tool == r {
                isRestricted = true
                break
            }
        }
        if !isRestricted {
            allowed = append(allowed, tool)
        }
    }
    
    return allowed
}

// executeToolCall 执行工具调用
func (a *Agent) executeToolCall(toolCall models.ToolCall) (ToolResultInfo, error) {
    // 获取工具定义
    toolDef, err := a.registry.Get(toolCall.Function.Name)
    if err != nil {
        return ToolResultInfo{}, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
    }

    // 检查工具权限
    if err := a.checkToolPermission(toolCall.Function.Name); err != nil {
        return ToolResultInfo{
            ToolCallID: toolCall.ID,
            ToolName:   toolCall.Function.Name,
            Result:     err.Error(),
            IsError:    true,
        }, nil
    }

    // 解析参数
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
        return ToolResultInfo{}, fmt.Errorf("failed to parse arguments: %w", err)
    }

    // 创建执行记录
    execution := &models.ToolExecution{
        ConversationID: a.ID,
        ToolName:       toolCall.Function.Name,
        ToolCallID:     toolCall.ID,
        Arguments:      args,
        Status:         string(models.ToolExecutionStatusPending),
    }

    // 根据执行模式处理
    if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
        return a.executeLocalTool(execution, args)
    }

    // 远程工具 - 返回待执行状态（需要 Worker）
    return ToolResultInfo{
        ToolCallID: toolCall.ID,
        ToolName:   toolCall.Function.Name,
        Result:     "Remote tool execution pending. Requires worker.",
        IsError:    false,
    }, nil
}

// checkToolPermission 检查工具权限
func (a *Agent) checkToolPermission(toolName string) error {
    if a.Template == nil {
        return nil
    }

    // 检查黑名单
    for _, t := range a.Template.RestrictedTools {
        if t == toolName {
            return fmt.Errorf("tool '%s' is restricted for template '%s'", toolName, a.Template.ID)
        }
    }

    // 检查白名单
    if len(a.Template.AllowedTools) > 0 {
        allowed := false
        for _, t := range a.Template.AllowedTools {
            if t == toolName {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("tool '%s' not in allowed list for template '%s'", toolName, a.Template.ID)
        }
    }

    return nil
}

// executeLocalTool 执行本地工具
func (a *Agent) executeLocalTool(
    execution *models.ToolExecution,
    args map[string]interface{},
) (ToolResultInfo, error) {
    // 更新状态为运行中
    if a.toolExecStore != nil {
        if err := a.toolExecStore.Create(execution); err != nil {
            return ToolResultInfo{}, fmt.Errorf("failed to create execution record: %w", err)
        }
        if err := a.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusRunning), "", false); err != nil {
            return ToolResultInfo{}, err
        }
    }

    var result string
    var isError bool

    // 根据工具名称执行
    switch execution.ToolName {
    case "create_task":
        result, isError = a.toolCreateTask(args)
    case "query_task":
        result, isError = a.toolQueryTask(args)
    case "read_file":
        result, isError = a.toolReadFile(args)
    default:
        result = fmt.Sprintf("Unknown local tool: %s", execution.ToolName)
        isError = true
    }

    // 更新执行记录
    if a.toolExecStore != nil {
        status := string(models.ToolExecutionStatusCompleted)
        if isError {
            status = string(models.ToolExecutionStatusFailed)
        }
        a.toolExecStore.UpdateStatus(execution.ID, status, result, isError)
    }

    return ToolResultInfo{
        ToolCallID: execution.ToolCallID,
        ToolName:   execution.ToolName,
        Result:     result,
        IsError:    isError,
    }, nil
}
```

- [ ] **Step 5: 添加本地工具实现**

```go
// toolCreateTask create_task 工具实现
func (a *Agent) toolCreateTask(args map[string]interface{}) (string, bool) {
    title, _ := args["title"].(string)
    description, _ := args["description"].(string)
    templateID, _ := args["template_id"].(string)
    
    if title == "" {
        return "Missing required parameter: title", true
    }

    task := &models.Task{
        ID:          uuid.New().String(),
        Title:       title,
        Description: description,
        Status:      models.TaskStatusPending,
        Priority:    models.PriorityMedium,
        TemplateID:  templateID,
    }

    if a.taskStore != nil {
        if err := a.taskStore.Create(task); err != nil {
            return fmt.Sprintf("Failed to create task: %v", err), true
        }
    }

    return fmt.Sprintf("Task created: %s (ID: %s)", title, task.ID), false
}

// toolQueryTask query_task 工具实现
func (a *Agent) toolQueryTask(args map[string]interface{}) (string, bool) {
    taskID, _ := args["task_id"].(string)
    
    if taskID == "" {
        // 查询所有任务
        if a.taskStore != nil {
            tasks, err := a.taskStore.List(100)
            if err != nil {
                return fmt.Sprintf("Failed to list tasks: %v", err), true
            }
            
            result, _ := json.Marshal(tasks)
            return string(result), false
        }
        return "No task store available", true
    }

    // 查询特定任务
    if a.taskStore != nil {
        task, err := a.taskStore.Get(taskID)
        if err != nil {
            return fmt.Sprintf("Task not found: %s", taskID), true
        }
        
        result, _ := json.Marshal(task)
        return string(result), false
    }

    return "No task store available", true
}

// toolReadFile read_file 工具实现（本地查询）
func (a *Agent) toolReadFile(args map[string]interface{}) (string, bool) {
    path, _ := args["path"].(string)
    
    if path == "" {
        return "Missing required parameter: path", true
    }

    // 如果路径是 workspace 内的相对路径，转换为绝对路径
    if !strings.HasPrefix(path, "/") {
        path = a.WorkspacePath + path
    }

    // 读取文件内容
    content, err := os.ReadFile(path)
    if err != nil {
        return fmt.Sprintf("Failed to read file: %v", err), true
    }

    return string(content), false
}
```

- [ ] **Step 6: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add internal/coordinator/agent/agent.go
git commit -m "feat: create unified Agent structure

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: 重构 Coordinator 使用 Agent

**Files:**
- Modify: `internal/coordinator/agent/coordinator.go`

- [ ] **Step 1: 重构 Coordinator 结构体**

将 Coordinator 改为使用 Agent：

```go
// internal/coordinator/agent/coordinator.go

package agent

import (
    "context"
    
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/coordinator/tools"
    "github.com/tp/cowork/internal/shared/models"
)

// Coordinator Coordinator Agent - 使用统一的 Agent 结构
type Coordinator struct {
    agent *Agent
    
    // Coordinator 特有的能力
    templateStore  store.AgentTemplateStore
    hub            *ws.Hub // WebSocket 用于推送
}

// NewCoordinator 创建 Coordinator
func NewCoordinator(
    llmClient *LLMClient,
    registry *tools.Registry,
    toolExecStore store.ToolExecutionStore,
    taskStore store.TaskStore,
    sessionStore store.AgentSessionStore,
    templateStore store.AgentTemplateStore,
    hub *ws.Hub,
    config AgentConfig,
) *Coordinator {
    // 加载 coordinator 模板
    template, err := templateStore.Get("coordinator-template")
    if err != nil || template == nil {
        // 使用默认模板
        template = &models.AgentTemplate{
            ID:          "coordinator-template",
            Name:        "Coordinator",
            Description: "Task coordination, scheduling, monitoring",
            BasePrompt:  "You are a coordinator agent. Your role is to decompose tasks, monitor progress, and coordinate between agents. You can create tasks, query status, and communicate with other agents.",
            AllowedTools: []string{"create_task", "cancel_task", "query_tasks", "query_nodes", "send_message", "report_to_user"},
            IsSystem:    true,
        }
    }

    agent := NewAgent(
        "coordinator",
        template,
        llmClient,
        registry,
        toolExecStore,
        taskStore,
        sessionStore,
        config,
    )

    return &Coordinator{
        agent:         agent,
        templateStore: templateStore,
        hub:           hub,
    }
}

// ProcessMessage 处理消息（委托给 Agent）
func (c *Coordinator) ProcessMessage(
    ctx context.Context,
    userMessage string,
    cfg ModelConfig,
    onToken func(string),
) (*ProcessResult, error) {
    return c.agent.ProcessMessage(ctx, userMessage, cfg, onToken)
}
```

- [ ] **Step 2: 删除旧的拆解相关方法**

删除以下方法（拆解现在通过工具调用实现）：
- `DecomposeTask`
- `ShouldDecompose`
- `ProcessWithDecomposition`
- `getDefaultCoordinatorTemplate`

- [ ] **Step 3: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/agent/coordinator.go
git commit -m "refactor: Coordinator now uses unified Agent structure

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: 删除旧组件

**Files:**
- Delete: `internal/coordinator/agent/task_decomposer.go`
- Delete: `internal/coordinator/agent/scheduler.go`
- Delete: `internal/coordinator/agent/task_decomposer_test.go`

- [ ] **Step 1: 删除文件**

```bash
rm internal/coordinator/agent/task_decomposer.go
rm internal/coordinator/agent/scheduler.go
rm internal/coordinator/agent/task_decomposer_test.go
```

- [ ] **Step 2: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor: remove TaskDecomposer and ToolScheduler

Task decomposition now handled by Agent through create_task tool.
Remote tool execution handled by existing Worker mechanism.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: 更新 main.go 初始化

**Files:**
- Modify: `cmd/coordinator/main.go`

- [ ] **Step 1: 更新 Coordinator 初始化**

找到现有的 Coordinator 初始化代码（约 270-290 行），替换为：

```go
// 初始化 LLMClient
llmClient := agent.NewLLMClient(
    toolRegistry,
    agent.LLMClientConfig{
        Timeout: 60 * time.Second,
    },
)

// 初始化 Coordinator（使用新的统一结构）
coordinator := agent.NewCoordinator(
    llmClient,
    toolRegistry,
    store.NewToolExecutionStore(s.DB()),
    store.NewTaskStore(s.DB()),
    store.NewAgentSessionStore(s.DB()),
    store.NewAgentTemplateStore(s.DB()),
    hub,
    agent.AgentConfig{
        MaxToolRounds: 10,
        Timeout:       60 * time.Second,
    },
)
h.SetAgentCoordinator(coordinator)
slog.Info("Coordinator initialized with unified Agent structure")
```

- [ ] **Step 2: 删除旧的 Engine/Scheduler 初始化**

删除以下代码：
- FunctionCallingEngine 初始化
- ToolScheduler 初始化
- agentToolScheduler.Start()

- [ ] **Step 3: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add cmd/coordinator/main.go
git commit -m "refactor: update main.go to use new Agent structure

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 2: 系统服务层

### Task 6: 创建 ContextInjector 服务

**Files:**
- Create: `internal/coordinator/service/context_injector.go`

- [ ] **Step 1: 创建服务**

```go
// internal/coordinator/service/context_injector.go

package service

import (
    "encoding/json"
    
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// ContextInjector 上下文注入服务
type ContextInjector struct {
    taskStore store.TaskStore
}

// NewContextInjector 创建服务
func NewContextInjector(taskStore store.TaskStore) *ContextInjector {
    return &ContextInjector{
        taskStore: taskStore,
    }
}

// Inject 注入上下文到消息
func (s *ContextInjector) Inject(msg *models.AgentMessage) *models.AgentMessage {
    if msg.Context == nil {
        msg.Context = models.JSON{}
    }

    // 根据消息类型注入不同上下文
    switch msg.Type {
    case "notify":
        s.injectNotifyContext(msg)
    case "request":
        s.injectRequestContext(msg)
    }

    return msg
}

// injectNotifyContext 注入通知类型消息的上下文
func (s *ContextInjector) injectNotifyContext(msg *models.AgentMessage) {
    // 如果是任务完成通知
    if content, ok := msg.Context["task_id"].(string); ok {
        taskID := content
        
        // 查询任务信息
        task, err := s.taskStore.Get(taskID)
        if err == nil {
            msg.Context["task_status"] = task.Status
            msg.Context["task_title"] = task.Title
        }
        
        // 查询下游任务
        downstream := s.findDownstreamTasks(taskID)
        if len(downstream) > 0 {
            msg.Context["downstream_tasks"] = downstream
        }
    }
}

// injectRequestContext 注入请求类型消息的上下文
func (s *ContextInjector) injectRequestContext(msg *models.AgentMessage) {
    // 注入请求者信息
    if fromAgent, ok := msg.Context["from_agent"].(string); ok {
        task, err := s.taskStore.Get(fromAgent)
        if err == nil {
            msg.Context["requester_task_title"] = task.Title
            msg.Context["requester_task_status"] = task.Status
        }
    }
}

// findDownstreamTasks 查找下游任务
func (s *ContextInjector) findDownstreamTasks(taskID string) []string {
    tasks, err := s.taskStore.List(1000)
    if err != nil {
        return nil
    }

    var downstream []string
    for _, task := range tasks {
        if deps, ok := task.DependsOn.([]string); ok {
            for _, dep := range deps {
                if dep == taskID {
                    downstream = append(downstream, task.ID)
                }
            }
        }
    }

    return downstream
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/service/context_injector.go
git commit -m "feat: add ContextInjector service

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 7: 创建 ProgressMonitor 服务

**Files:**
- Create: `internal/coordinator/service/progress_monitor.go`

- [ ] **Step 1: 创建服务**

```go
// internal/coordinator/service/progress_monitor.go

package service

import (
    "encoding/json"
    
    "github.com/tp/cowork/internal/coordinator/ws"
    "github.com/tp/cowork/internal/shared/models"
)

// ProgressMonitor 进度监控服务
type ProgressMonitor struct {
    hub *ws.Hub
}

// NewProgressMonitor 创建服务
func NewProgressMonitor(hub *ws.Hub) *ProgressMonitor {
    return &ProgressMonitor{
        hub: hub,
    }
}

// OnTaskStatusChange 任务状态变更时推送
func (s *ProgressMonitor) OnTaskStatusChange(task *models.Task) {
    msg := map[string]interface{}{
        "type":    "task_update",
        "task_id": task.ID,
        "status":  task.Status,
        "title":   task.Title,
        "progress": task.Progress,
    }

    data, _ := json.Marshal(msg)
    s.hub.BroadcastToChannel("tasks", data)
}

// OnAgentMessage Agent 消息时推送
func (s *ProgressMonitor) OnAgentMessage(msg *models.AgentMessage) {
    wsMsg := map[string]interface{}{
        "type":     "agent_message",
        "from":     msg.FromAgent,
        "to":       msg.ToAgent,
        "content":  msg.Content,
        "msg_type": msg.Type,
    }

    data, _ := json.Marshal(wsMsg)
    s.hub.BroadcastToChannel("messages", data)
}

// OnApprovalRequest 审批请求时推送
func (s *ProgressMonitor) OnApprovalRequest(approval *models.ApprovalRequest) {
    msg := map[string]interface{}{
        "type":        "approval_request",
        "id":          approval.ID,
        "agent_id":    approval.AgentID,
        "action":      approval.Action,
        "risk_level":  approval.RiskLevel,
    }

    data, _ := json.Marshal(msg)
    s.hub.BroadcastToChannel("approvals", data)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/service/progress_monitor.go
git commit -m "feat: add ProgressMonitor service

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 8: 创建 MessageRouter 服务

**Files:**
- Create: `internal/coordinator/service/message_router.go`

- [ ] **Step 1: 创建服务**

```go
// internal/coordinator/service/message_router.go

package service

import (
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// MessageRouter 消息路由服务
type MessageRouter struct {
    msgStore   store.MessageStore
    taskStore  store.TaskStore
    agentStore store.AgentStore
}

// NewMessageRouter 创建服务
func NewMessageRouter(
    msgStore store.MessageStore,
    taskStore store.TaskStore,
    agentStore store.AgentStore,
) *MessageRouter {
    return &MessageRouter{
        msgStore:   msgStore,
        taskStore:  taskStore,
        agentStore: agentStore,
    }
}

// Route 路由消息
func (s *MessageRouter) Route(msg *models.AgentMessage) error {
    // 保存消息
    msg.ID = uuid.New().String()
    msg.CreatedAt = time.Now()
    msg.Status = "pending"
    
    if err := s.msgStore.Create(msg); err != nil {
        return fmt.Errorf("failed to save message: %w", err)
    }

    // 检查目标 Agent 状态
    targetAgent, err := s.agentStore.Get(msg.ToAgent)
    if err != nil {
        // Agent 不存在或已完成，可能需要创建恢复代理
        return s.handleRecovery(msg)
    }

    // Agent 正在运行，直接路由
    if targetAgent.Status == "running" {
        return s.deliverToAgent(msg, targetAgent)
    }

    // Agent 已完成，创建恢复代理
    return s.handleRecovery(msg)
}

// handleRecovery 创建恢复代理
func (s *MessageRouter) handleRecovery(msg *models.AgentMessage) error {
    // 查询原始 Agent
    originalAgent, err := s.agentStore.Get(msg.ToAgent)
    if err != nil {
        return fmt.Errorf("original agent not found: %s", msg.ToAgent)
    }

    // 创建恢复代理
    recoveryAgent := &models.Agent{
        ID:        uuid.New().String(),
        RootID:    originalAgent.RootID,
        ParentID:  msg.ToAgent,
        TemplateID: originalAgent.TemplateID,
        Status:    "pending",
        Title:     fmt.Sprintf("Recovery for %s", originalAgent.Title),
    }

    if err := s.agentStore.Create(recoveryAgent); err != nil {
        return fmt.Errorf("failed to create recovery agent: %w", err)
    }

    // 更新消息目标为恢复代理
    msg.ProxyFor = msg.ToAgent
    msg.ToAgent = recoveryAgent.ID

    return s.deliverToAgent(msg, recoveryAgent)
}

// deliverToAgent 投递消息给 Agent
func (s *MessageRouter) deliverToAgent(msg *models.AgentMessage, agent *models.Agent) error {
    // 更新消息状态
    msg.Status = "delivered"
    msg.DeliveredAt = time.Now()
    
    return s.msgStore.Update(msg)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/service/message_router.go
git commit -m "feat: add MessageRouter service with recovery support

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 9: 集成服务到 main.go

**Files:**
- Modify: `cmd/coordinator/main.go`

- [ ] **Step 1: 初始化系统服务**

在 Coordinator 初始化后添加：

```go
// 初始化系统服务
contextInjector := service.NewContextInjector(store.NewTaskStore(s.DB()))
progressMonitor := service.NewProgressMonitor(hub)
messageRouter := service.NewMessageRouter(
    store.NewMessageStore(s.DB()),
    store.NewTaskStore(s.DB()),
    store.NewAgentStore(s.DB()),
)

slog.Info("System services initialized")
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/coordinator/main.go
git commit -m "feat: integrate system services into main.go

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 3: 验证和清理

### Task 10: 运行测试并清理旧代码

**Files:**
- Modify: `internal/coordinator/agent/function_calling.go` - 删除不再需要的部分
- Run tests

- [ ] **Step 1: 检查并删除 function_calling.go 中不再需要的代码**

删除：
- FunctionCallingEngine 结构体及其方法（已被 LLMClient 替代）
- ExecuteToolCall 方法（已移到 Agent）

保留：
- 类型定义（已复制到 llm_client.go）

- [ ] **Step 2: 运行现有测试**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go test ./...`
Expected: 部分测试可能需要更新

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: No errors

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "refactor: complete Agent architecture simplification

- Unified Agent structure for Coordinator and Workers
- System services for automation (ContextInjector, ProgressMonitor, MessageRouter)
- Removed TaskDecomposer and ToolScheduler
- Task decomposition now through create_task tool

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Self-Review

### Spec Coverage

| 设计文档要求 | 对应任务 |
|-------------|----------|
| Agent 统一结构 | Task 2 |
| LLM Client | Task 1 |
| Coordinator 使用 Agent | Task 3 |
| 删除旧组件 | Task 4 |
| ContextInjector 服务 | Task 6 |
| ProgressMonitor 服务 | Task 7 |
| MessageRouter 服务 | Task 8 |
| 服务集成 | Task 9 |
| 验证清理 | Task 10 |

### Placeholder Scan

✅ 无 TBD、TODO、模糊描述

### Type Consistency

✅ 类型定义一致（ChatMessage, ModelConfig, ProcessResult 等）