package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Config 日志配置
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output string // stdout, stderr
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
}

var (
	// 全局 logger 实例
	log zerolog.Logger
)

func init() {
	// 初始化默认 logger
	log = New(DefaultConfig())
}

// New 创建新的 logger
func New(cfg Config) zerolog.Logger {
	// 设置输出
	var output io.Writer
	switch cfg.Output {
	case "stderr":
		output = os.Stderr
	default:
		output = os.Stdout
	}

	// 设置格式
	if cfg.Format == "text" {
		// 使用 ConsoleWriter 进行人类可读的输出
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}

	// 设置日志级别
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	return zerolog.New(output).Level(level).With().Timestamp().Logger()
}

// SetLogger 设置全局 logger
func SetLogger(l zerolog.Logger) {
	log = l
}

// Configure 使用配置初始化全局 logger
func Configure(cfg Config) {
	log = New(cfg)
}

// GetLogger 获取全局 logger
func GetLogger() *zerolog.Logger {
	return &log
}

// --- 便捷方法 ---

// Debug 输出 debug 日志
func Debug() *zerolog.Event {
	return log.Debug()
}

// Info 输出 info 日志
func Info() *zerolog.Event {
	return log.Info()
}

// Warn 输出 warn 日志
func Warn() *zerolog.Event {
	return log.Warn()
}

// Error 输出 error 日志
func Error() *zerolog.Event {
	return log.Error()
}

// Fatal 输出 fatal 日志并退出
func Fatal() *zerolog.Event {
	return log.Fatal()
}

// Panic 输出 panic 日志并 panic
func Panic() *zerolog.Event {
	return log.Panic()
}

// With 返回带有字段的 logger
func With() zerolog.Context {
	return log.With()
}

// --- 上下文日志 ---

// Logger 从上下文获取 logger
type Logger struct {
	zl zerolog.Logger
}

// NewLogger 创建新的 Logger
func NewLogger(cfg Config) *Logger {
	return &Logger{zl: New(cfg)}
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{zl: l.zl.With().Interface(key, value).Logger()}
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.zl.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{zl: ctx.Logger()}
}

// Debug 输出 debug 日志
func (l *Logger) Debug() *zerolog.Event {
	return l.zl.Debug()
}

// Info 输出 info 日志
func (l *Logger) Info() *zerolog.Event {
	return l.zl.Info()
}

// Warn 输出 warn 日志
func (l *Logger) Warn() *zerolog.Event {
	return l.zl.Warn()
}

// Error 输出 error 日志
func (l *Logger) Error() *zerolog.Event {
	return l.zl.Error()
}