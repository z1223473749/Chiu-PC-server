package deviceapi

import (
	"net/http"
	"strconv"

	"ffmpegserver/API/middleware"
	"ffmpegserver/model"
	"ffmpegserver/public/sql"

	"github.com/gin-gonic/gin"
)

// Handler 设备管理 API 处理器
type Handler struct{}

// NewHandler 创建设备 API 处理器
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册路由
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/devices", h.List)
	r.PUT("/devices/:id", h.Update)
	r.DELETE("/devices/:id", h.Delete)
}

// List 获取当前用户设备列表
func (h *Handler) List(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var devices []model.PcDevice
	sql.Gdb.Where("user_id = ?", userID).Order("last_active DESC").Find(&devices)

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// Update 更新设备（备注名）
func (h *Handler) Update(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var req struct {
		DeviceName string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	result := sql.Gdb.Model(&model.PcDevice{}).Where("id = ? AND user_id = ?", id, userID).Update("device_name", req.DeviceName)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// Delete 删除设备
func (h *Handler) Delete(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 检查该设备是否有运行中的任务
	var runningCount int64
	sql.Gdb.Model(&model.VideoDedupTask{}).Where("pc_code IN (SELECT pc_code FROM pc_devices WHERE id = ?) AND status = ? AND deleted_at = 0", id, model.TaskStatusRunning).Count(&runningCount)
	if runningCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该设备有正在运行的任务，无法删除"})
		return
	}

	result := sql.Gdb.Where("id = ? AND user_id = ?", id, userID).Delete(&model.PcDevice{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
