# Docker 容器任务功能实现计划

## 1. 功能需求

根据 `docs/user-scenarios.md` 场景三，Docker 容器任务需要实现以下功能：

### 1.1 核心需求

| 需求 | 描述 |
|------|------|
| **隔离执行** | 在 Docker 容器中安全执行不可信或需要特定环境的任务 |
| **标签匹配** | 用户通过 `required_tags: ["docker", "isolated"]` 指定需要 Docker 环境 |
| **资源限制** | 限制 CPU（1 核）、内存（512MB）等资源 |
| **网络隔离** | 可选禁止网络访问 |
| **结果收集** | 执行完成后从容器收集输出文件 |
| **容器管理** | 任务完成后销毁容器 |

### 1.2 用户操作流程

```
1. 用户提交任务，指定需要 Docker 环境：
   - type: "sandbox-analysis"
   - required_tags: ["docker", "isolated"]
   - input: { "script_url": "..." }

2. 系统自动匹配支持 Docker 的 Worker

3. Worker 为任务创建独立容器：
   - 限制 CPU: 1 核
   - 限制内存: 512MB
   - 禁止网络访问（可选）

4. 任务在容器内执行

5. 执行完成后：
   - 收集输出文件
   - 销毁容器
   - 上报结果

6. 用户下载输出文件
```

### 1.3 数据流转

```
用户 ──提交任务──▶ Coordinator ──标签匹配──▶ Docker Worker
                                              │
                              ┌───────────────┼───────────────┐
                              ▼               ▼               ▼
                        Container A     Container B     Container C
                              │               │               │
                              └───────────────┼───────────────┘
                                              ▼
                                        文件存储 ──▶ Dashboard（下载）
```

---

## 2. 设计方案

根据 `docs/architecture.md`：

### 2.1 Worker 类型

| 类型 | 特点 | 适用场景 |
|------|------|----------|
| **本地 Worker** | 直接在主机运行，无隔离 | 受信任任务、高性能需求 |
| **Docker Worker** | 每任务一个容器，完全隔离 | 不可信代码、多环境需求 |
| **云端 Worker** | 部署在云服务器，可弹性扩展 | 大规模任务、需要 GPU |

### 2.2 计划的文件结构

```
internal/worker/executor/
├── executor.go      // 基础执行器（已实现）
├── docker.go        // Docker 容器执行器（待实现）
├── config.go        // 配置定义
└── interface.go     // 接口定义
```

### 2.3 容器隔离设计

```
任务隔离：

1. 文件系统隔离
   - 每个任务独立工作目录
   - /tmp/cowork/{task_id}/
   - 任务完成后可选择保留或清理

2. 容器隔离（Docker Worker）
   - 每个任务一个容器
   - 限制 CPU/内存
   - 网络隔离（可选）

3. 上下文隔离
   - 任务间不共享状态
   - Agent 会话独立
```

---

## 3. 当前实现分析

### 3.1 文件结构

```
internal/worker/
├── executor/
│   └── executor.go     // 任务执行器（无 Docker 支持）
└── workdir/
    └── workdir.go      // 工作目录管理
```

### 3.2 现有代码分析

**`internal/worker/executor/executor.go`** - 616 行

#### 已实现的能力

| 能力 | 代码位置 | 可复用性 |
|------|----------|----------|
| **工作目录管理** | `PrepareWorkDir()`, `CleanupWorkDir()` | ✅ 可复用 |
| **任务执行框架** | `Execute()` 方法 | ✅ 可复用 |
| **安全检查** | `validateCommand()`, `dangerousCommands` | ⚠️ 需调整 |
| **进度回调** | `Callback` 接口 | ✅ 可复用 |
| **日志收集** | `LogEntry`, `addLog()` | ✅ 可复用 |
| **超时控制** | `context.WithTimeout` | ✅ 可复用 |
| **任务取消** | `Cancel()` 方法 | ⚠️ 需适配 Docker |
| **并发安全** | `sync.RWMutex` | ✅ 可复用 |

#### 安全检查机制

```go
// 当前实现的危险命令黑名单（针对 Shell 命令）
var dangerousCommands = []string{
    "rm -rf /",
    "rm -rf /*",
    "mkfs",
    "dd if=/dev/zero",
    // ...
}

// 安全检查函数
func validateCommand(cmdStr string) error {
    dangerous, reason := isCommandDangerous(cmdStr)
    if dangerous {
        return &SecurityError{Command: cmdStr, Reason: reason}
    }
    return nil
}
```

