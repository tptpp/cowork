package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/errors"
	"github.com/tp/cowork/internal/shared/models"
)

// FileHandler 文件处理器
type FileHandler struct {
	fileStore    store.TaskFileStore
	taskStore    store.TaskStore
	agentStore   store.AgentSessionStore
	uploadDir    string
	maxFileSize  int64 // 最大文件大小 (bytes)
}

// FileHandlerConfig 文件处理器配置
type FileHandlerConfig struct {
	UploadDir   string
	MaxFileSize int64
}

// NewFileHandler 创建文件处理器
func NewFileHandler(
	fileStore store.TaskFileStore,
	taskStore store.TaskStore,
	agentStore store.AgentSessionStore,
	cfg FileHandlerConfig,
) *FileHandler {
	// 设置默认值
	if cfg.UploadDir == "" {
		cfg.UploadDir = "./uploads"
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 100 * 1024 * 1024 // 100MB 默认
	}

	// 确保上传目录存在
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}

	return &FileHandler{
		fileStore:   fileStore,
		taskStore:   taskStore,
		agentStore:  agentStore,
		uploadDir:   cfg.UploadDir,
		maxFileSize: cfg.MaxFileSize,
	}
}

// UploadResponse 文件上传响应
type UploadResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
}

// UploadFile 上传文件 (通用)
// POST /api/files/upload
func (h *FileHandler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		failWithError(c, errors.InvalidRequest("No file provided"))
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.maxFileSize {
		failWithError(c, errors.InvalidRequest(fmt.Sprintf("File too large, max size is %d bytes", h.maxFileSize)))
		return
	}

	// 保存文件
	fileRecord, err := h.saveFile(file, header, "", "")
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to save file", err))
		return
	}

	success(c, UploadResponse{
		ID:       fmt.Sprintf("%d", fileRecord.ID),
		Filename: fileRecord.Name,
		Size:     fileRecord.Size,
		MimeType: fileRecord.MimeType,
		URL:      fmt.Sprintf("/api/files/%d", fileRecord.ID),
	})
}

// UploadTaskFile 上传任务文件
// POST /api/tasks/:id/files
func (h *FileHandler) UploadTaskFile(c *gin.Context) {
	taskID := c.Param("id")

	// 验证任务存在
	_, err := h.taskStore.Get(taskID)
	if err != nil {
		failWithError(c, errors.TaskNotFound(taskID))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		failWithError(c, errors.InvalidRequest("No file provided"))
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.maxFileSize {
		failWithError(c, errors.InvalidRequest(fmt.Sprintf("File too large, max size is %d bytes", h.maxFileSize)))
		return
	}

	// 保存文件
	fileRecord, err := h.saveFile(file, header, taskID, "")
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to save file", err))
		return
	}

	success(c, UploadResponse{
		ID:       fmt.Sprintf("%d", fileRecord.ID),
		Filename: fileRecord.Name,
		Size:     fileRecord.Size,
		MimeType: fileRecord.MimeType,
		URL:      fmt.Sprintf("/api/files/%d", fileRecord.ID),
	})
}

// UploadAgentFile 上传 Agent 会话文件
// POST /api/agent/sessions/:id/files
func (h *FileHandler) UploadAgentFile(c *gin.Context) {
	sessionID := c.Param("id")

	// 验证会话存在
	_, err := h.agentStore.Get(sessionID)
	if err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "Session not found"))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		failWithError(c, errors.InvalidRequest("No file provided"))
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.maxFileSize {
		failWithError(c, errors.InvalidRequest(fmt.Sprintf("File too large, max size is %d bytes", h.maxFileSize)))
		return
	}

	// 保存文件 (agent session 文件不关联 task)
	fileRecord, err := h.saveFile(file, header, "", sessionID)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to save file", err))
		return
	}

	success(c, UploadResponse{
		ID:       fmt.Sprintf("%d", fileRecord.ID),
		Filename: fileRecord.Name,
		Size:     fileRecord.Size,
		MimeType: fileRecord.MimeType,
		URL:      fmt.Sprintf("/api/files/%d", fileRecord.ID),
	})
}

// GetTaskFiles 获取任务文件列表
// GET /api/tasks/:id/files
func (h *FileHandler) GetTaskFiles(c *gin.Context) {
	taskID := c.Param("id")

	// 验证任务存在
	_, err := h.taskStore.Get(taskID)
	if err != nil {
		failWithError(c, errors.TaskNotFound(taskID))
		return
	}

	files, err := h.fileStore.ListByTask(taskID)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get files", err))
		return
	}

	success(c, gin.H{"files": files})
}

// DownloadFile 下载文件
// GET /api/files/:id
func (h *FileHandler) DownloadFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		failWithError(c, errors.InvalidRequest("Invalid file ID"))
		return
	}

	fileRecord, err := h.fileStore.Get(uint(id))
	if err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "File not found"))
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(fileRecord.Path); os.IsNotExist(err) {
		failWithError(c, errors.New(errors.CodeNotFound, "File not found on disk"))
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileRecord.Name))
	c.Header("Content-Type", fileRecord.MimeType)
	c.Header("Content-Length", strconv.FormatInt(fileRecord.Size, 10))

	// 发送文件
	c.File(fileRecord.Path)
}

// DeleteFile 删除文件
// DELETE /api/files/:id
func (h *FileHandler) DeleteFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		failWithError(c, errors.InvalidRequest("Invalid file ID"))
		return
	}

	fileRecord, err := h.fileStore.Get(uint(id))
	if err != nil {
		failWithError(c, errors.New(errors.CodeNotFound, "File not found"))
		return
	}

	// 删除物理文件
	if err := os.Remove(fileRecord.Path); err != nil && !os.IsNotExist(err) {
		// 文件不存在不算错误，继续删除数据库记录
		failWithError(c, errors.WrapInternal("Failed to delete file", err))
		return
	}

	// 删除数据库记录
	if err := h.fileStore.Delete(uint(id)); err != nil {
		failWithError(c, errors.WrapInternal("Failed to delete file record", err))
		return
	}

	success(c, gin.H{"id": idStr, "deleted": true})
}

// saveFile 保存文件到磁盘并创建数据库记录
func (h *FileHandler) saveFile(
	file multipart.File,
	header *multipart.FileHeader,
	taskID string,
	sessionID string,
) (*models.TaskFile, error) {
	// 按日期分目录存储
	now := time.Now()
	dateDir := now.Format("2006/01/02")
	uploadPath := filepath.Join(h.uploadDir, dateDir)

	// 创建目录
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// 生成唯一文件名
	ext := filepath.Ext(header.Filename)
	filename := uuid.New().String() + ext
	filePath := filepath.Join(uploadPath, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// 检测 MIME 类型
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 创建数据库记录
	fileRecord := &models.TaskFile{
		TaskID:   taskID,
		Name:     header.Filename,
		Path:     filePath,
		Size:     size,
		MimeType: mimeType,
	}

	if err := h.fileStore.Create(fileRecord); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return fileRecord, nil
}

// GetFileContent 获取文件内容 (用于 Agent 上下文)
func (h *FileHandler) GetFileContent(id uint) (string, error) {
	fileRecord, err := h.fileStore.Get(id)
	if err != nil {
		return "", err
	}

	// 读取文件内容
	content, err := os.ReadFile(fileRecord.Path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}