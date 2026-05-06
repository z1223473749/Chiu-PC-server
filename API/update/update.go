package updateapi

import (
	"ffmpegserver/API/middleware"
	"ffmpegserver/service/update"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const storageRoot = "public/updates"
const serverBaseURL = "http://localhost:9902"

// Handler 更新包 API 处理器
type Handler struct{}

// NewHandler 创建处理器
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册路由
func (h *Handler) Register(r *gin.RouterGroup) {
	// 所有登录用户可查看列表
	r.GET("/updates", h.List)

	// 仅 role=888 超管可上传/删除
	admin := r.Group("/admin", middleware.RequireRole(888))
	{
		admin.POST("/updates", h.Upload)
		admin.DELETE("/updates/:id", h.Delete)
	}
}

// List 获取更新包列表
// GET /api/updates
func (h *Handler) List(c *gin.Context) {
	list, err := update.ListPackages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 追加 download_url 字段
	type item struct {
		ID          int64  `json:"id"`
		Version     string `json:"version"`
		UpdateType  string `json:"update_type"`
		FileName    string `json:"file_name"`
		FileList    string `json:"file_list"`
		Size        int64  `json:"size"`
		Checksum    string `json:"checksum"`
		Description string `json:"description"`
		CreatedBy   int32  `json:"created_by"`
		CreatedAt   int64  `json:"created_at"`
		DownloadURL string `json:"download_url"`
	}

	result := make([]item, 0, len(list))
	for _, u := range list {
		result = append(result, item{
			ID:          u.ID,
			Version:     u.Version,
			UpdateType:  u.UpdateType,
			FileName:    u.FileName,
			FileList:    u.FileList,
			Size:        u.Size,
			Checksum:    u.Checksum,
			Description: u.Description,
			CreatedBy:   u.CreatedBy,
			CreatedAt:   u.CreatedAt,
			DownloadURL: fmt.Sprintf("%s/updates/%s/%s", serverBaseURL, u.Version, u.FileName),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Upload 上传更新包
// POST /api/admin/updates
// multipart/form-data: file + version + update_type + file_list + description
func (h *Handler) Upload(c *gin.Context) {
	version := c.PostForm("version")
	updateType := c.PostForm("update_type")
	fileList := c.PostForm("file_list")
	description := c.PostForm("description")

	if version == "" || updateType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version 和 update_type 不能为空"})
		return
	}
	if updateType != "full" && updateType != "patch" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "update_type 必须为 full 或 patch"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少上传文件"})
		return
	}

	createdBy := middleware.GetUserIDFromContext(c)

	record, err := update.SavePackage(storageRoot, fileHeader, version, updateType, fileList, description, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":           record.ID,
			"version":      record.Version,
			"update_type":  record.UpdateType,
			"file_name":    record.FileName,
			"file_list":    record.FileList,
			"size":         record.Size,
			"checksum":     record.Checksum,
			"description":  record.Description,
			"created_by":   record.CreatedBy,
			"created_at":   record.CreatedAt,
			"download_url": fmt.Sprintf("%s/updates/%s/%s", serverBaseURL, record.Version, record.FileName),
		},
	})
}

// Delete 删除更新包
// DELETE /api/admin/updates/:id
func (h *Handler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	if err := update.DeletePackage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
