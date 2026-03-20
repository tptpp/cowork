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

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现错误解包
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	e.Details = details
	return e
}

// WithDetail 添加单个详细信息
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
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

// Wrapf 格式化包装错误
func Wrapf(code Code, cause error, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// --- 通用错误 ---

// InvalidRequest 无效请求错误
func InvalidRequest(message string) *AppError {
	return New(CodeInvalidRequest, message)
}

// InvalidRequestf 格式化无效请求错误
func InvalidRequestf(format string, args ...interface{}) *AppError {
	return New(CodeInvalidRequest, fmt.Sprintf(format, args...))
}

// NotFound 未找到错误
func NotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

// NotFoundf 格式化未找到错误
func NotFoundf(format string, args ...interface{}) *AppError {
	return New(CodeNotFound, fmt.Sprintf(format, args...))
}

// InternalError 内部错误
func InternalError(message string) *AppError {
	return New(CodeInternalError, message)
}

// InternalErrorf 格式化内部错误
func InternalErrorf(format string, args ...interface{}) *AppError {
	return New(CodeInternalError, fmt.Sprintf(format, args...))
}

// WrapInternal 包装内部错误
func WrapInternal(message string, cause error) *AppError {
	return Wrap(CodeInternalError, message, cause)
}

// --- 任务相关错误 ---

// TaskNotFound 任务未找到
func TaskNotFound(taskID string) *AppError {
	return New(CodeTaskNotFound, fmt.Sprintf("Task %s not found", taskID))
}

// TaskInvalidState 任务状态无效
func TaskInvalidState(taskID string, currentState, expectedState string) *AppError {
	return New(CodeTaskInvalidState,
		fmt.Sprintf("Task %s is in %s state, expected %s", taskID, currentState, expectedState))
}

// TaskTimeout 任务超时
func TaskTimeout(taskID string) *AppError {
	return New(CodeTaskTimeout, fmt.Sprintf("Task %s timed out", taskID))
}

// --- Worker 相关错误 ---

// WorkerNotFound Worker 未找到
func WorkerNotFound(workerID string) *AppError {
	return New(CodeWorkerNotFound, fmt.Sprintf("Worker %s not found", workerID))
}

// WorkerOffline Worker 离线
func WorkerOffline(workerID string) *AppError {
	return New(CodeWorkerOffline, fmt.Sprintf("Worker %s is offline", workerID))
}

// --- Agent 相关错误 ---

// SessionNotFound 会话未找到
func SessionNotFound(sessionID string) *AppError {
	return New(CodeSessionNotFound, fmt.Sprintf("Session %s not found", sessionID))
}

// ModelNotAvailable 模型不可用
func ModelNotAvailable(model string) *AppError {
	return New(CodeModelNotAvailable, fmt.Sprintf("Model %s is not available", model))
}

// --- 存储相关错误 ---

// DatabaseError 数据库错误
func DatabaseError(operation string, cause error) *AppError {
	return Wrap(CodeDatabaseError, fmt.Sprintf("Database error during %s", operation), cause)
}

// RecordNotFound 记录未找到
func RecordNotFound(table, field, value string) *AppError {
	return New(CodeRecordNotFound, fmt.Sprintf("%s with %s=%s not found", table, field, value))
}

// --- 错误检查 ---

// IsCode 检查错误是否为指定错误码
func IsCode(err error, code Code) bool {
	var appErr *AppError
	if As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// IsNotFound 检查是否为未找到错误
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound) ||
		IsCode(err, CodeTaskNotFound) ||
		IsCode(err, CodeWorkerNotFound) ||
		IsCode(err, CodeSessionNotFound) ||
		IsCode(err, CodeRecordNotFound)
}

// IsInternal 检查是否为内部错误
func IsInternal(err error) bool {
	return IsCode(err, CodeInternalError) || IsCode(err, CodeDatabaseError)
}

// --- 标准库兼容 ---

// As 类似 errors.As
func As(err error, target interface{}) bool {
	for {
		if err == nil {
			return false
		}
		appErr, ok := err.(*AppError)
		if ok {
			switch t := target.(type) {
			case **AppError:
				*t = appErr
				return true
			}
		}
		// 检查是否实现了 Unwrap 接口
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
}

// Is 类似 errors.Is，检查错误链中是否包含目标错误
func Is(err, target error) bool {
	for {
		if err == target {
			return true
		}
		if err == nil {
			return false
		}
		appErr, ok := err.(*AppError)
		if ok && appErr.Cause != nil {
			if appErr.Cause == target {
				return true
			}
			err = appErr.Cause
			continue
		}
		return false
	}
}