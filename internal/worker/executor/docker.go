package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/tp/cowork/internal/shared/models"
)

// DockerExecutor Docker 容器执行器
type DockerExecutor struct {
	config  DockerConfig
	client  *client.Client
	running map[string]*ContainerInfo
	mu      sync.RWMutex
	ctx     context.Context
}

// ContainerInfo 运行中的容器信息
type ContainerInfo struct {
	TaskID      string
	ContainerID string
	WorkDir     string
	StartTime   time.Time
	Status      string
}

// NewDockerExecutor 创建 Docker 执行器
func NewDockerExecutor(cfg DockerConfig) (*DockerExecutor, error) {
	// 创建 Docker 客户端
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// 检查 Docker 是否可用
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker not available: %w", err)
	}

	return &DockerExecutor{
		config:  cfg,
		client:  cli,
		running: make(map[string]*ContainerInfo),
		ctx:     context.Background(),
	}, nil
}

// IsAvailable 检查 Docker 是否可用
func (e *DockerExecutor) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.client.Ping(ctx)
	return err == nil
}

// Execute 在 Docker 容器中执行任务
func (e *DockerExecutor) Execute(task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID:    task.ID,
		StartTime: time.Now(),
	}

	// 创建执行上下文
	ctx, cancel := context.WithTimeout(e.ctx, e.config.Timeout)
	defer cancel()

	// 1. 创建工作目录
	workDir := filepath.Join(e.config.WorkDirBase, task.ID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create work directory: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 记录工作目录创建
	if callback != nil {
		callback.OnProgress(task.ID, 5)
	}

	// 2. 准备输入文件
	if err := e.prepareInputFiles(task, workDir); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to prepare input files: %v", err)
		result.EndTime = time.Now()
		return result
	}

	if callback != nil {
		callback.OnProgress(task.ID, 10)
	}

	// 3. 拉取镜像（如需要）
	imageName := e.getImage(task)
	if err := e.pullImageIfNeeded(ctx, imageName); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to pull image: %v", err)
		result.EndTime = time.Now()
		return result
	}

	if callback != nil {
		callback.OnProgress(task.ID, 20)
	}

	// 4. 创建容器
	containerID, err := e.createContainer(ctx, task, workDir, imageName)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create container: %v", err)
		result.EndTime = time.Now()
		return result
	}

	if callback != nil {
		callback.OnProgress(task.ID, 30)
	}

	// 5. 启动容器
	if err := e.startContainer(ctx, containerID); err != nil {
		e.removeContainer(ctx, containerID)
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to start container: %v", err)
		result.EndTime = time.Now()
		return result
	}

	if callback != nil {
		callback.OnProgress(task.ID, 40)
	}

	// 6. 等待执行完成并收集日志
	exitCode, logs, err := e.waitForContainer(ctx, task.ID, containerID, callback)

	// 7. 收集输出文件
	outputFiles, _ := e.collectOutputFiles(ctx, containerID, workDir)

	// 8. 清理容器
	if e.config.AutoRemove {
		e.removeContainer(ctx, containerID)
	}

	// 9. 设置结果
	result.Output = models.JSON{
		"exit_code":     exitCode,
		"logs":          logs,
		"output_files":  outputFiles,
		"container_id":  containerID,
		"work_dir":      workDir,
		"execution_type": "docker",
	}

	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = err.Error()
	} else if exitCode != 0 {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("container exited with code %d", exitCode)
	} else {
		result.Status = models.TaskStatusCompleted
	}

	result.EndTime = time.Now()
	return result
}

// getImage 获取任务使用的镜像
func (e *DockerExecutor) getImage(task *models.Task) string {
	if img, ok := task.Input["image"].(string); ok && img != "" {
		return img
	}
	return e.config.DefaultImage
}

