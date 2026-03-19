package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS 配置
type CORSConfig struct {
	// AllowedOrigins 允许的域名列表， "*" 表示允许所有
	AllowedOrigins []string
	// AllowedMethods 允许的 HTTP 方法
	AllowedMethods []string
	// AllowedHeaders 允许的请求头
	AllowedHeaders []string
	// ExposedHeaders 暴露给客户端的响应头
	ExposedHeaders []string
	// AllowCredentials 是否允许携带凭证
	AllowCredentials bool
	// MaxAge 预检请求缓存时间（秒）
	MaxAge int
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key", "X-Request-ID"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS 创建 CORS 中间件
func CORS(config CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		// 检查是否允许该来源
		allowedOrigin := ""
		for _, o := range config.AllowedOrigins {
			if o == "*" {
				allowedOrigin = "*"
				break
			}
			// 支持通配符匹配，如 *.example.com
			if strings.HasPrefix(o, "*.") {
				domain := o[2:]
				if strings.HasSuffix(origin, domain) ||
					strings.HasSuffix(origin, "://"+domain) ||
					strings.Contains(origin, "://"+domain+":") {
					allowedOrigin = origin
					break
				}
			}
			// 精确匹配
			if o == origin {
				allowedOrigin = origin
				break
			}
		}

		// 如果没有匹配的来源，使用第一个允许的来源（或拒绝）
		if allowedOrigin == "" {
			if len(config.AllowedOrigins) > 0 {
				// 对于非预检请求，仍然继续处理
				if c.Request.Method != http.MethodOptions {
					c.Next()
					return
				}
				// 对于预检请求，返回第一个允许的来源
				allowedOrigin = config.AllowedOrigins[0]
			} else {
				allowedOrigin = "*"
			}
		}

		// 设置 CORS 头
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))

		if len(config.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
		}

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", string(rune(config.MaxAge)))
		}

		// 处理预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ParseOrigins 解析来源字符串（逗号分隔）
func ParseOrigins(originsStr string) []string {
	if originsStr == "" {
		return []string{"*"}
	}

	origins := strings.Split(originsStr, ",")
	result := make([]string, 0, len(origins))
	for _, o := range origins {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}