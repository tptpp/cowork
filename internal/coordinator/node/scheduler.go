package node

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// Scheduler 节点调度器
type Scheduler struct {
	registry         *Registry
	nodeStore        store.NodeStore
	taskStore        store.TaskStore
	taskDepStore     store.TaskDependencyStore
}

// NewScheduler 创建节点调度器
func NewScheduler(registry *Registry, nodeStore store.NodeStore, taskStore store.TaskStore, taskDepStore store.TaskDependencyStore) *Scheduler {
	return &Scheduler{
		registry:     registry,
		nodeStore:    nodeStore,
		taskStore:    taskStore,
		taskDepStore: taskDepStore,
	}
}

// AssignTask 分配任务到节点
func (s *Scheduler) AssignTask(ctx context.Context, taskID string, requiredCapabilities []string) (*models.Node, error) {
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	nodes, err := s.nodeStore.ListByCapabilities(requiredCapabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var idleNodes []models.Node
	for _, node := range nodes {
		if node.Status == models.NodeStatusIdle {
			idleNodes = append(idleNodes, node)
		}
	}

	if len(idleNodes) == 0 {
		return nil, fmt.Errorf("no available node with capabilities %v", requiredCapabilities)
	}

	selectedNode := idleNodes[0]

	if err := s.nodeStore.UpdateStatus(selectedNode.ID, models.NodeStatusBusy, &taskID); err != nil {
		return nil, fmt.Errorf("failed to update node status: %w", err)
	}

	task.WorkerID = &selectedNode.ID
	task.Status = models.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	if err := s.taskStore.Update(task); err != nil {
		s.nodeStore.UpdateStatus(selectedNode.ID, models.NodeStatusIdle, nil)
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	log.Printf("Assigned task %s to node %s", taskID, selectedNode.ID)
	return &selectedNode, nil
}

// ReleaseNode 释放节点
func (s *Scheduler) ReleaseNode(ctx context.Context, nodeID string) error {
	return s.nodeStore.UpdateStatus(nodeID, models.NodeStatusIdle, nil)
}

// CompleteTask 完成任务
func (s *Scheduler) CompleteTask(ctx context.Context, taskID string, output models.JSON, taskErr error) error {
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Output = output

	if taskErr != nil {
		task.Status = models.TaskStatusFailed
		errMsg := taskErr.Error()
		task.Error = &errMsg
	} else {
		task.Status = models.TaskStatusCompleted
		task.Progress = 100
	}

	if err := s.taskStore.Update(task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if task.WorkerID != nil {
		if err := s.ReleaseNode(ctx, *task.WorkerID); err != nil {
			log.Printf("Failed to release node %s: %v", *task.WorkerID, err)
		}
	}
	return nil
}

// GetTaskDependencies 获取任务依赖状态
func (s *Scheduler) GetTaskDependencies(ctx context.Context, taskID string) ([]string, bool, error) {
	deps, err := s.taskDepStore.GetByTaskID(taskID)
	if err != nil {
		return nil, false, err
	}

	if len(deps) == 0 {
		return nil, true, nil
	}

	var depIDs []string
	allSatisfied := true
	for _, dep := range deps {
		depIDs = append(depIDs, dep.DependsOnTaskID)
		if !dep.IsSatisfied {
			// Check if the dependency task is completed
			depTask, err := s.taskStore.Get(dep.DependsOnTaskID)
			if err != nil {
				allSatisfied = false
				continue
			}
			if depTask.Status != models.TaskStatusCompleted {
				allSatisfied = false
			} else {
				// Mark as satisfied if the task is completed
				s.taskDepStore.MarkSatisfied(dep.ID)
			}
		}
	}
	return depIDs, allSatisfied, nil
}

// GetReadyTasks 获取可以开始的任务
func (s *Scheduler) GetReadyTasks(ctx context.Context) ([]models.Task, error) {
	tasks, err := s.taskStore.GetByStatus(models.TaskStatusPending)
	if err != nil {
		return nil, err
	}

	var readyTasks []models.Task
	for _, task := range tasks {
		_, satisfied, err := s.GetTaskDependencies(ctx, task.ID)
		if err != nil {
			continue
		}
		if satisfied {
			readyTasks = append(readyTasks, task)
		}
	}
	return readyTasks, nil
}