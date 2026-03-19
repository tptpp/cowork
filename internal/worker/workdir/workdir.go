package workdir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 工作目录管理器
type Manager struct {
	baseDir string
	mu      sync.RWMutex
}

// TaskContext 任务上下文信息
type TaskContext struct {
	TaskID      string                 `json:"task_id"`
	Type        string                 `json:"type"`
	CreatedAt   time.Time              `json:"created_at"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  string                 `json:"duration,omitempty"`
}

// NewManager 创建工作目录管理器
func NewManager(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Manager{
		baseDir: baseDir,
	}, nil
}

// CreateTaskDir 创建任务工作目录
func (m *Manager) CreateTaskDir(taskID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskDir := filepath.Join(m.baseDir, taskID)

	// 创建主目录
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create task directory: %w", err)
	}

	// 创建标准子目录
	subDirs := []string{
		"input",   // 输入文件
		"output",  // 输出文件
		"logs",    // 日志文件
		"temp",    // 临时文件
		"artifacts", // 产物文件
	}

	for _, sub := range subDirs {
		path := filepath.Join(taskDir, sub)
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create subdirectory %s: %w", sub, err)
		}
	}

	return taskDir, nil
}

// RemoveTaskDir 删除任务工作目录
func (m *Manager) RemoveTaskDir(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskDir := filepath.Join(m.baseDir, taskID)
	return os.RemoveAll(taskDir)
}

// GetTaskDir 获取任务工作目录路径
func (m *Manager) GetTaskDir(taskID string) string {
	return filepath.Join(m.baseDir, taskID)
}

// WriteContext 写入任务上下文
func (m *Manager) WriteContext(taskID string, ctx *TaskContext) error {
	taskDir := m.GetTaskDir(taskID)
	ctxPath := filepath.Join(taskDir, "context.json")

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := ioutil.WriteFile(ctxPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// ReadContext 读取任务上下文
func (m *Manager) ReadContext(taskID string) (*TaskContext, error) {
	taskDir := m.GetTaskDir(taskID)
	ctxPath := filepath.Join(taskDir, "context.json")

	data, err := ioutil.ReadFile(ctxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	var ctx TaskContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	return &ctx, nil
}

// WriteResult 写入任务结果
func (m *Manager) WriteResult(taskID string, result *TaskResult) error {
	taskDir := m.GetTaskDir(taskID)
	resultPath := filepath.Join(taskDir, "result.json")

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := ioutil.WriteFile(resultPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result file: %w", err)
	}

	return nil
}

// ReadResult 读取任务结果
func (m *Manager) ReadResult(taskID string) (*TaskResult, error) {
	taskDir := m.GetTaskDir(taskID)
	resultPath := filepath.Join(taskDir, "result.json")

	data, err := ioutil.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read result file: %w", err)
	}

	var result TaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// WriteInputFile 写入输入文件
func (m *Manager) WriteInputFile(taskID, filename string, data []byte) error {
	taskDir := m.GetTaskDir(taskID)
	inputPath := filepath.Join(taskDir, "input", filename)

	if err := ioutil.WriteFile(inputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write input file: %w", err)
	}

	return nil
}

// ReadInputFile 读取输入文件
func (m *Manager) ReadInputFile(taskID, filename string) ([]byte, error) {
	taskDir := m.GetTaskDir(taskID)
	inputPath := filepath.Join(taskDir, "input", filename)

	data, err := ioutil.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %w", err)
	}

	return data, nil
}

// WriteOutputFile 写入输出文件
func (m *Manager) WriteOutputFile(taskID, filename string, data []byte) error {
	taskDir := m.GetTaskDir(taskID)
	outputPath := filepath.Join(taskDir, "output", filename)

	if err := ioutil.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// ReadOutputFile 读取输出文件
func (m *Manager) ReadOutputFile(taskID, filename string) ([]byte, error) {
	taskDir := m.GetTaskDir(taskID)
	outputPath := filepath.Join(taskDir, "output", filename)

	data, err := ioutil.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	return data, nil
}

// ListOutputFiles 列出输出文件
func (m *Manager) ListOutputFiles(taskID string) ([]string, error) {
	taskDir := m.GetTaskDir(taskID)
	outputDir := filepath.Join(taskDir, "output")

	entries, err := ioutil.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read output directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// AppendLog 追加日志
func (m *Manager) AppendLog(taskID, level, message string) error {
	taskDir := m.GetTaskDir(taskID)
	logPath := filepath.Join(taskDir, "logs", "task.log")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02T15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	if _, err := f.WriteString(logLine); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}

// ReadLogs 读取日志
func (m *Manager) ReadLogs(taskID string) ([]byte, error) {
	taskDir := m.GetTaskDir(taskID)
	logPath := filepath.Join(taskDir, "logs", "task.log")

	data, err := ioutil.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	return data, nil
}

// GetWorkDirInfo 获取工作目录信息
func (m *Manager) GetWorkDirInfo(taskID string) (map[string]interface{}, error) {
	taskDir := m.GetTaskDir(taskID)

	info := map[string]interface{}{
		"path": taskDir,
	}

	// 获取目录大小
	var size int64
	filepath.Walk(taskDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	info["size"] = size

	// 检查子目录
	subDirs := []string{"input", "output", "logs", "temp", "artifacts"}
	for _, sub := range subDirs {
		path := filepath.Join(taskDir, sub)
		if entries, err := ioutil.ReadDir(path); err == nil {
			info[sub+"_count"] = len(entries)
		}
	}

	return info, nil
}

// CleanupOldDirs 清理旧的工作目录
func (m *Manager) CleanupOldDirs(maxAge time.Duration) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := ioutil.ReadDir(m.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var cleaned []string
	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 检查目录修改时间
		if entry.ModTime().Before(cutoff) {
			path := filepath.Join(m.baseDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				continue // 忽略错误
			}
			cleaned = append(cleaned, entry.Name())
		}
	}

	return cleaned, nil
}