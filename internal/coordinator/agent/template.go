package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// TemplateManager Agent模板管理器
type TemplateManager struct {
	store store.AgentTemplateStore
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(store store.AgentTemplateStore) *TemplateManager {
	return &TemplateManager{store: store}
}

// InitSystemTemplates 初始化系统预设模板
func (m *TemplateManager) InitSystemTemplates(ctx context.Context) error {
	templates := []models.AgentTemplate{
		{
			ID:                  "coordinator-template",
			Name:                "Coordinator",
			Description:         "负责任务拆解、调度、监控的协调者Agent",
			BasePrompt:          "你是Coordinator Agent，负责接收用户意图、拆解任务、调度节点、监控进度。你需要分析用户请求，生成任务树，管理任务依赖关系，处理Agent间消息路由。",
			AllowedTools:        models.StringArray{"dispatch_task", "assign_node", "monitor_progress", "create_agent", "send_message"},
			RequiredCapabilities: models.NodeCapabilities{},
			DefaultApprovalLevel: "low",
			IsSystem:            true,
		},
		{
			ID:                  "dev-template",
			Name:                "Developer",
			Description:         "负责代码实现的开发Agent",
			BasePrompt:          "你是开发Agent，负责代码编写、文件编辑、API实现。你需要根据任务描述和API文档完成代码开发，汇报进度里程碑。",
			AllowedTools:        models.StringArray{"read_file", "write_file", "edit_file", "execute_shell", "git", "browser"},
			RestrictedTools:     models.StringArray{"delete_file", "deploy"},
			RequiredCapabilities: models.NodeCapabilities{"editor": true},
			DefaultApprovalLevel: "medium",
			IsSystem:            true,
		},
		{
			ID:                  "test-template",
			Name:                "Tester",
			Description:         "负责测试验证的测试Agent",
			BasePrompt:          "你是测试Agent，负责测试执行、结果分析、bug报告。你需要运行测试、分析结果、发现问题后通知Coordinator创建修复任务。",
			AllowedTools:        models.StringArray{"read_file", "execute_shell", "run_test", "browser"},
			RestrictedTools:     models.StringArray{"write_file", "edit_file"},
			RequiredCapabilities: models.NodeCapabilities{"test_env": true},
			DefaultApprovalLevel: "low",
			IsSystem:            true,
		},
		{
			ID:                  "review-template",
			Name:                "Reviewer",
			Description:         "负责代码审查的ReviewAgent",
			BasePrompt:          "你是代码审查Agent，负责代码质量检查、安全审计、最佳实践建议。",
			AllowedTools:        models.StringArray{"read_file", "execute_shell"},
			RestrictedTools:     models.StringArray{"write_file", "edit_file"},
			RequiredCapabilities: models.NodeCapabilities{},
			DefaultApprovalLevel: "low",
			IsSystem:            true,
		},
		{
			ID:                  "deploy-template",
			Name:                "Deployer",
			Description:         "负责部署发布的部署Agent",
			BasePrompt:          "你是部署Agent，负责代码部署、环境配置、发布验证。所有部署操作需要用户审批。",
			AllowedTools:        models.StringArray{"read_file", "execute_shell", "deploy"},
			RequiredCapabilities: models.NodeCapabilities{"docker": true},
			DefaultApprovalLevel: "high",
			IsSystem:            true,
		},
		{
			ID:                  "research-template",
			Name:                "Researcher",
			Description:         "负责调研分析的研究Agent",
			BasePrompt:          "你是研究Agent，负责技术调研、文档分析、方案建议。",
			AllowedTools:        models.StringArray{"read_file", "browse_web", "execute_shell"},
			RestrictedTools:     models.StringArray{"write_file", "edit_file"},
			RequiredCapabilities: models.NodeCapabilities{"browser": true},
			DefaultApprovalLevel: "low",
			IsSystem:            true,
		},
	}

	for _, tmpl := range templates {
		existing, err := m.store.Get(tmpl.ID)
		if err == nil && existing != nil {
			continue
		}

		tmpl.CreatedAt = time.Now()
		if err := m.store.Create(&tmpl); err != nil {
			return fmt.Errorf("failed to create template %s: %w", tmpl.ID, err)
		}
	}
	return nil
}

// Get 获取模板
func (m *TemplateManager) Get(ctx context.Context, templateID string) (*models.AgentTemplate, error) {
	return m.store.Get(templateID)
}

// GetByName 按名称获取模板
func (m *TemplateManager) GetByName(ctx context.Context, name string) (*models.AgentTemplate, error) {
	return m.store.GetByName(name)
}

// List 列出所有模板
func (m *TemplateManager) List(ctx context.Context) ([]models.AgentTemplate, error) {
	return m.store.List()
}

// Create 创建自定义模板
func (m *TemplateManager) Create(ctx context.Context, template *models.AgentTemplate) error {
	template.ID = uuid.New().String()
	template.IsSystem = false
	return m.store.Create(template)
}

// Update 更新模板
func (m *TemplateManager) Update(ctx context.Context, template *models.AgentTemplate) error {
	if template.IsSystem {
		return fmt.Errorf("cannot update system template")
	}
	return m.store.Update(template)
}

// Delete 删除模板
func (m *TemplateManager) Delete(ctx context.Context, templateID string) error {
	template, err := m.store.Get(templateID)
	if err != nil {
		return err
	}
	if template.IsSystem {
		return fmt.Errorf("cannot delete system template")
	}
	return m.store.Delete(templateID)
}

// MatchTemplate 根据任务类型匹配合适的模板
func (m *TemplateManager) MatchTemplate(ctx context.Context, taskType string, taskDescription string) (*models.AgentTemplate, error) {
	templates, err := m.store.ListSystem()
	if err != nil {
		return nil, err
	}

	switch taskType {
	case "develop", "code", "implement":
		return m.findTemplateByName(templates, "Developer")
	case "test", "testing":
		return m.findTemplateByName(templates, "Tester")
	case "review", "audit":
		return m.findTemplateByName(templates, "Reviewer")
	case "deploy", "release":
		return m.findTemplateByName(templates, "Deployer")
	case "research", "analyze":
		return m.findTemplateByName(templates, "Researcher")
	default:
		return m.findTemplateByName(templates, "Developer")
	}
}

func (m *TemplateManager) findTemplateByName(templates []models.AgentTemplate, name string) (*models.AgentTemplate, error) {
	for _, t := range templates {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template %s not found", name)
}