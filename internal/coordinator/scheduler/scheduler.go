package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/tp/cowork/internal/shared/models"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/ws"
)

// Config 调度器配置
type Config struct {
	PollInterval     time.Duration // 轮询间隔
	WorkerTimeout    time.Duration // Worker 超时时间
	MaxRetryAttempts int           // 最大重试次数
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		PollInterval:     2 * time.Second,
		WorkerTimeout:    30 * time.Second,
		MaxRetryAttempts: 3,
	}
}

// Scheduler 任务调度器
type Scheduler struct {
	config      Config
	taskStore   store.TaskStore
	workerStore store.WorkerStore
	hub         *ws.Hub

	// 运行状态
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Worker 状态缓存
	workerLoad map[string]int // worker_id -> current task count
	mu         sync.RWMutex
}

// New 创建新的调度器
func New(cfg Config, taskStore store.TaskStore, workerStore store.WorkerStore, hub *ws.Hub) *Scheduler {
	return &Scheduler{
		config:      cfg,
		taskStore:   taskStore,
		workerStore: workerStore,
		hub:         hub,
		workerLoad:  make(map[string]int),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.wg.Add(1)
	go s.run()

	log.Println("Scheduler started")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	log.Println("Scheduler stopped")
}

// run 主循环
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.schedule()
			s.checkWorkerHealth()
		}
	}
}

// schedule 执行调度
func (s *Scheduler) schedule() {
	// 获取待调度的任务
	pendingTasks, err := s.taskStore.GetByStatus(models.TaskStatusPending)
	if err != nil {
		log.Printf("Failed to get pending tasks: %v", err)
		return
	}

	if len(pendingTasks) == 0 {
		return
	}

	// 获取可用 Worker
	workers, err := s.workerStore.List()
	if err != nil {
		log.Printf("Failed to get workers: %v", err)
		return
	}

	// 过滤在线 Worker
	var onlineWorkers []models.Worker
	for _, w := range workers {
		if w.Status == models.WorkerStatusIdle || w.Status == models.WorkerStatusBusy {
			// 检查心跳
			if time.Since(w.LastSeen) < s.config.WorkerTimeout {
				onlineWorkers = append(onlineWorkers, w)
			}
		}
	}

	if len(onlineWorkers) == 0 {
		return
	}

	// 按优先级排序任务
	s.sortTasksByPriority(pendingTasks)

	// 为每个任务分配 Worker
	for _, task := range pendingTasks {
		if task.WorkerID != nil {
			continue // 已分配
		}

		worker := s.selectWorker(task, onlineWorkers)
		if worker == nil {
			log.Printf("No suitable worker found for task %s (required tags: %v)", task.ID, task.RequiredTags)
			continue
		}

		s.assignTask(&task, worker)
	}
}

