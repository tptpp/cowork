package agent

import (
	"fmt"
	"sync"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// DependencyManager 依赖管理器
// 负责管理任务之间的依赖关系，确定任务是否可以执行
type DependencyManager struct {
	depStore  store.TaskDependencyStore
	taskStore store.TaskStore

	// 缓存
	dependencyCache map[string][]string // taskID -> 依赖的 taskID 列表
	mu              sync.RWMutex
}

// NewDependencyManager 创建依赖管理器
func NewDependencyManager(
	depStore store.TaskDependencyStore,
	taskStore store.TaskStore,
) *DependencyManager {
	return &DependencyManager{
		depStore:        depStore,
		taskStore:       taskStore,
		dependencyCache: make(map[string][]string),
	}
}

// CanStartTask 检查任务是否可以开始执行
// 返回是否可以开始，以及未满足的依赖列表
func (m *DependencyManager) CanStartTask(taskID string) (bool, []string, error) {
	// 获取任务的所有依赖
	dependencies, err := m.depStore.GetByTaskID(taskID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	// 如果没有依赖，可以开始
	if len(dependencies) == 0 {
		return true, nil, nil
	}

	var unsatisfied []string

	for _, dep := range dependencies {
		// 如果依赖已经满足，跳过
		if dep.IsSatisfied {
			continue
		}

		// 检查被依赖任务的状态
		depTask, err := m.taskStore.Get(dep.DependsOnTaskID)
		if err != nil {
			return false, nil, fmt.Errorf("failed to get dependency task %s: %w", dep.DependsOnTaskID, err)
		}

		// 根据依赖类型检查
		satisfied := m.checkDependencySatisfied(dep.Type, depTask)

		if satisfied {
			// 更新依赖状态
			if err := m.depStore.MarkSatisfied(dep.ID); err != nil {
				return false, nil, fmt.Errorf("failed to mark dependency satisfied: %w", err)
			}
		} else {
			unsatisfied = append(unsatisfied, dep.DependsOnTaskID)
		}
	}

	return len(unsatisfied) == 0, unsatisfied, nil
}

// checkDependencySatisfied 检查依赖是否满足
func (m *DependencyManager) checkDependencySatisfied(depType models.DependencyType, depTask *models.Task) bool {
	switch depType {
	case models.DependencyTypeFinish:
		// 任务完成即可
		return depTask.Status == models.TaskStatusCompleted ||
			depTask.Status == models.TaskStatusFailed ||
			depTask.Status == models.TaskStatusCancelled

	case models.DependencyTypeSuccess:
		// 任务必须成功
		return depTask.Status == models.TaskStatusCompleted

	case models.DependencyTypeFailure:
		// 任务必须失败
		return depTask.Status == models.TaskStatusFailed ||
			depTask.Status == models.TaskStatusCancelled

	default:
		return depTask.Status == models.TaskStatusCompleted
	}
}

// GetReadyTasks 获取可以开始执行的任务列表
func (m *DependencyManager) GetReadyTasks(groupID string) ([]string, error) {
	// 获取任务组中的所有待执行任务
	group, err := m.depStore.GetGroupTasks(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group tasks: %w", err)
	}

	var readyTasks []string

	for _, task := range group {
		if task.Status != models.TaskStatusPending {
			continue
		}

		canStart, _, err := m.CanStartTask(task.ID)
		if err != nil {
			return nil, err
		}

		if canStart {
			readyTasks = append(readyTasks, task.ID)
		}
	}

	return readyTasks, nil
}

// GetBlockedTasks 获取被阻塞的任务列表
func (m *DependencyManager) GetBlockedTasks(groupID string) (map[string][]string, error) {
	// 获取任务组中的所有待执行任务
	group, err := m.depStore.GetGroupTasks(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group tasks: %w", err)
	}

	blocked := make(map[string][]string)

	for _, task := range group {
		if task.Status != models.TaskStatusPending {
			continue
		}

		canStart, unsatisfied, err := m.CanStartTask(task.ID)
		if err != nil {
			return nil, err
		}

		if !canStart {
			blocked[task.ID] = unsatisfied
		}
	}

	return blocked, nil
}

// UpdateDependencies 更新依赖状态
// 当一个任务状态改变时调用，更新所有依赖该任务的其他任务
func (m *DependencyManager) UpdateDependencies(completedTaskID string) error {
	// 获取所有依赖此任务的依赖记录
	dependents, err := m.depStore.GetDependents(completedTaskID)
	if err != nil {
		return fmt.Errorf("failed to get dependents: %w", err)
	}

	for _, dep := range dependents {
		// 检查并更新状态
		depTask, err := m.taskStore.Get(dep.DependsOnTaskID)
		if err != nil {
			continue
		}

		if m.checkDependencySatisfied(dep.Type, depTask) {
			if err := m.depStore.MarkSatisfied(dep.ID); err != nil {
				return fmt.Errorf("failed to mark dependency satisfied: %w", err)
			}
		}
	}

	return nil
}

// GetDependencyGraph 获取依赖关系图
// 返回邻接表表示的图（任务 -> 它依赖的任务）
func (m *DependencyManager) GetDependencyGraph(groupID string) (map[string][]string, error) {
	group, err := m.depStore.GetGroupTasks(groupID)
	if err != nil {
		return nil, err
	}

	graph := make(map[string][]string)

	for _, task := range group {
		deps, err := m.depStore.GetByTaskID(task.ID)
		if err != nil {
			return nil, err
		}

		for _, dep := range deps {
			graph[task.ID] = append(graph[task.ID], dep.DependsOnTaskID)
		}

		// 确保即使没有依赖也有条目
		if _, exists := graph[task.ID]; !exists {
			graph[task.ID] = []string{}
		}
	}

	return graph, nil
}

// GetExecutionLayers 获取执行层级
// 将任务按依赖关系分层，同一层的任务可以并行执行
func (m *DependencyManager) GetExecutionLayers(groupID string) ([][]string, error) {
	graph, err := m.GetDependencyGraph(groupID)
	if err != nil {
		return nil, err
	}

	// 计算入度
	inDegree := make(map[string]int)
	reverseGraph := make(map[string][]string) // 反向图

	for taskID := range graph {
		inDegree[taskID] = 0
		reverseGraph[taskID] = []string{}
	}

	for taskID, deps := range graph {
		for _, dep := range deps {
			reverseGraph[dep] = append(reverseGraph[dep], taskID)
			inDegree[taskID]++
		}
	}

	var layers [][]string

	for {
		// 找出入度为 0 的任务
		var layer []string
		for taskID, degree := range inDegree {
			if degree == 0 {
				layer = append(layer, taskID)
			}
		}

		if len(layer) == 0 {
			break
		}

		layers = append(layers, layer)

		// 移除这些任务，更新入度
		for _, taskID := range layer {
			delete(inDegree, taskID)
			for _, dependent := range reverseGraph[taskID] {
				if _, exists := inDegree[dependent]; exists {
					inDegree[dependent]--
				}
			}
		}
	}

	// 检查是否有剩余任务（循环依赖）
	if len(inDegree) > 0 {
		return nil, fmt.Errorf("circular dependency detected, remaining tasks: %v", inDegree)
	}

	return layers, nil
}

// ValidateDependencies 验证依赖关系是否有效
func (m *DependencyManager) ValidateDependencies(taskID string, dependsOn []string) error {
	// 检查自依赖
	for _, dep := range dependsOn {
		if dep == taskID {
			return fmt.Errorf("task cannot depend on itself")
		}
	}

	// 检查依赖任务是否存在
	for _, dep := range dependsOn {
		_, err := m.taskStore.Get(dep)
		if err != nil {
			return fmt.Errorf("dependency task %s not found", dep)
		}
	}

	return nil
}

// GetCriticalPath 获取关键路径
// 返回最长依赖链的任务列表
func (m *DependencyManager) GetCriticalPath(groupID string) ([]string, error) {
	graph, err := m.GetDependencyGraph(groupID)
	if err != nil {
		return nil, err
	}

	// 动态规划计算最长路径
	dp := make(map[string]int)
	path := make(map[string]string)

	var dfs func(taskID string) int
	dfs = func(taskID string) int {
		if val, exists := dp[taskID]; exists {
			return val
		}

		maxLen := 0
		for _, dep := range graph[taskID] {
			depLen := dfs(dep)
			if depLen+1 > maxLen {
				maxLen = depLen + 1
				path[taskID] = dep
			}
		}

		dp[taskID] = maxLen
		return maxLen
	}

	// 计算所有任务的最大深度
	maxDepth := 0
	var endTask string

	for taskID := range graph {
		depth := dfs(taskID)
		if depth > maxDepth {
			maxDepth = depth
			endTask = taskID
		}
	}

	// 重建路径
	var criticalPath []string
	current := endTask
	for current != "" {
		criticalPath = append([]string{current}, criticalPath...)
		current = path[current]
	}

	return criticalPath, nil
}