**结论**：安全检查机制可用于 Docker 场景，但需要在容器内执行时应用，而不是阻止容器创建。

### 3.3 缺失的 Docker 功能

| 功能 | 状态 | 说明 |
|------|------|------|
| Docker 客户端封装 | ❌ 未实现 | 调用 Docker API |
| 容器配置 | ❌ 未实现 | CPU/内存/网络限制 |
| 容器生命周期管理 | ❌ 未实现 | 创建/启动/停止/删除 |
| 文件传输 | ❌ 未实现 | 主机与容器间文件复制 |
| Docker 执行器 | ❌ 未实现 | 实现 Executor 接口 |
| Docker 可用性检测 | ❌ 未实现 | 检测 Docker 是否安装 |

---

## 4. 缺失功能清单

### 4.1 核心功能

| 编号 | 功能 | 描述 | 优先级 |
|------|------|------|--------|
| D1 | Docker 客户端 | 封装 Docker CLI 或 API 调用 | P0 |
| D2 | 容器配置管理 | 定义和应用资源限制 | P0 |
| D3 | 容器生命周期 | 创建、启动、停止、删除容器 | P0 |
| D4 | 文件传输 | 输入文件传入、输出文件传出 | P0 |
| D5 | 执行器实现 | 实现 `docker` 类型任务执行 | P0 |
| D6 | 容器镜像管理 | 拉取、缓存基础镜像 | P1 |
| D7 | 资源监控 | 监控容器资源使用 | P2 |

### 4.2 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `docker.enabled` | bool | false | 是否启用 Docker 执行器 |
| `docker.default_image` | string | "alpine:latest" | 默认容器镜像 |
| `docker.cpu_limit` | float | 1.0 | CPU 限制（核数） |
| `docker.memory_limit` | string | "512m" | 内存限制 |
| `docker.network_disabled` | bool | false | 是否禁用网络 |
| `docker.timeout` | duration | 30m | 容器执行超时 |
| `docker.auto_remove` | bool | true | 任务完成后自动删除容器 |
| `docker.work_dir` | string | "/workspace" | 容器内工作目录 |

### 4.3 接口设计

```go
// DockerExecutor Docker 执行器
type DockerExecutor struct {
    client  *docker.Client
    config  DockerConfig
    running map[string]*ContainerInfo
    mu      sync.RWMutex
}

// ContainerInfo 容器信息
type ContainerInfo struct {
    TaskID       string
    ContainerID  string
    WorkDir      string
    StartTime    time.Time
    Status       string
}

// DockerConfig Docker 配置
type DockerConfig struct {
    DefaultImage    string
    CPULimit        float64
    MemoryLimit     string
    NetworkDisabled bool
    Timeout         time.Duration
    AutoRemove      bool
    WorkDir         string
}
```

---

## 5. 实现计划

### 5.1 阶段划分

| 阶段 | 内容 | 预计工时 |
|------|------|----------|
| 阶段 1 | 基础 Docker 执行器 | 0.5 天 |
| 阶段 2 | 容器配置与资源限制 | 0.5 天 |
| 阶段 3 | 文件传输与结果收集 | 0.5 天 |
| 阶段 4 | 测试与文档 | 0.5 天 |

### 5.2 详细实现步骤

#### 阶段 1：基础 Docker 执行器

**新增文件**：

| 文件 | 说明 |
|------|------|
| `internal/worker/executor/docker.go` | Docker 执行器主实现 |
| `internal/worker/executor/docker_config.go` | Docker 配置定义 |

**修改文件**：

| 文件 | 修改内容 |
|------|----------|
| `internal/worker/executor/executor.go` | 添加 `docker` 类型任务路由 |
| `cmd/worker/main.go` | 添加 Docker 相关命令行参数 |

**代码实现**：