// getCommand 获取任务执行的命令
func (e *DockerExecutor) getCommand(task *models.Task) []string {
	// 支持多种命令格式
	if cmd, ok := task.Input["command"].(string); ok && cmd != "" {
		return []string{"sh", "-c", cmd}
	}
	if cmds, ok := task.Input["command"].([]interface{}); ok {
		var result []string
		for _, c := range cmds {
			if s, ok := c.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if args, ok := task.Input["args"].([]interface{}); ok {
		var result []string
		for _, a := range args {
			if s, ok := a.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return []string{"sh", "-c", "echo 'No command specified'"}
}

// getEnvironment 获取环境变量
func (e *DockerExecutor) getEnvironment(task *models.Task) []string {
	env := []string{}

	if envMap, ok := task.Input["env"].(map[string]interface{}); ok {
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%v", k, v))
		}
	}

	return env
}

// pullImageIfNeeded 按需拉取镜像
func (e *DockerExecutor) pullImageIfNeeded(ctx context.Context, imageName string) error {
	// 检查拉取策略
	if e.config.PullPolicy == "never" {
		return nil
	}

	// 检查本地是否已有镜像
	if e.config.PullPolicy == "missing" {
		_, _, err := e.client.ImageInspectWithRaw(ctx, imageName)
		if err == nil {
			// 镜像已存在
			return nil
		}
	}

	// 拉取镜像
	reader, err := e.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// 读取拉取进度（忽略输出）
	_, err = io.Copy(io.Discard, reader)
	return err
}

// createContainer 创建容器
func (e *DockerExecutor) createContainer(ctx context.Context, task *models.Task, workDir, imageName string) (string, error) {
	// 构建容器配置
	config := &container.Config{
		Image:      imageName,
		Cmd:        e.getCommand(task),
		WorkingDir: e.config.WorkDir,
		Env:        e.getEnvironment(task),
		Tty:        false,
	}

	// 构建主机配置
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: int64(e.config.CPULimit * 1e9),
			Memory:   parseMemory(e.config.MemoryLimit),
		},
		AutoRemove: false,
		Binds: []string{
			fmt.Sprintf("%s:%s", workDir, e.config.WorkDir),
		},
	}

	// 网络隔离
	if e.config.NetworkDisabled || e.isNetworkDisabled(task) {
		hostConfig.NetworkMode = "none"
	}

	// 创建容器
	resp, err := e.client.ContainerCreate(ctx, config, hostConfig, nil, nil, fmt.Sprintf("cowork-%s", task.ID))
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 注册运行中的容器
	e.mu.Lock()
	e.running[task.ID] = &ContainerInfo{
		TaskID:      task.ID,
		ContainerID: resp.ID,
		WorkDir:     workDir,
		StartTime:   time.Now(),
		Status:      "created",
	}
	e.mu.Unlock()

	return resp.ID, nil
}

// startContainer 启动容器
func (e *DockerExecutor) startContainer(ctx context.Context, containerID string) error {
	if err := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// 更新状态
	e.mu.Lock()
	for _, info := range e.running {
		if info.ContainerID == containerID {
			info.Status = "running"
			break
		}
	}
	e.mu.Unlock()

	return nil
}

// waitForContainer 等待容器执行完成
func (e *DockerExecutor) waitForContainer(ctx context.Context, taskID, containerID string, callback Callback) (int, string, error) {
	// 获取日志流
	logsReader, err := e.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return -1, "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logsReader.Close()

	// 创建输出管道
	stdoutPipe, stdoutWriter := io.Pipe()
	stderrPipe, stderrWriter := io.Pipe()

	// 解析 Docker 日志格式
	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()
		stdcopy.StdCopy(stdoutWriter, stderrWriter, logsReader)
	}()

	// 收集输出
	var outputBuilder strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner := io.MultiReader(stdoutPipe, stderrPipe)
		buf := make([]byte, 1024)
		for {
			n, err := scanner.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outputBuilder.WriteString(chunk)
				if callback != nil {
					callback.OnLog(taskID, "stdout", chunk)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待容器退出
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int
	select {
	case <-ctx.Done():
		// 超时，停止容器
		e.client.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: intPtr(10)})
		return -1, outputBuilder.String(), fmt.Errorf("execution timeout")

	case err := <-errCh:
		return -1, outputBuilder.String(), fmt.Errorf("container error: %w", err)

	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	}

	// 等待日志收集完成
	<-done

	// 更新状态
	e.mu.Lock()
	if info, ok := e.running[taskID]; ok {
		info.Status = "exited"
	}
	e.mu.Unlock()

	return exitCode, outputBuilder.String(), nil
}

// prepareInputFiles 准备输入文件
func (e *DockerExecutor) prepareInputFiles(task *models.Task, workDir string) error {
	inputDir := filepath.Join(workDir, "input")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		return err
	}

	// 处理输入文件
	if inputFiles, ok := task.Input["files"].(map[string]interface{}); ok {
		for name, content := range inputFiles {
			filePath := filepath.Join(inputDir, name)
			switch v := content.(type) {
			case string:
				if err := os.WriteFile(filePath, []byte(v), 0644); err != nil {
					return fmt.Errorf("failed to write input file %s: %w", name, err)
				}
			case []byte:
				if err := os.WriteFile(filePath, v, 0644); err != nil {
					return fmt.Errorf("failed to write input file %s: %w", name, err)
				}
			}
		}
	}

	// 创建输出目录
	outputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	return nil
}

// collectOutputFiles 收集输出文件
func (e *DockerExecutor) collectOutputFiles(ctx context.Context, containerID, workDir string) ([]string, error) {
	outputDir := filepath.Join(workDir, "output")
	var files []string

	// 遍历输出目录
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(workDir, path)
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}

// removeContainer 移除容器
func (e *DockerExecutor) removeContainer(ctx context.Context, containerID string) error {
	return e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

// isNetworkDisabled 检查任务是否禁用网络
func (e *DockerExecutor) isNetworkDisabled(task *models.Task) bool {
	if v, ok := task.Input["network_disabled"].(bool); ok {
		return v
	}
	if v, ok := task.Input["networkDisabled"].(bool); ok {
		return v
	}
	return false
}

// Cancel 取消任务执行
func (e *DockerExecutor) Cancel(taskID string) error {
	e.mu.RLock()
	info, ok := e.running[taskID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task %s not found in running containers", taskID)
	}

	// 停止容器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := e.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: intPtr(5)})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// 更新状态
	e.mu.Lock()
	info.Status = "cancelled"
	e.mu.Unlock()

	return nil
}

// ListRunning 列出运行中的容器
func (e *DockerExecutor) ListRunning() []ContainerInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]ContainerInfo, 0, len(e.running))
	for _, info := range e.running {
		result = append(result, *info)
	}
	return result
}

// Cleanup 清理所有运行中的容器
func (e *DockerExecutor) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ctx := context.Background()
	var lastErr error

	for taskID, info := range e.running {
		if err := e.removeContainer(ctx, info.ContainerID); err != nil {
			lastErr = fmt.Errorf("failed to remove container for task %s: %w", taskID, err)
		}
		delete(e.running, taskID)
	}

	return lastErr
}

// intPtr 返回 int 的指针
func intPtr(n int) *int {
	return &n
}