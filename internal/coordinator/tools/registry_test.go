package tools

import (
	"testing"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
	"gorm.io/gorm"
)

// mockToolDefinitionStore 模拟工具定义存储
type mockToolDefinitionStore struct {
	tools map[string]*models.ToolDefinition
}

func newMockStore() *mockToolDefinitionStore {
	return &mockToolDefinitionStore{
		tools: make(map[string]*models.ToolDefinition),
	}
}

func (m *mockToolDefinitionStore) Create(tool *models.ToolDefinition) error {
	m.tools[tool.Name] = tool
	return nil
}

func (m *mockToolDefinitionStore) Get(id string) (*models.ToolDefinition, error) {
	for _, tool := range m.tools {
		if tool.ID == id {
			return tool, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockToolDefinitionStore) GetByName(name string) (*models.ToolDefinition, error) {
	if tool, ok := m.tools[name]; ok {
		return tool, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockToolDefinitionStore) List(opts store.ToolListOptions) ([]models.ToolDefinition, int64, error) {
	var result []models.ToolDefinition
	for _, tool := range m.tools {
		result = append(result, *tool)
	}
	return result, int64(len(result)), nil
}

func (m *mockToolDefinitionStore) Update(tool *models.ToolDefinition) error {
	m.tools[tool.Name] = tool
	return nil
}

func (m *mockToolDefinitionStore) Delete(id string) error {
	for name, tool := range m.tools {
		if tool.ID == id {
			delete(m.tools, name)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockToolDefinitionStore) GetByCategory(category models.ToolCategory) ([]models.ToolDefinition, error) {
	var result []models.ToolDefinition
	for _, tool := range m.tools {
		if tool.Category == category {
			result = append(result, *tool)
		}
	}
	return result, nil
}

func (m *mockToolDefinitionStore) GetEnabled() ([]models.ToolDefinition, error) {
	var result []models.ToolDefinition
	for _, tool := range m.tools {
		if tool.IsEnabled {
			result = append(result, *tool)
		}
	}
	return result, nil
}

func (m *mockToolDefinitionStore) GetBuiltin() ([]models.ToolDefinition, error) {
	var result []models.ToolDefinition
	for _, tool := range m.tools {
		if tool.IsBuiltin {
			result = append(result, *tool)
		}
	}
	return result, nil
}

func TestRegistryInitialize(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)

	if err := registry.Initialize(); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 检查内置工具是否已注册
	tools := registry.List()
	if len(tools) == 0 {
		t.Error("Expected at least one tool after initialization")
	}

	// 检查 execute_shell 工具
	tool, err := registry.Get("execute_shell")
	if err != nil {
		t.Errorf("Expected to find execute_shell tool: %v", err)
	}
	if tool == nil {
		t.Error("execute_shell tool should not be nil")
	}
}

func TestRegistryRegister(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 注册自定义工具
	customTool := &models.ToolDefinition{
		ID:          "custom-1",
		Name:        "custom_tool",
		Description: "A custom tool",
		Parameters: models.JSON{
			"type": "object",
		},
		Category:    models.ToolCategoryCustom,
		ExecuteMode: models.ToolExecuteModeLocal,
		IsEnabled:   true,
		IsBuiltin:   false,
	}

	if err := registry.Register(customTool); err != nil {
		t.Fatalf("Failed to register custom tool: %v", err)
	}

	// 验证注册
	tool, err := registry.Get("custom_tool")
	if err != nil {
		t.Errorf("Expected to find custom_tool: %v", err)
	}
	if tool.Name != "custom_tool" {
		t.Error("Tool name mismatch")
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 尝试注册已存在的工具
	duplicateTool := &models.ToolDefinition{
		ID:          "dup-1",
		Name:        "execute_shell", // 已存在
		Description: "Duplicate tool",
	}

	err := registry.Register(duplicateTool)
	if err == nil {
		t.Error("Expected error when registering duplicate tool")
	}
}

func TestRegistryGet(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 获取存在的工具
	tool, err := registry.Get("execute_shell")
	if err != nil {
		t.Errorf("Failed to get execute_shell: %v", err)
	}
	if tool.Name != "execute_shell" {
		t.Error("Tool name mismatch")
	}

	// 获取不存在的工具
	_, err = registry.Get("non_existent")
	if err == nil {
		t.Error("Expected error when getting non-existent tool")
	}
}

func TestRegistryUpdate(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 更新工具
	tool, _ := registry.Get("execute_shell")
	tool.Description = "Updated description"

	if err := registry.Update(tool); err != nil {
		t.Fatalf("Failed to update tool: %v", err)
	}

	// 验证更新
	updated, _ := registry.Get("execute_shell")
	if updated.Description != "Updated description" {
		t.Error("Tool description not updated")
	}
}

func TestRegistryDelete(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 注册自定义工具
	customTool := &models.ToolDefinition{
		ID:          "custom-1",
		Name:        "custom_tool",
		Description: "A custom tool",
		IsEnabled:   true,
		IsBuiltin:   false,
	}
	_ = registry.Register(customTool)

	// 删除自定义工具
	if err := registry.Delete("custom_tool"); err != nil {
		t.Fatalf("Failed to delete custom tool: %v", err)
	}

	// 验证删除
	_, err := registry.Get("custom_tool")
	if err == nil {
		t.Error("Expected error when getting deleted tool")
	}

	// 尝试删除内置工具
	err = registry.Delete("execute_shell")
	if err == nil {
		t.Error("Expected error when deleting builtin tool")
	}
}

func TestRegistryEnableDisable(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 禁用工具
	if err := registry.Disable("execute_shell"); err != nil {
		t.Fatalf("Failed to disable tool: %v", err)
	}

	// 验证禁用
	_, err := registry.Get("execute_shell")
	if err == nil {
		t.Error("Expected error when getting disabled tool")
	}

	// 启用工具
	if err := registry.Enable("execute_shell"); err != nil {
		t.Fatalf("Failed to enable tool: %v", err)
	}

	// 验证启用
	tool, err := registry.Get("execute_shell")
	if err != nil {
		t.Errorf("Failed to get enabled tool: %v", err)
	}
	if !tool.IsEnabled {
		t.Error("Tool should be enabled")
	}
}

func TestRegistryGetOpenAITools(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	openaiTools := registry.GetOpenAITools()

	if len(openaiTools) == 0 {
		t.Error("Expected at least one OpenAI tool")
	}

	// 检查格式
	for _, tool := range openaiTools {
		if tool["type"] != "function" {
			t.Error("OpenAI tool type should be 'function'")
		}
		function, ok := tool["function"].(map[string]interface{})
		if !ok {
			t.Error("OpenAI tool function field not found")
			continue
		}
		if function["name"] == nil {
			t.Error("OpenAI tool name should not be nil")
		}
	}
}

func TestRegistryGetOpenAIToolsByNames(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	names := []string{"execute_shell", "read_file"}
	openaiTools := registry.GetOpenAIToolsByNames(names)

	if len(openaiTools) != 2 {
		t.Errorf("Expected 2 OpenAI tools, got %d", len(openaiTools))
	}
}

func TestRegistryListByCategory(t *testing.T) {
	store := newMockStore()
	registry := NewRegistry(store)
	_ = registry.Initialize()

	// 获取系统类别的工具
	systemTools := registry.ListByCategory(models.ToolCategorySystem)

	for _, tool := range systemTools {
		if tool.Category != models.ToolCategorySystem {
			t.Errorf("Expected system category, got %s", tool.Category)
		}
	}
}