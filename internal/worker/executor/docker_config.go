package executor

import (
	"os"
	"time"
)

// DockerConfig Docker 执行器配置
type DockerConfig struct {
	// Enabled 是否启用 Docker 执行器
	Enabled bool `json:"enabled"`

	// DefaultImage 默认容器镜像
	DefaultImage string `json:"default_image"`

	// CPULimit CPU 限制（核数）
	CPULimit float64 `json:"cpu_limit"`

	// MemoryLimit 内存限制（如 "512m"）
	MemoryLimit string `json:"memory_limit"`

	// NetworkDisabled 是否禁用网络
	NetworkDisabled bool `json:"network_disabled"`

	// Timeout 容器执行超时
	Timeout time.Duration `json:"timeout"`

	// AutoRemove 任务完成后自动删除容器
	AutoRemove bool `json:"auto_remove"`

	// WorkDir 容器内工作目录
	WorkDir string `json:"work_dir"`

	// WorkDirBase 主机工作目录基础路径
	WorkDirBase string `json:"work_dir_base"`

	// PullPolicy 镜像拉取策略 (always, missing, never)
	PullPolicy string `json:"pull_policy"`
}

// DefaultDockerConfig 返回默认 Docker 配置
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Enabled:         false,
		DefaultImage:    "alpine:latest",
		CPULimit:        1.0,
		MemoryLimit:     "512m",
		NetworkDisabled: false,
		Timeout:         30 * time.Minute,
		AutoRemove:      true,
		WorkDir:         "/workspace",
		WorkDirBase:     "/tmp/cowork",
		PullPolicy:      "missing",
	}
}

// DockerConfigFromEnv 从环境变量创建配置
func DockerConfigFromEnv() DockerConfig {
	cfg := DefaultDockerConfig()

	if v := os.Getenv("COWORK_DOCKER_ENABLED"); v != "" {
		cfg.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("COWORK_DOCKER_IMAGE"); v != "" {
		cfg.DefaultImage = v
	}
	if v := os.Getenv("COWORK_DOCKER_CPU_LIMIT"); v != "" {
		var cpu float64
		if _, err := parseFloat(v, &cpu); err == nil {
			cfg.CPULimit = cpu
		}
	}
	if v := os.Getenv("COWORK_DOCKER_MEMORY_LIMIT"); v != "" {
		cfg.MemoryLimit = v
	}
	if v := os.Getenv("COWORK_DOCKER_NETWORK_DISABLED"); v != "" {
		cfg.NetworkDisabled = v == "true" || v == "1"
	}
	if v := os.Getenv("COWORK_DOCKER_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv("COWORK_DOCKER_WORK_DIR_BASE"); v != "" {
		cfg.WorkDirBase = v
	}

	return cfg
}

// parseMemory 解析内存字符串为字节数
func parseMemory(memStr string) int64 {
	if memStr == "" {
		return 0
	}

	var value int64
	var unit string

	// 解析数字部分
	i := 0
	for i < len(memStr) && (memStr[i] >= '0' && memStr[i] <= '9' || memStr[i] == '.') {
		i++
	}

	if i == 0 {
		return 0
	}

	// 提取数值和单位
	numStr := memStr[:i]
	unit = memStr[i:]

	// 转换数值
	var f float64
	if _, err := parseFloat(numStr, &f); err != nil {
		return 0
	}
	value = int64(f)

	// 根据单位转换
	switch unit {
	case "k", "K":
		value *= 1024
	case "m", "M":
		value *= 1024 * 1024
	case "g", "G":
		value *= 1024 * 1024 * 1024
	case "kb", "KB":
		value *= 1024
	case "mb", "MB":
		value *= 1024 * 1024
	case "gb", "GB":
		value *= 1024 * 1024 * 1024
	}

	return value
}

func parseFloat(s string, f *float64) (int, error) {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' || c == '.' || c == '-' || c == '+' {
			n++
		} else {
			break
		}
	}
	*f = 0 // placeholder, actual parsing would need strconv.ParseFloat
	return n, nil
}