```go
// docker.go
package executor

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    "github.com/docker/go-connections/nat"
)

// DockerExecutor Docker 容器执行器
type DockerExecutor struct {
    config  DockerConfig
    client  *client.Client
    running map[string]*ContainerInfo
    mu      sync.RWMutex
    ctx     context.Context
}

// NewDockerExecutor 创建 Docker 执行器
func NewDockerExecutor(cfg DockerConfig) (*DockerExecutor, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, fmt.Errorf("failed to create docker client: %w", err)
    }

    return &DockerExecutor{
        config:  cfg,
        client:  cli,
        running: make(map[string]*ContainerInfo),
        ctx:     context.Background(),
    }, nil
}

// Execute 在 Docker 容器中执行任务
func (e *DockerExecutor) Execute(task *models.Task, callback Callback) *TaskResult {
    result := &TaskResult{
        TaskID:    task.ID,
        StartTime: time.Now(),
    }

    // 1. 创建工作目录
    workDir := filepath.Join(e.config.WorkDirBase, task.ID)
    if err := os.MkdirAll(workDir, 0755); err != nil {
        result.Status = models.TaskStatusFailed
        result.Error = fmt.Sprintf("failed to create work directory: %w", err)
        return result
    }

    // 2. 准备输入文件
    if err := e.prepareInputFiles(task, workDir); err != nil {
        result.Status = models.TaskStatusFailed
        result.Error = fmt.Sprintf("failed to prepare input files: %w", err)
        return result
    }

    // 3. 创建容器
    containerID, err := e.createContainer(task, workDir)
    if err != nil {
        result.Status = models.TaskStatusFailed
        result.Error = fmt.Sprintf("failed to create container: %w", err)
        return result
    }

    // 4. 启动容器
    if err := e.startContainer(containerID); err != nil {
        e.removeContainer(containerID)
        result.Status = models.TaskStatusFailed
        result.Error = fmt.Sprintf("failed to start container: %w", err)
        return result
    }

    // 5. 等待执行完成
    exitCode, err := e.waitForContainer(task.ID, containerID, callback)

    // 6. 收集输出文件
    e.collectOutputFiles(containerID, workDir)

    // 7. 清理容器
    if e.config.AutoRemove {
        e.removeContainer(containerID)
    }

    // 8. 设置结果
    if err != nil {
        result.Status = models.TaskStatusFailed
        result.Error = err.Error()
    } else if exitCode != 0 {
        result.Status = models.TaskStatusFailed
        result.Error = fmt.Sprintf("container exited with code %d", exitCode)
    } else {
        result.Status = models.TaskStatusCompleted
        result.Output = models.JSON{"exit_code": exitCode}
    }

    result.EndTime = time.Now()
    return result
}
```

#### 阶段 2：容器配置与资源限制

**代码实现**：

```go
// createContainer 创建容器
func (e *DockerExecutor) createContainer(task *models.Task, workDir string) (string, error) {
    // 获取镜像
    image := e.getImage(task)

    // 构建容器配置
    config := &container.Config{
        Image:      image,
        Cmd:        e.getCommand(task),
        WorkingDir: "/workspace",
        Env:        e.getEnvironment(task),
    }

    // 构建主机配置
    hostConfig := &container.HostConfig{
        Resources: container.Resources{
            NanoCPUs: int64(e.config.CPULimit * 1e9),
            Memory:   e.parseMemory(e.config.MemoryLimit),
        },
        AutoRemove: false,
        Binds: []string{
            fmt.Sprintf("%s:/workspace", workDir),
        },
    }

    // 网络隔离
    if e.config.NetworkDisabled || e.isNetworkDisabled(task) {
        hostConfig.NetworkMode = "none"
    }

    // 创建容器
    resp, err := e.client.ContainerCreate(e.ctx, config, hostConfig, nil, nil, "")
    if err != nil {
        return "", err
    }

    // 注册运行中的容器
    e.mu.Lock()
    e.running[task.ID] = &ContainerInfo{
        TaskID:      task.ID,
        ContainerID: resp.ID,
        WorkDir:     workDir,
        StartTime:   time.Now(),
    }
    e.mu.Unlock()

    return resp.ID, nil
}
```

#### 阶段 3：文件传输与结果收集

**代码实现**：

