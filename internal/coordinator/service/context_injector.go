package service

import (
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
	if taskID, ok := msg.Context["task_id"].(string); ok {
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
	tasks, _, err := s.taskStore.List(store.ListOptions{PageSize: 1000})
	if err != nil {
		return nil
	}

	var downstream []string
	for _, task := range tasks {
		for _, req := range task.Requires {
			if req == taskID {
				downstream = append(downstream, task.ID)
			}
		}
	}

	return downstream
}