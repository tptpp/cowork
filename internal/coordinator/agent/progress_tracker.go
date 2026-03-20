package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// ProgressTracker 进度追踪器
// 负责追踪任务组的整体进度，生成进度报告
type ProgressTracker struct {
	groupStore store.TaskGroupStore
	taskStore  store.TaskStore
	depManager *DependencyManager

	// 进度回调
	onProgress func(groupID string, report *models.TaskProgressReport)

	// 缓存
	reports      map[string]*models.TaskProgressReport
	estimatedDurations map[string]time.Duration // task type -> estimated duration
	mu           sync.RWMutex
}

// ProgressTrackerConfig 进度追踪器配置
type ProgressTrackerConfig struct {
	// 默认任务预估时间（按类型）
	DefaultDurations map[string]time.Duration
}

// NewProgressTracker 创建进度追踪器
func NewProgressTracker(
	groupStore store.TaskGroupStore,
	taskStore store.TaskStore,
	depManager *DependencyManager,
	config ProgressTrackerConfig,
) *ProgressTracker {
	pt := &ProgressTracker{
		groupStore:    groupStore,
		taskStore:     taskStore,
		depManager:    depManager,
		reports:       make(map[string]*models.TaskProgressReport),
		estimatedDurations: make(map[string]time.Duration),
	}

	// 设置默认预估时间
	if config.DefaultDurations != nil {
		for k, v := range config.DefaultDurations {
			pt.estimatedDurations[k] = v
		}
	} else {
		// 设置默认值
		pt.estimatedDurations = map[string]time.Duration{
			"code":    30 * time.Minute,
			"research": 20 * time.Minute,
			"file":    5 * time.Minute,
			"shell":   10 * time.Minute,
			"web":     15 * time.Minute,
			"review":  15 * time.Minute,
			"deploy":  20 * time.Minute,
		}
	}

	return pt
}

// SetProgressCallback 设置进度回调
func (t *ProgressTracker) SetProgressCallback(callback func(groupID string, report *models.TaskProgressReport)) {
	t.onProgress = callback
}

// GetProgressReport 获取任务组的进度报告
func (t *ProgressTracker) GetProgressReport(groupID string) (*models.TaskProgressReport, error) {
	// 检查缓存
	t.mu.RLock()
	if report, exists := t.reports[groupID]; exists {
		t.mu.RUnlock()
		return report, nil
	}
	t.mu.RUnlock()

	// 生成新报告
	return t.generateReport(groupID)
}

// generateReport 生成进度报告
func (t *ProgressTracker) generateReport(groupID string) (*models.TaskProgressReport, error) {
	// 获取任务组
	group, err := t.groupStore.Get(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task group: %w", err)
	}

	// 获取任务组中的所有任务
	tasks, err := t.depManager.depStore.GetGroupTasks(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group tasks: %w", err)
	}

	report := &models.TaskProgressReport{
		GroupID: groupID,
		Status:  group.Status,
	}

	// 统计各状态任务数量
	for _, task := range tasks {
		switch task.Status {
		case models.TaskStatusCompleted:
			report.CompletedTasks++
		case models.TaskStatusRunning:
			report.RunningTasks++
		case models.TaskStatusPending:
			report.PendingTasks++
		case models.TaskStatusFailed:
			report.FailedTasks++
		}
		report.TotalTasks++

		// 收集任务状态信息
		taskInfo := models.TaskStatusInfo{
			TaskID:   task.ID,
			Title:    task.Title,
			Status:   task.Status,
			Progress: task.Progress,
		}

		// 获取依赖
		deps, _ := t.depManager.depStore.GetByTaskID(task.ID)
		for _, dep := range deps {
			taskInfo.Dependencies = append(taskInfo.Dependencies, dep.DependsOnTaskID)
		}

		// 检查是否可以开始
		canStart, _, _ := t.depManager.CanStartTask(task.ID)
		taskInfo.CanBeStarted = canStart

		report.TaskStatuses = append(report.TaskStatuses, taskInfo)
	}

	// 计算整体进度
	if report.TotalTasks > 0 {
		completedWeight := float64(report.CompletedTasks)
		runningWeight := float64(report.RunningTasks) * 0.5 // 运行中的任务算 50%

		for _, task := range tasks {
			if task.Status == models.TaskStatusRunning {
				runningWeight += float64(task.Progress) / 200.0 // 加上运行中任务的进度
			}
		}

		report.Progress = int((completedWeight + runningWeight) / float64(report.TotalTasks) * 100)
		if report.Progress > 100 {
			report.Progress = 100
		}
	}

	// 计算预估剩余时间
	remainingTime := t.estimateRemainingTime(tasks)
	if remainingTime > 0 {
		seconds := int(remainingTime.Seconds())
		report.EstimatedTimeRemaining = &seconds
	}

	// 更新缓存
	t.mu.Lock()
	t.reports[groupID] = report
	t.mu.Unlock()

	return report, nil
}

// estimateRemainingTime 估算剩余时间
func (t *ProgressTracker) estimateRemainingTime(tasks []models.Task) time.Duration {
	var total time.Duration

	for _, task := range tasks {
		switch task.Status {
		case models.TaskStatusPending:
			// 完整的预估时间
			taskType := task.Type
			if duration, exists := t.estimatedDurations[taskType]; exists {
				total += duration
			} else {
				total += 15 * time.Minute // 默认 15 分钟
			}

		case models.TaskStatusRunning:
			// 剩余的预估时间（按进度）
			taskType := task.Type
			var duration time.Duration
			if d, exists := t.estimatedDurations[taskType]; exists {
				duration = d
			} else {
				duration = 15 * time.Minute
			}
			remaining := float64(duration) * (1 - float64(task.Progress)/100)
			total += time.Duration(remaining)
		}
	}

	return total
}

