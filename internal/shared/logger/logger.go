package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"

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

// zerologHandler 是 slog.Handler 的 zerolog 实现
type zerologHandler struct {
	zl    zerolog.Logger
	mu    sync.Mutex
	attrs []slog.Attr
	level slog.Level
}

// New 创建新的 slog.Logger
func New(level, format string) *slog.Logger {
	// 设置输出
	output := os.Stdout

	// 创建 zerolog logger
	zl := zerolog.New(output).With().Timestamp().Logger()

	// 设置格式
	if format == "text" {
		zl = zl.Output(zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置日志级别
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(&zerologHandler{
		zl:    zl,
		level: lvl,
	})
}

// Init 初始化全局默认 logger
func Init(level, format string) {
	slog.SetDefault(New(level, format))
}

// Configure 使用配置初始化全局 logger
func Configure(cfg Config) {
	Init(cfg.Level, cfg.Format)
}

// Enabled 检查日志级别是否启用
func (h *zerologHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle 处理日志记录
func (h *zerologHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var event *zerolog.Event
	switch r.Level {
	case slog.LevelDebug:
		event = h.zl.Debug()
	case slog.LevelInfo:
		event = h.zl.Info()
	case slog.LevelWarn:
		event = h.zl.Warn()
	case slog.LevelError:
		event = h.zl.Error()
	default:
		event = h.zl.Info()
	}

	// 添加属性
	for _, attr := range h.attrs {
		event = addAttr(event, attr)
	}

	// 添加记录属性
	r.Attrs(func(attr slog.Attr) bool {
		event = addAttr(event, attr)
		return true
	})

	event.Msg(r.Message)
	return nil
}

// WithAttrs 返回带有属性的新 handler
func (h *zerologHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()

	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &zerologHandler{
		zl:    h.zl,
		attrs: newAttrs,
		level: h.level,
	}
}

// WithGroup 返回带有分组的新 handler
func (h *zerologHandler) WithGroup(name string) slog.Handler {
	// 简化实现：直接返回当前 handler
	// 分组功能可以后续增强
	return h
}

// addAttr 将 slog.Attr 添加到 zerolog.Event
func addAttr(event *zerolog.Event, attr slog.Attr) *zerolog.Event {
	if attr.Key == "" {
		return event
	}

	// 处理 slog.Value
	value := attr.Value
	kind := value.Kind()

	switch kind {
	case slog.KindString:
		event.Str(attr.Key, value.String())
	case slog.KindInt64:
		event.Int64(attr.Key, value.Int64())
	case slog.KindUint64:
		event.Uint64(attr.Key, value.Uint64())
	case slog.KindFloat64:
		event.Float64(attr.Key, value.Float64())
	case slog.KindBool:
		event.Bool(attr.Key, value.Bool())
	case slog.KindTime:
		event.Time(attr.Key, value.Time())
	case slog.KindDuration:
		event.Dur(attr.Key, value.Duration())
	case slog.KindGroup:
		// 处理组属性
		groupAttrs := value.Group()
		for _, ga := range groupAttrs {
			event = addAttr(event, ga)
		}
	default:
		// 使用 Any() 处理其他类型
		event.Any(attr.Key, value.Any())
	}

	return event
}

// --- 上下文日志辅助函数 ---

// ctxKey 上下文键类型
type ctxKey struct{}

// loggerKey logger 上下文键
var loggerKey = ctxKey{}

// WithContext 将 logger 添加到上下文
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext 从上下文获取 logger
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
