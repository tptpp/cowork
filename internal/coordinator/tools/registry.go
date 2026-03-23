package tools

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// Registry 工具注册中心
type Registry struct {
	store   store.ToolDefinitionStore
	cache   map[string]*models.ToolDefinition
	mu      sync.RWMutex
}

// NewRegistry 创建工具注册中心
func NewRegistry(store store.ToolDefinitionStore) *Registry {
	return &Registry{
		store: store,
		cache: make(map[string]*models.ToolDefinition),
	}
}

// Initialize 初始化工具注册中心，注册内置工具
func (r *Registry) Initialize() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 注册内置工具
	builtinTools := GetBuiltinTools()
	for _, tool := range builtinTools {
		// 检查是否已存在
		existing, err := r.store.GetByName(tool.Name)
		if err == nil && existing != nil {
			// 已存在，更新缓存
			r.cache[tool.Name] = existing
			continue
		}

		// 不存在，创建新记录
		tool.ID = uuid.New().String()
		if err := r.store.Create(tool); err != nil {
			return fmt.Errorf("failed to create builtin tool %s: %w", tool.Name, err)
		}
		r.cache[tool.Name] = tool
	}

	// 加载所有已启用的工具到缓存
	tools, err := r.store.GetEnabled()
	if err != nil {
		return fmt.Errorf("failed to load enabled tools: %w", err)
	}

	for _, tool := range tools {
		r.cache[tool.Name] = &tool
	}

	return nil
}

// Register 注册新工具
func (r *Registry) Register(tool *models.ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查名称是否已存在
	if _, exists := r.cache[tool.Name]; exists {
		return fmt.Errorf("tool %s already exists", tool.Name)
	}

	// 生成 ID
	if tool.ID == "" {
		tool.ID = uuid.New().String()
	}

	// 保存到数据库
	if err := r.store.Create(tool); err != nil {
		return fmt.Errorf("failed to create tool: %w", err)
	}

	// 更新缓存
	r.cache[tool.Name] = tool

	return nil
}

// Get 获取工具定义
func (r *Registry) Get(name string) (*models.ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.cache[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// GetByID 根据 ID 获取工具定义
func (r *Registry) GetByID(id string) (*models.ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.cache {
		if tool.ID == id {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool with id %s not found", id)
}

// List 列出所有工具
func (r *Registry) List() []*models.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*models.ToolDefinition, 0, len(r.cache))
	for _, tool := range r.cache {
		tools = append(tools, tool)
	}

	return tools
}

// ListByCategory 按类别列出工具
func (r *Registry) ListByCategory(category models.ToolCategory) []*models.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []*models.ToolDefinition
	for _, tool := range r.cache {
		if tool.Category == category {
			tools = append(tools, tool)
		}
	}

	return tools
}

// GetBuiltinTools 获取内置工具列表
func (r *Registry) GetBuiltinTools() []*models.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var builtinTools []*models.ToolDefinition
	for _, tool := range r.cache {
		if tool.IsBuiltin {
			builtinTools = append(builtinTools, tool)
		}
	}

	return builtinTools
}

// GetToolNames 获取所有工具名称列表（按字母排序）
func (r *Registry) GetToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.cache))
	for name := range r.cache {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Update 更新工具定义
func (r *Registry) Update(tool *models.ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否存在
	if _, exists := r.cache[tool.Name]; !exists {
		return fmt.Errorf("tool %s not found", tool.Name)
	}

	// 更新数据库
	if err := r.store.Update(tool); err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	// 更新缓存
	r.cache[tool.Name] = tool

	return nil
}

// Delete 删除工具
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.cache[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	// 不允许删除内置工具
	if tool.IsBuiltin {
		return fmt.Errorf("cannot delete builtin tool %s", name)
	}

	// 从数据库删除
	if err := r.store.Delete(tool.ID); err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	// 从缓存删除
	delete(r.cache, name)

	return nil
}

// Enable 启用工具
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 先检查缓存
	if tool, exists := r.cache[name]; exists {
		tool.IsEnabled = true
		if err := r.store.Update(tool); err != nil {
			return fmt.Errorf("failed to update tool: %w", err)
		}
		return nil
	}

	// 缓存中没有，从数据库加载
	tool, err := r.store.GetByName(name)
	if err != nil {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.IsEnabled = true
	if err := r.store.Update(tool); err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	// 添加到缓存
	r.cache[name] = tool

	return nil
}

// Disable 禁用工具
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.cache[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.IsEnabled = false

	// 更新数据库
	if err := r.store.Update(tool); err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	// 从缓存移除
	delete(r.cache, name)

	return nil
}

// GetOpenAITools 获取 OpenAI 格式的工具列表
func (r *Registry) GetOpenAITools() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(r.cache))
	for _, tool := range r.cache {
		tools = append(tools, tool.ToOpenAITool())
	}

	return tools
}

// GetOpenAIToolsByNames 根据名称列表获取 OpenAI 格式的工具
func (r *Registry) GetOpenAIToolsByNames(names []string) []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		if tool, exists := r.cache[name]; exists {
			tools = append(tools, tool.ToOpenAITool())
		}
	}

	return tools
}

// Reload 从数据库重新加载工具
func (r *Registry) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空缓存
	r.cache = make(map[string]*models.ToolDefinition)

	// 加载所有已启用的工具
	tools, err := r.store.GetEnabled()
	if err != nil {
		return fmt.Errorf("failed to reload tools: %w", err)
	}

	for _, tool := range tools {
		r.cache[tool.Name] = &tool
	}

	return nil
}