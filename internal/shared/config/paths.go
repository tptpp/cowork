package config

import (
	"os"
	"path/filepath"
)

// AppName 应用名称
const AppName = "cowork"

// GetDefaultBaseDir 返回默认基础目录 ~/.cowork
// 使用用户主目录下的统一结构，避免与 /tmp 下的临时文件冲突
func GetDefaultBaseDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取用户主目录，回退到临时目录
		return filepath.Join(os.TempDir(), AppName)
	}
	return filepath.Join(homeDir, "."+AppName)
}

// GetCoordinatorDir 返回协调者目录 ~/.cowork/coordinator
func GetCoordinatorDir() string {
	return filepath.Join(GetDefaultBaseDir(), "coordinator")
}

// GetCoordinatorDBPath 返回协调者数据库路径 ~/.cowork/coordinator/cowork.db
func GetCoordinatorDBPath() string {
	return filepath.Join(GetCoordinatorDir(), AppName+".db")
}

// GetWorkerDir 返回指定 worker 的目录 ~/.cowork/workers/{name}
func GetWorkerDir(name string) string {
	return filepath.Join(GetDefaultBaseDir(), "workers", name)
}

// GetWorkerWorkspaceDir 返回 worker 工作目录 ~/.cowork/workers/{name}/workspace
func GetWorkerWorkspaceDir(name string) string {
	return filepath.Join(GetWorkerDir(name), "workspace")
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// GetSettingFilePath 返回配置文件路径 ~/.cowork/setting.json
// Worker 从此文件读取 AI 配置等
func GetSettingFilePath() string {
	return filepath.Join(GetDefaultBaseDir(), "setting.json")
}