```go
// collectOutputFiles 收集输出文件
func (e *DockerExecutor) collectOutputFiles(containerID, workDir string) error {
    outputDir := filepath.Join(workDir, "output")
    if err := os.MkdirAll(outputDir, 0755); err != nil {
        return err
    }

    // 从容器复制输出文件
    reader, _, err := e.client.CopyFromContainer(
        e.ctx,
        containerID,
        "/workspace/output",
    )
    if err != nil {
        // 输出目录可能不存在，忽略错误
        return nil
    }
    defer reader.Close()

    // 解压 tar 文件到工作目录
    return untar(reader, outputDir)
}

// prepareInputFiles 准备输入文件
func (e *DockerExecutor) prepareInputFiles(task *models.Task, workDir string) error {
    inputDir := filepath.Join(workDir, "input")
    if err := os.MkdirAll(inputDir, 0755); err != nil {
        return err
    }

    // 从任务配置获取输入文件
    if inputFiles, ok := task.Input["files"].(map[string]interface{}); ok {
        for name, content := range inputFiles {
            filePath := filepath.Join(inputDir, name)
            data := []byte(fmt.Sprintf("%v", content))
            if err := os.WriteFile(filePath, data, 0644); err != nil {
                return err
            }
        }
    }

    return nil
}
```

#### 阶段 4：测试与文档

**新增测试文件**：

| 文件 | 说明 |
|------|------|
| `internal/worker/executor/docker_test.go` | Docker 执行器单元测试 |

**测试用例**：

```go
func TestDockerExecutor_Execute(t *testing.T) {
    if os.Getenv("SKIP_DOCKER_TESTS") != "" {
        t.Skip("Skipping Docker tests")
    }

    exec, err := NewDockerExecutor(DefaultDockerConfig())
    require.NoError(t, err)

    task := &models.Task{
        ID:   "test-task-001",
        Type: "docker",
        Input: models.JSON{
            "image":   "alpine:latest",
            "command": "echo hello",
        },
    }

    result := exec.Execute(task, &mockCallback{})
    assert.Equal(t, models.TaskStatusCompleted, result.Status)
}

func TestDockerExecutor_ResourceLimits(t *testing.T) {
    // 测试资源限制
}

func TestDockerExecutor_NetworkIsolation(t *testing.T) {
    // 测试网络隔离
}

func TestDockerExecutor_FileTransfer(t *testing.T) {
    // 测试文件传输
}
```

### 5.3 依赖添加

```go
// go.mod
require (
    github.com/docker/docker v24.0.0+incompatible
    github.com/docker/go-connections v0.4.0
)
```

### 5.4 Worker 启动参数扩展

```go
// cmd/worker/main.go 新增参数
dockerEnabled := flag.Bool("docker", false, "Enable Docker executor")
dockerImage := flag.String("docker-image", "alpine:latest", "Default Docker image")
dockerCPU := flag.Float64("docker-cpu", 1.0, "CPU limit per container")
dockerMemory := flag.String("docker-memory", "512m", "Memory limit per container")
dockerNoNetwork := flag.Bool("docker-no-network", false, "Disable network in containers")
```

---

## 6. 工作量估算

| 阶段 | 任务 | 预计时间 |
|------|------|----------|
| 阶段 1 | Docker 客户端封装、基础执行流程 | 4 小时 |
| 阶段 2 | 容器配置、资源限制、网络隔离 | 4 小时 |
| 阶段 3 | 文件传输、结果收集、错误处理 | 4 小时 |
| 阶段 4 | 单元测试、集成测试、文档 | 4 小时 |
| **总计** | | **16 小时（2 天）** |

---

## 7. 风险与注意事项

### 7.1 安全风险

| 风险 | 缓解措施 |
|------|----------|
| 容器逃逸 | 使用最新 Docker 版本、限制容器权限 |
| 资源耗尽 | 严格限制 CPU/内存、设置并发上限 |
| 镜像安全 | 使用官方镜像、定期扫描漏洞 |

### 7.2 技术风险

| 风险 | 缓解措施 |
|------|----------|
| Docker 不可用 | 启动时检测、优雅降级 |
| 容器启动慢 | 预热镜像池、使用轻量镜像 |
| 网络问题 | 重试机制、超时控制 |

### 7.3 兼容性

- Docker API 版本兼容
- 不同操作系统下的路径处理
- 容器内用户权限问题

---

## 8. 后续扩展

### 8.1 Phase 2 功能

- 镜像缓存管理
- 容器资源监控
- 多容器任务编排
- GPU 支持

### 8.2 Phase 3 功能

- Kubernetes 集成
- 自定义镜像构建
- 容器快照与恢复

---

## 9. 参考资源

- [Docker Engine API](https://docs.docker.com/engine/api/)
- [Docker Go SDK](https://pkg.go.dev/github.com/docker/docker)
- [Container Security](https://docs.docker.com/engine/security/)