// UpdateProgress 更新进度
// 当任务状态改变时调用
func (t *ProgressTracker) UpdateProgress(ctx context.Context, groupID string) error {
	// 清除缓存
	t.mu.Lock()
	delete(t.reports, groupID)
	t.mu.Unlock()

	// 生成新报告
	report, err := t.generateReport(groupID)
	if err != nil {
		return err
	}

	// 更新任务组状态
	group, err := t.groupStore.Get(groupID)
	if err != nil {
		return err
	}

	group.Progress = report.Progress
	group.CompletedTasks = report.CompletedTasks
	group.FailedTasks = report.FailedTasks

	// 判断任务组整体状态
	if report.CompletedTasks+report.FailedTasks == report.TotalTasks {
		if report.FailedTasks > 0 {
			group.Status = models.TaskGroupStatusFailed
		} else {
			group.Status = models.TaskGroupStatusCompleted
		}
		now := time.Now()
		group.CompletedAt = &now
	} else if report.RunningTasks > 0 {
		group.Status = models.TaskGroupStatusRunning
	}

	if err := t.groupStore.Update(group); err != nil {
		return err
	}

	// 触发回调
	if t.onProgress != nil {
		t.onProgress(groupID, report)
	}

	return nil
}

// StartMonitoring 开始监控任务组
// 定期更新进度
func (t *ProgressTracker) StartMonitoring(ctx context.Context, groupID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.UpdateProgress(ctx, groupID); err != nil {
				log.Printf("Failed to update progress for group %s: %v", groupID, err)
			}
		}
	}
}

// GetOverallProgress 获取整体进度百分比
func (t *ProgressTracker) GetOverallProgress(groupID string) (int, error) {
	report, err := t.GetProgressReport(groupID)
	if err != nil {
		return 0, err
	}
	return report.Progress, nil
}

// GetTaskExecutionOrder 获取建议的执行顺序
// 综合考虑依赖关系和优先级
func (t *ProgressTracker) GetTaskExecutionOrder(groupID string) ([]string, error) {
	// 获取执行层级
	layers, err := t.depManager.GetExecutionLayers(groupID)
	if err != nil {
		return nil, err
	}

	// 获取任务组中的所有任务
	tasks, err := t.depManager.depStore.GetGroupTasks(groupID)
	if err != nil {
		return nil, err
	}

	// 构建任务 ID -> 任务的映射
	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}

	// 按层级和优先级排序
	var order []string
	for _, layer := range layers {
		// 在每层内按优先级排序
		sortedLayer := t.sortByPriority(layer, taskMap)
		order = append(order, sortedLayer...)
	}

	return order, nil
}

// sortByPriority 按优先级排序
func (t *ProgressTracker) sortByPriority(taskIDs []string, taskMap map[string]*models.Task) []string {
	// 简单的优先级排序
	// high -> medium -> low
	priorityOrder := map[models.Priority]int{
		models.PriorityHigh:   0,
		models.PriorityMedium: 1,
		models.PriorityLow:    2,
	}

	// 冒泡排序
	result := make([]string, len(taskIDs))
	copy(result, taskIDs)

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			taskI, existsI := taskMap[result[i]]
			taskJ, existsJ := taskMap[result[j]]

			if !existsI || !existsJ {
				continue
			}

			if priorityOrder[taskI.Priority] > priorityOrder[taskJ.Priority] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetStatistics 获取统计信息
func (t *ProgressTracker) GetStatistics(groupID string) (map[string]interface{}, error) {
	report, err := t.GetProgressReport(groupID)
	if err != nil {
		return nil, err
	}

	// 获取执行层级
	layers, err := t.depManager.GetExecutionLayers(groupID)
	if err != nil {
		return nil, err
	}

	// 获取关键路径
	criticalPath, err := t.depManager.GetCriticalPath(groupID)
	if err != nil {
		log.Printf("Failed to get critical path: %v", err)
	}

	stats := map[string]interface{}{
		"total_tasks":       report.TotalTasks,
		"completed_tasks":   report.CompletedTasks,
		"running_tasks":     report.RunningTasks,
		"pending_tasks":     report.PendingTasks,
		"failed_tasks":      report.FailedTasks,
		"progress":          report.Progress,
		"execution_layers":  len(layers),
		"critical_path_len": len(criticalPath),
	}

	if report.EstimatedTimeRemaining != nil {
		stats["estimated_remaining_seconds"] = *report.EstimatedTimeRemaining
	}

	return stats, nil
}

// FormatProgress 格式化进度显示
func (t *ProgressTracker) FormatProgress(groupID string) (string, error) {
	report, err := t.GetProgressReport(groupID)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("任务组进度: %d%%\n", report.Progress)
	result += fmt.Sprintf("状态: %s\n", report.Status)
	result += fmt.Sprintf("总任务: %d\n", report.TotalTasks)
	result += fmt.Sprintf("  ✓ 已完成: %d\n", report.CompletedTasks)
	result += fmt.Sprintf("  ◷ 运行中: %d\n", report.RunningTasks)
	result += fmt.Sprintf("  ○ 等待中: %d\n", report.PendingTasks)
	result += fmt.Sprintf("  ✗ 已失败: %d\n", report.FailedTasks)

	if report.EstimatedTimeRemaining != nil {
		duration := time.Duration(*report.EstimatedTimeRemaining) * time.Second
		result += fmt.Sprintf("预估剩余时间: %v\n", duration.Round(time.Minute))
	}

	return result, nil
}