// sortTasksByPriority 按优先级排序任务 (使用 sort.Slice，O(n log n))
func (s *Scheduler) sortTasksByPriority(tasks []models.Task) {
	// 优先级权重
	priorityWeight := map[models.Priority]int{
		models.PriorityHigh:   3,
		models.PriorityMedium: 2,
		models.PriorityLow:    1,
	}

	// 使用 sort.Slice 进行排序，时间复杂度 O(n log n)
	sort.Slice(tasks, func(i, j int) bool {
		weightI := priorityWeight[tasks[i].Priority]
		weightJ := priorityWeight[tasks[j].Priority]

		// 优先级高的排前面
		if weightI != weightJ {
			return weightI > weightJ
		}

		// 同优先级按创建时间排序，早的排前面
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
}

// selectWorker 选择合适的 Worker
func (s *Scheduler) selectWorker(task models.Task, workers []models.Worker) *models.Worker {
	var candidates []models.Worker

	// 1. 标签匹配
	for _, worker := range workers {
		if s.matchTags(task.RequiredTags, worker.Tags) {
			// 检查并发限制
			currentLoad := s.getWorkerLoad(worker.ID)
			if currentLoad < worker.MaxConcurrent {
				candidates = append(candidates, worker)
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// 2. 模型匹配（优先）
	if task.PreferredModel != "" {
		for _, worker := range candidates {
			if worker.Model == task.PreferredModel {
				return &worker
			}
		}
	}

	// 3. 负载均衡（选择负载最低的）
	return s.selectLeastLoaded(candidates)
}

// matchTags 检查标签匹配
func (s *Scheduler) matchTags(required []string, available []string) bool {
	if len(required) == 0 {
		return true
	}

	availableMap := make(map[string]bool)
	for _, tag := range available {
		availableMap[tag] = true
	}

	for _, req := range required {
		if !availableMap[req] {
			return false
		}
	}

	return true
}

// selectLeastLoaded 选择负载最低的 Worker
func (s *Scheduler) selectLeastLoaded(workers []models.Worker) *models.Worker {
	if len(workers) == 0 {
		return nil
	}

	leastLoaded := &workers[0]
	minLoad := s.getWorkerLoad(workers[0].ID)

	for i := 1; i < len(workers); i++ {
		load := s.getWorkerLoad(workers[i].ID)
		if load < minLoad {
			minLoad = load
			leastLoaded = &workers[i]
		}
	}

	return leastLoaded
}

// getWorkerLoad 获取 Worker 当前负载
func (s *Scheduler) getWorkerLoad(workerID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workerLoad[workerID]
}

// assignTask 分配任务给 Worker
func (s *Scheduler) assignTask(task *models.Task, worker *models.Worker) {
	now := time.Now()

	// 更新任务状态
	task.WorkerID = &worker.ID
	task.Status = models.TaskStatusRunning
	task.StartedAt = &now

	if err := s.taskStore.Update(task); err != nil {
		log.Printf("Failed to assign task %s to worker %s: %v", task.ID, worker.ID, err)
		return
	}

	// 更新负载计数
	s.mu.Lock()
	s.workerLoad[worker.ID]++
	s.mu.Unlock()

	log.Printf("Task %s assigned to worker %s (%s)", task.ID, worker.ID, worker.Name)

	// 广播任务状态更新
	s.broadcastTaskUpdate(task, "assigned")
}

// broadcastTaskUpdate 广播任务状态更新
func (s *Scheduler) broadcastTaskUpdate(task *models.Task, event string) {
	update := map[string]interface{}{
		"event":     event,
		"task_id":   task.ID,
		"status":    task.Status,
		"worker_id": task.WorkerID,
		"progress":  task.Progress,
		"timestamp": time.Now().Unix(),
	}

	if task.StartedAt != nil {
		update["started_at"] = task.StartedAt
	}

	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal task update: %v", err)
		return
	}

	s.hub.BroadcastToChannel("tasks", data)
	s.hub.BroadcastToChannel("task:"+task.ID, data)
}

// checkWorkerHealth 检查 Worker 健康状态
func (s *Scheduler) checkWorkerHealth() {
	workers, err := s.workerStore.List()
	if err != nil {
		return
	}

	for _, worker := range workers {
		// 检查心跳超时
		if worker.Status != models.WorkerStatusOffline {
			if time.Since(worker.LastSeen) > s.config.WorkerTimeout {
				log.Printf("Worker %s timed out, marking offline", worker.ID)

				// 更新 Worker 状态
				worker.Status = models.WorkerStatusOffline
				s.workerStore.Update(&worker)

				// 处理该 Worker 正在运行的任务
				s.handleWorkerFailure(&worker)

				// 广播 Worker 离线
				s.broadcastWorkerUpdate(&worker, "offline")
			}
		}
	}
}

// handleWorkerFailure 处理 Worker 故障
func (s *Scheduler) handleWorkerFailure(worker *models.Worker) {
	// 获取该 Worker 正在运行的任务
	runningTasks, err := s.taskStore.GetByStatus(models.TaskStatusRunning)
	if err != nil {
		return
	}

	for _, task := range runningTasks {
		if task.WorkerID != nil && *task.WorkerID == worker.ID {
			// 重置任务状态为 pending，等待重新分配
			task.Status = models.TaskStatusPending
			task.WorkerID = nil
			task.StartedAt = nil

			// 记录错误日志
			errMsg := "Worker offline, task rescheduled"
			task.Error = &errMsg

			if err := s.taskStore.Update(&task); err != nil {
				log.Printf("Failed to reset task %s: %v", task.ID, err)
			} else {
				log.Printf("Task %s reset to pending due to worker failure", task.ID)
			}

			// 清除负载计数
			s.mu.Lock()
			delete(s.workerLoad, worker.ID)
			s.mu.Unlock()
		}
	}
}

// broadcastWorkerUpdate 广播 Worker 状态更新
func (s *Scheduler) broadcastWorkerUpdate(worker *models.Worker, event string) {
	update := map[string]interface{}{
		"event":     event,
		"worker_id": worker.ID,
		"name":      worker.Name,
		"status":    worker.Status,
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(update)
	s.hub.BroadcastToChannel("workers", data)
}

// UpdateTaskProgress 更新任务进度
func (s *Scheduler) UpdateTaskProgress(taskID string, progress int, output models.JSON) error {
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return err
	}

	task.Progress = progress
	if output != nil {
		task.Output = output
	}

	if err := s.taskStore.Update(task); err != nil {
		return err
	}

	// 广播进度更新
	s.broadcastTaskUpdate(task, "progress")

	return nil
}

// CompleteTask 完成任务
func (s *Scheduler) CompleteTask(taskID string, output models.JSON) error {
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = models.TaskStatusCompleted
	task.Progress = 100
	task.Output = output
	task.CompletedAt = &now

	if err := s.taskStore.Update(task); err != nil {
		return err
	}

	// 更新 Worker 负载
	if task.WorkerID != nil {
		s.mu.Lock()
		if s.workerLoad[*task.WorkerID] > 0 {
			s.workerLoad[*task.WorkerID]--
		}
		s.mu.Unlock()

		// 更新 Worker 统计
		worker, _ := s.workerStore.Get(*task.WorkerID)
		if worker != nil {
			worker.CompletedTasks++
			s.workerStore.Update(worker)
		}
	}

	log.Printf("Task %s completed", taskID)

	// 广播完成
	s.broadcastTaskUpdate(task, "completed")

	return nil
}

// FailTask 任务失败
func (s *Scheduler) FailTask(taskID string, errMsg string) error {
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = models.TaskStatusFailed
	task.Error = &errMsg
	task.CompletedAt = &now

	if err := s.taskStore.Update(task); err != nil {
		return err
	}

	// 更新 Worker 负载
	if task.WorkerID != nil {
		s.mu.Lock()
		if s.workerLoad[*task.WorkerID] > 0 {
			s.workerLoad[*task.WorkerID]--
		}
		s.mu.Unlock()

		// 更新 Worker 统计
		worker, _ := s.workerStore.Get(*task.WorkerID)
		if worker != nil {
			worker.FailedTasks++
			s.workerStore.Update(worker)
		}
	}

	log.Printf("Task %s failed: %s", taskID, errMsg)

	// 广播失败
	s.broadcastTaskUpdate(task, "failed")

	return nil
}

// GetSchedulerStats 获取调度器统计
func (s *Scheduler) GetSchedulerStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"worker_load": s.workerLoad,
	}
}