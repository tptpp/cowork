package utils

import (
	"os"
	"strings"
)

// ExpandEnvVars 展开字符串中的环境变量
// 支持 $VAR 和 ${VAR} 格式
func ExpandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		// 支持 ${VAR} 格式
		if strings.HasPrefix(key, "{") && strings.HasSuffix(key, "}") {
			key = key[1 : len(key)-1]
		}
		return os.Getenv(key)
	})
}

// ParseStringList 解析逗号分隔的字符串列表
// 自动去除空白和空项
func ParseStringList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// TruncateID 截断 ID 用于日志显示
// 如果 ID 长度小于 maxLen，返回完整 ID
func TruncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen]
}

// TruncateString 截断字符串
// 如果字符串长度小于 maxLen，返回原字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}