package taskapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ffmpegserver/API/middleware"
	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"ffmpegserver/service/video_dedup"

	"github.com/gin-gonic/gin"
)

// Handler 去重任务 API 处理器
type Handler struct{}

// NewHandler 创建处理器
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册路由
func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/tasks", h.Create)
	r.GET("/tasks", h.List)
	r.GET("/tasks/stats", h.Stats)
	r.GET("/tasks/:id", h.Detail)
	r.DELETE("/tasks", h.Delete)
}

// Create 创建任务
func (h *Handler) Create(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var input struct {
		PCCode          string                      `json:"pc_code"`
		ConcurrentLimit int                         `json:"concurrent_limit"`
		Tasks           []video_dedup.TaskFileInput `json:"tasks"`
		OutputDir       string                      `json:"output_dir"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if len(input.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少提供一个任务"})
		return
	}

	tasks, err := video_dedup.CreateTasks(userID, video_dedup.CreateTaskInput{
		PCCode:          input.PCCode,
		ConcurrentLimit: input.ConcurrentLimit,
		Tasks:           input.Tasks,
		OutputDir:       input.OutputDir,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// List 分页查询任务列表
func (h *Handler) List(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	status := c.Query("status")
	pcCode := c.Query("pc_code")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}
	offset := (page - 1) * pageSize

	db := sql.Gdb.Model(&model.VideoDedupTask{}).Where("user_id = ? AND deleted_at = 0", userID)

	if status != "" {
		db = db.Where("status = ?", status)
	}
	if pcCode != "" {
		db = db.Where("pc_code = ?", pcCode)
	}

	var total int64
	db.Count(&total)

	var tasks []model.VideoDedupTask
	db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks)

	c.JSON(http.StatusOK, gin.H{
		"data":      tasks,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Stats 任务统计
func (h *Handler) Stats(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	startDate := c.DefaultQuery("start_date", "")
	endDate := c.DefaultQuery("end_date", "")
	rangeDays := c.DefaultQuery("range", "7d")

	// 计算日期范围
	if startDate == "" {
		switch rangeDays {
		case "30d":
			startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		case "all":
			startDate = "2020-01-01"
		default: // 7d
			startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		}
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	// 当前概览
	var overview struct {
		Total   int64
		Running int64
		Waiting int64
		Done    int64
		Error   int64
	}
	sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("user_id = ? AND deleted_at = 0", userID).
		Select("COUNT(*) as total, SUM(CASE WHEN status=1 THEN 1 ELSE 0 END) as running, SUM(CASE WHEN status=0 THEN 1 ELSE 0 END) as waiting, SUM(CASE WHEN status=2 THEN 1 ELSE 0 END) as done, SUM(CASE WHEN status=3 THEN 1 ELSE 0 END) as error").
		Scan(&overview)

	// 每日统计
	var dailyStats []model.TaskDailyStat
	sql.Gdb.Where("user_id = ? AND date >= ? AND date <= ?", userID, startDate, endDate).
		Order("date ASC").
		Find(&dailyStats)

	// 设备统计
	type DeviceStat struct {
		PCCode     string `json:"pc_code"`
		DeviceName string `json:"device_name"`
		Total      int64  `json:"total"`
		Completed  int64  `json:"completed"`
		Failed     int64  `json:"failed"`
	}
	var deviceStats []DeviceStat
	sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("user_id = ? AND deleted_at = 0", userID).
		Select("pc_code, MAX(device_name) as device_name, COUNT(*) as total, SUM(CASE WHEN status=2 THEN 1 ELSE 0 END) as completed, SUM(CASE WHEN status=3 THEN 1 ELSE 0 END) as failed").
		Group("pc_code").
		Scan(&deviceStats)

	// 最近任务
	var recentTasks []model.VideoDedupTask
	sql.Gdb.Where("user_id = ? AND deleted_at = 0", userID).
		Order("created_at DESC").
		Limit(5).
		Find(&recentTasks)

	c.JSON(http.StatusOK, gin.H{
		"overview":     overview,
		"daily_stats":  dailyStats,
		"device_stats": deviceStats,
		"recent_tasks": recentTasks,
		"start_date":   startDate,
		"end_date":     endDate,
	})
}

// Detail 任务详情
func (h *Handler) Detail(c *gin.Context) {
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

	var task model.VideoDedupTask
	if err := sql.Gdb.Where("id = ? AND user_id = ? AND deleted_at = 0", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// Delete 批量删除任务（软删）
func (h *Handler) Delete(c *gin.Context) {
	userID := middleware.GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req struct {
		TaskIDs []int64 `json:"task_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.TaskIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	// 检查是否有运行中的任务
	var runningCount int64
	sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("id IN ? AND user_id = ? AND status = ? AND deleted_at = 0", req.TaskIDs, userID, model.TaskStatusRunning).
		Count(&runningCount)

	if runningCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("有 %d 个任务正在运行，请先完成任务后再删除", runningCount)})
		return
	}

	now := time.Now().Unix()
	result := sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("id IN ? AND user_id = ? AND deleted_at = 0", req.TaskIDs, userID).
		Update("deleted_at", now)

	if result.Error != nil {
		errMsg := result.Error.Error()
		if errMsg == "" {
			errMsg = "删除失败"
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": result.RowsAffected,
		"message": "删除成功",
	})
}
