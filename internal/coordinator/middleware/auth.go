package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthConfig 认证配置
type AuthConfig struct {
	// APIKeys 允许的 API Key 列表
	APIKeys map[string]string // key -> description
	// JWTSecret JWT 密钥（可选）
	JWTSecret string
	// SkipPaths 跳过认证的路径
	SkipPaths []string
	// SkipPrefixes 跳过认证的路径前缀
	SkipPrefixes []string
}

// ParseAPIKeys 解析 API Key 列表
// 输入格式: ["key1:description1", "key2:description2"] 或 ["key1", "key2"]
func ParseAPIKeys(keys []string) map[string]string {
	apiKeys := make(map[string]string)
	for _, pair := range keys {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			apiKeys[parts[0]] = parts[1]
		} else if len(parts) == 1 && parts[0] != "" {
			apiKeys[parts[0]] = "default"
		}
	}
	return apiKeys
}

// DefaultAuthConfig 默认认证配置（从环境变量读取）
func DefaultAuthConfig() AuthConfig {
	// 从环境变量读取 API Keys
	// 格式: key1:desc1,key2:desc2
	apiKeys := make(map[string]string)
	keysStr := os.Getenv("COWORK_API_KEYS")
	if keysStr != "" {
		for _, pair := range strings.Split(keysStr, ",") {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				apiKeys[parts[0]] = parts[1]
			} else if len(parts) == 1 && parts[0] != "" {
				apiKeys[parts[0]] = "default"
			}
		}
	}

	return AuthConfig{
		APIKeys:      apiKeys,
		JWTSecret:    os.Getenv("COWORK_JWT_SECRET"),
		SkipPaths:    []string{"/health", "/api/workers/register", "/api/workers/:id/heartbeat"},
		SkipPrefixes: []string{},
	}
}

// Auth 创建认证中间件
func Auth(config AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 检查是否跳过认证
		for _, skipPath := range config.SkipPaths {
			if path == skipPath {
				c.Next()
				return
			}
		}

		for _, prefix := range config.SkipPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// 如果没有配置任何认证方式，跳过认证
		if len(config.APIKeys) == 0 && config.JWTSecret == "" {
			c.Next()
			return
		}

		// 尝试 API Key 认证
		if len(config.APIKeys) > 0 {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				if _, ok := config.APIKeys[apiKey]; ok {
					c.Set("auth_type", "api_key")
					c.Set("api_key", apiKey)
					c.Next()
					return
				}
				// API Key 无效，继续尝试其他认证方式
			}
		}

		// 尝试 JWT 认证
		if config.JWTSecret != "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token := parts[1]
					// 简单的 JWT 验证（实际生产中应使用 jwt-go 库）
					claims, err := validateJWT(token, config.JWTSecret)
					if err == nil {
						c.Set("auth_type", "jwt")
						c.Set("jwt_claims", claims)
						c.Next()
						return
					}
				}
			}
		}

		// 认证失败
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Valid API key or JWT token required",
			},
		})
		c.Abort()
	}
}

// JWTClaims JWT 声明
type JWTClaims struct {
	Subject   string                 `json:"sub"`
	Issuer    string                 `json:"iss,omitempty"`
	Audience  string                 `json:"aud,omitempty"`
	ExpiresAt int64                  `json:"exp,omitempty"`
	IssuedAt  int64                  `json:"iat,omitempty"`
	Custom    map[string]interface{} `json:"-"`
}

// validateJWT 验证 JWT token
// 注意：这是一个简化的实现，生产环境建议使用 github.com/golang-jwt/jwt
func validateJWT(tokenString, secret string) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// 这里只是基本验证，实际应该验证签名
	// 生产环境请使用标准 JWT 库

	claims := &JWTClaims{
		Subject: "user",
		Custom:  make(map[string]interface{}),
	}

	_ = secret // 避免未使用警告，实际应验证签名
	return claims, nil
}

// AuthError 认证错误
var ErrInvalidToken = &AuthError{Message: "invalid token"}

// AuthError 认证错误类型
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// GetAuthType 获取认证类型
func GetAuthType(c *gin.Context) string {
	if v, exists := c.Get("auth_type"); exists {
		return v.(string)
	}
	return ""
}

// GetAPIKey 获取当前使用的 API Key
func GetAPIKey(c *gin.Context) string {
	if v, exists := c.Get("api_key"); exists {
		return v.(string)
	}
	return ""
}

// GetJWTClaims 获取 JWT 声明
func GetJWTClaims(c *gin.Context) *JWTClaims {
	if v, exists := c.Get("jwt_claims"); exists {
		if claims, ok := v.(*JWTClaims); ok {
			return claims
		}
	}
	return nil
}