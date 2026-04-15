package errors

import (
	"fmt"
	"net/http"
)

// Code 错误码类型
type Code string

// 错误码定义
const (
	// 通用错误码 (1xxx)
	CodeInvalidRequest   Code = "INVALID_REQUEST"
	CodeNotFound         Code = "NOT_FOUND"
	CodeInternalError    Code = "INTERNAL_ERROR"
	CodeConflict         Code = "CONFLICT"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeRateLimited      Code = "RATE_LIMITED"

	// 任务相关错误码 (2xxx)
	CodeTaskNotFound     Code = "TASK_NOT_FOUND"
	CodeTaskInvalidState Code = "TASK_INVALID_STATE"
	CodeTaskTimeout      Code = "TASK_TIMEOUT"
	CodeTaskCancelled    Code = "TASK_CANCELLED"

	// Worker 相关错误码 (3xxx)
	CodeWorkerNotFound   Code = "WORKER_NOT_FOUND"
	CodeWorkerOffline    Code = "WORKER_OFFLINE"
	CodeWorkerBusy       Code = "WORKER_BUSY"

	// Agent 相关错误码 (4xxx)
	CodeSessionNotFound  Code = "SESSION_NOT_FOUND"
	CodeModelNotAvailable Code = "MODEL_NOT_AVAILABLE"
	CodeMessageFailed    Code = "MESSAGE_FAILED"

	// 存储相关错误码 (5xxx)
	CodeDatabaseError    Code = "DATABASE_ERROR"
	CodeRecordNotFound   Code = "RECORD_NOT_FOUND"
	CodeDuplicateEntry   Code = "DUPLICATE_ENTRY"
)

// HTTPStatus 返回错误码对应的 HTTP 状态码
func (c Code) HTTPStatus() int {
	switch c {
	case CodeInvalidRequest, CodeTaskInvalidState:
		return http.StatusBadRequest
	case CodeNotFound, CodeTaskNotFound, CodeWorkerNotFound,
		CodeSessionNotFound, CodeRecordNotFound:
		return http.StatusNotFound
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeConflict, CodeDuplicateEntry:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// AppError 应用错误类型
type AppError struct {
	Code    Code
	Message string
	Cause   error
	Details map[string]interface{}
}

// Error 实现 error 接口 (被调用当 AppError 作为 error 使用)
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现错误解包 (支持 errors.Is/errors.As)
func (e *AppError) Unwrap() error {
	return e.Cause
}

// --- 构造函数 ---

// New 创建新的应用错误
func New(code Code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装错误
func Wrap(code Code, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// --- 通用错误 (实际使用) ---

// InvalidRequest 无效请求错误
func InvalidRequest(message string) *AppError {
	return New(CodeInvalidRequest, message)
}

// WrapInternal 包装内部错误
func WrapInternal(message string, cause error) *AppError {
	return Wrap(CodeInternalError, message, cause)
}

// --- 任务相关错误 (实际使用) ---

// TaskNotFound 任务未找到
func TaskNotFound(taskID string) *AppError {
	return New(CodeTaskNotFound, fmt.Sprintf("Task %s not found", taskID))
}

// --- Worker 相关错误 (实际使用) ---

// WorkerNotFound Worker 未找到
func WorkerNotFound(workerID string) *AppError {
	return New(CodeWorkerNotFound, fmt.Sprintf("Worker %s not found", workerID))
}

// --- Agent 相关错误 (实际使用) ---

// SessionNotFound 会话未找到
func SessionNotFound(sessionID string) *AppError {
	return New(CodeSessionNotFound, fmt.Sprintf("Session %s not found", sessionID))
}