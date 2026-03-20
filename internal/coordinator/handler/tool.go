package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/errors"
	"github.com/tp/cowork/internal/shared/models"
)

// ToolHandler 工具 API 处理器
type ToolHandler struct {
	registry *tools.Registry
}

// NewToolHandler 创建工具处理器
func NewToolHandler(registry *tools.Registry) *ToolHandler {
	return &ToolHandler{
		registry: registry,
	}
}

// ToolResponse 工具响应
type ToolResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  models.JSON           `json:"parameters"`
	Category    models.ToolCategory   `json:"category"`
	ExecuteMode models.ToolExecuteMode `json:"execute_mode"`
	Permission  models.ToolPermission `json:"permission"`
	Handler     string                `json:"handler"`
	IsEnabled   bool                  `json:"is_enabled"`
	IsBuiltin   bool                  `json:"is_builtin"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
}

// ToolListResponse 工具列表响应
type ToolListResponse struct {
	Tools []ToolResponse `json:"tools"`
	Total int            `json:"total"`
}

// fromModel 从模型转换
func toolFromModel(tool *models.ToolDefinition) ToolResponse {
	return ToolResponse{
		ID:          tool.ID,
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  tool.Parameters,
		Category:    tool.Category,
		ExecuteMode: tool.ExecuteMode,
		Permission:  tool.Permission,
		Handler:     tool.Handler,
		IsEnabled:   tool.IsEnabled,
		IsBuiltin:   tool.IsBuiltin,
		CreatedAt:   tool.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   tool.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// GetTools 获取工具列表
// GET /api/tools
func (h *ToolHandler) GetTools(c *gin.Context) {
	category := c.Query("category")
	enabled := c.Query("enabled")
	openaiFormat := c.Query("openai") == "true"

	// 如果请求 OpenAI 格式
	if openaiFormat {
		var openaiTools []map[string]interface{}
		if category != "" {
			openaiTools = h.registry.GetOpenAIToolsByNames([]string{category})
		} else {
			openaiTools = h.registry.GetOpenAITools()
		}
		success(c, gin.H{"tools": openaiTools})
		return
	}

	// 获取所有工具
	allTools := h.registry.List()

	// 过滤
	var filtered []ToolResponse
	for _, tool := range allTools {
		// 类别过滤
		if category != "" && string(tool.Category) != category {
			continue
		}
		// 启用状态过滤
		if enabled == "true" && !tool.IsEnabled {
			continue
		}
		if enabled == "false" && tool.IsEnabled {
			continue
		}
		filtered = append(filtered, toolFromModel(tool))
	}

	success(c, ToolListResponse{
		Tools: filtered,
		Total: len(filtered),
	})
}

// GetTool 获取工具详情
// GET /api/tools/:name
func (h *ToolHandler) GetTool(c *gin.Context) {
	name := c.Param("name")

	tool, err := h.registry.Get(name)
	if err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "Tool not found: "+name))
		return
	}

	success(c, toolFromModel(tool))
}

// CreateToolRequest 创建工具请求
type CreateToolRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description" binding:"required"`
	Parameters  map[string]interface{} `json:"parameters" binding:"required"`
	Category    string                 `json:"category"`
	ExecuteMode string                 `json:"execute_mode"`
	Permission  string                 `json:"permission"`
	Handler     string                 `json:"handler"`
	IsEnabled   *bool                  `json:"is_enabled"`
}

// CreateTool 创建自定义工具
// POST /api/tools
func (h *ToolHandler) CreateTool(c *gin.Context) {
	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 设置默认值
	category := models.ToolCategoryCustom
	if req.Category != "" {
		category = models.ToolCategory(req.Category)
	}

	executeMode := models.ToolExecuteModeRemote
	if req.ExecuteMode != "" {
		executeMode = models.ToolExecuteMode(req.ExecuteMode)
	}

	permission := models.ToolPermissionRead
	if req.Permission != "" {
		permission = models.ToolPermission(req.Permission)
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	// 创建工具定义
	tool := &models.ToolDefinition{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Parameters:  req.Parameters,
		Category:    category,
		ExecuteMode: executeMode,
		Permission:  permission,
		Handler:     req.Handler,
		IsEnabled:   isEnabled,
		IsBuiltin:   false,
	}

	if err := h.registry.Register(tool); err != nil {
		failWithError(c, errors.New(errors.CodeConflict, err.Error()))
		return
	}

	success(c, toolFromModel(tool))
}

// UpdateToolRequest 更新工具请求
type UpdateToolRequest struct {
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Category    string                 `json:"category"`
	ExecuteMode string                 `json:"execute_mode"`
	Permission  string                 `json:"permission"`
	Handler     string                 `json:"handler"`
	IsEnabled   *bool                  `json:"is_enabled"`
}

// UpdateTool 更新工具
// PUT /api/tools/:name
func (h *ToolHandler) UpdateTool(c *gin.Context) {
	name := c.Param("name")

	// 获取现有工具
	tool, err := h.registry.Get(name)
	if err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "Tool not found: "+name))
		return
	}

	// 不允许修改内置工具
	if tool.IsBuiltin {
		failWithError(c, errors.New(errors.CodeForbidden, "Cannot modify builtin tool"))
		return
	}

	var req UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 更新字段
	if req.Description != "" {
		tool.Description = req.Description
	}
	if req.Parameters != nil {
		tool.Parameters = req.Parameters
	}
	if req.Category != "" {
		tool.Category = models.ToolCategory(req.Category)
	}
	if req.ExecuteMode != "" {
		tool.ExecuteMode = models.ToolExecuteMode(req.ExecuteMode)
	}
	if req.Permission != "" {
		tool.Permission = models.ToolPermission(req.Permission)
	}
	if req.Handler != "" {
		tool.Handler = req.Handler
	}
	if req.IsEnabled != nil {
		tool.IsEnabled = *req.IsEnabled
	}

	if err := h.registry.Update(tool); err != nil {
		failWithError(c, errors.WrapInternal("Failed to update tool", err))
		return
	}

	success(c, toolFromModel(tool))
}

// DeleteTool 删除工具
// DELETE /api/tools/:name
func (h *ToolHandler) DeleteTool(c *gin.Context) {
	name := c.Param("name")

	if err := h.registry.Delete(name); err != nil {
		if err.Error() == "tool "+name+" not found" {
			failWithError(c, errors.New(errors.CodeNotFound, "Tool not found: "+name))
			return
		}
		if err.Error() == "cannot delete builtin tool "+name {
			failWithError(c, errors.New(errors.CodeForbidden, "Cannot delete builtin tool"))
			return
		}
		failWithError(c, errors.WrapInternal("Failed to delete tool", err))
		return
	}

	success(c, gin.H{"name": name})
}

// EnableTool 启用工具
// POST /api/tools/:name/enable
func (h *ToolHandler) EnableTool(c *gin.Context) {
	name := c.Param("name")

	if err := h.registry.Enable(name); err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "Tool not found: "+name))
		return
	}

	success(c, gin.H{"name": name, "enabled": true})
}

// DisableTool 禁用工具
// POST /api/tools/:name/disable
func (h *ToolHandler) DisableTool(c *gin.Context) {
	name := c.Param("name")

	if err := h.registry.Disable(name); err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "Tool not found: "+name))
		return
	}

	success(c, gin.H{"name": name, "enabled": false})
}