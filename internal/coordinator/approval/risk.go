package approval

import "github.com/tp/cowork/internal/shared/models"

// RiskClassification 风险分类规则
var RiskClassification = map[string]models.RiskLevel{
	"read_file":       models.RiskLevelLow,
	"browse_web":      models.RiskLevelLow,
	"run_local_test":  models.RiskLevelLow,
	"list_files":      models.RiskLevelLow,

	"write_file":      models.RiskLevelMedium,
	"edit_file":       models.RiskLevelMedium,
	"create_branch":   models.RiskLevelMedium,
	"execute_shell":   models.RiskLevelMedium,

	"delete_file":     models.RiskLevelHigh,
	"push_git":        models.RiskLevelHigh,
	"deploy":          models.RiskLevelHigh,
}

// DefaultTimeout 默认超时时间（秒）
var DefaultTimeout = map[models.RiskLevel]int{
	models.RiskLevelLow:    0,
	models.RiskLevelMedium: 60,
	models.RiskLevelHigh:   0,
}

// GetRiskLevel 获取操作的风险等级
func GetRiskLevel(action string) models.RiskLevel {
	if level, ok := RiskClassification[action]; ok {
		return level
	}
	return models.RiskLevelMedium
}

// IsHighRiskShell 检查是否是高风险 shell 命令
func IsHighRiskShell(command string) bool {
	highRiskPatterns := []string{"rm", "sudo", "chmod", "chown", "dd", "mkfs"}
	for _, pattern := range highRiskPatterns {
		if containsWord(command, pattern) {
			return true
		}
	}
	return false
}

func containsWord(s, word string) bool {
	for i := 0; i <= len(s)-len(word); i++ {
		if s[i:i+len(word)] == word {
			before := i == 0 || s[i-1] == ' '
			after := i+len(word) == len(s) || s[i+len(word)] == ' '
			if before && after {
				return true
			}
		}
	}
	return false
}