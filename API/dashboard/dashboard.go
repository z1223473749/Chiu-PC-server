package dashboard

import (
	"net/http"
	"time"

	"ffmpegserver/API/middleware"
	"ffmpegserver/model"
	"ffmpegserver/public/sql"

	"github.com/gin-gonic/gin"
)

// Handler 仪表盘 API 处理器
type Handler struct{}

// NewHandler 创建仪表盘 API 处理器
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册路由
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/dashboard", h.Dashboard)
}

// Dashboard 获取仪表盘数据
func (h *Handler) Dashboard(c *gin.Context) {
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

	// 概览
	type Overview struct {
		Total   int64 `json:"total"`
		Running int64 `json:"running"`
		Waiting int64 `json:"waiting"`
		Done    int64 `json:"done"`
		Failed  int64 `json:"failed"`
	}
	var overview Overview
	sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("user_id = ? AND deleted_at = 0", userID).
		Select("COUNT(*) as total, SUM(CASE WHEN status=1 THEN 1 ELSE 0 END) as running, SUM(CASE WHEN status=0 THEN 1 ELSE 0 END) as waiting, SUM(CASE WHEN status=2 THEN 1 ELSE 0 END) as done, SUM(CASE WHEN status=3 THEN 1 ELSE 0 END) as failed").
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
