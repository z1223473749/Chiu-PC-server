package video_dedup

import (
	"log"
	"sync"
	"time"

	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"ffmpegserver/service/ws"

	"gorm.io/gorm"
)

// scheduler 实例
var (
	scheduler     *Scheduler
	schedulerOnce sync.Once
)

// Scheduler 任务调度器
type Scheduler struct {
	mu      sync.Mutex
	running map[string]int // pcCode → 运行中的任务数
}

// GetScheduler 获取调度器单例
func GetScheduler() *Scheduler {
	schedulerOnce.Do(func() {
		scheduler = &Scheduler{
			running: make(map[string]int),
		}
	})
	return scheduler
}

// StartScheduler 启动调度器循环
func StartScheduler() {
	s := GetScheduler()
	log.Println("[Scheduler] 启动任务调度器")

	// 注册 WS 回调（WS → scheduler 的单向依赖，不产生循环导入）
	ws.OnTaskCompleted = func(userID int32, taskID int64, outputPath string) {
		// 更新 DB 状态
		sql.Gdb.Model(&model.VideoDedupTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
			"status":      model.TaskStatusDone,
			"output_path": outputPath,
			"progress":    100,
			"stage":       "completed",
			"updated_at":  time.Now().Unix(),
		})
		// 释放调度器计数
		s.releaseTask(userID, taskID)
		// 更新日统计
		updateDailyStat(userID, "complete")
	}
	ws.OnTaskError = func(userID int32, taskID int64, errMsg string) {
		// 更新 DB 状态
		sql.Gdb.Model(&model.VideoDedupTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
			"status":     model.TaskStatusError,
			"error_msg":  errMsg,
			"stage":      "error",
			"updated_at": time.Now().Unix(),
		})
		// 释放调度器计数
		s.releaseTask(userID, taskID)
		// 更新日统计
		updateDailyStat(userID, "fail")
	}

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Scheduler] pollOnce panic recovered: %v", r)
					}
				}()
				s.pollOnce()
			}()
		}
	}()
}

// pollOnce 执行一次调度
func (s *Scheduler) pollOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tasks []model.VideoDedupTask
	sql.Gdb.Where("status = ? AND deleted_at = 0", model.TaskStatusWaiting).
		Order("created_at ASC").
		Find(&tasks)

	for _, task := range tasks {
		// 检查设备并发限制
		if !s.canSchedule(task) {
			continue
		}

		// 发送 WS 消息
		msgPayload := map[string]interface{}{
			"task_id":       task.ID,
			"input_file":    task.InputFilePath,
			"output_dir":    task.OutputDir,
			"encrypted_arg": task.EncryptedArg,
			"trf_name":      task.TrfName,
		}

		ws.GlobalWsHub.PushToUserByPC(task.UserID, task.PCCode, "dedup_execute", msgPayload)

		// 更新状态
		now := time.Now().Unix()
		sql.Gdb.Model(&task).Updates(map[string]interface{}{
			"status":     model.TaskStatusRunning,
			"updated_at": now,
		})

		s.running[task.PCCode]++

		log.Printf("[Scheduler] 调度任务 %d → PC:%s", task.ID, task.PCCode)
	}

	// 清理孤儿任务：PC 断线期间卡在 running 状态超过 5 分钟的任务
	s.cleanupOrphanTasks()
}

// canSchedule 检查任务是否可以调度
func (s *Scheduler) canSchedule(task model.VideoDedupTask) bool {
	// 确保 PC 在线才调度
	if !ws.GlobalWsHub.IsPCConnected(task.UserID, task.PCCode) {
		return false
	}

	concurrentLimit := task.ConcurrentLimit
	if concurrentLimit <= 0 {
		concurrentLimit = 2
	}

	runningCount := s.running[task.PCCode]
	if runningCount >= concurrentLimit {
		return false
	}

	return true
}

// releaseTask 释放调度器计数（内部方法，由注册的回调调用）
func (s *Scheduler) releaseTask(userId int32, taskId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var task model.VideoDedupTask
	if err := sql.Gdb.Where("id = ?", taskId).First(&task).Error; err != nil {
		// 即使 DB 查不到也强制扣计数器，避免槽位泄露
		if s.running[""] > 0 {
			s.running[""]--
		}
		return
	}

	s.running[task.PCCode]--
	if s.running[task.PCCode] <= 0 {
		delete(s.running, task.PCCode)
	}
}

// cleanupOrphanTasks 清理孤儿任务（PC 断线期间一直卡在 running 状态的任务）
func (s *Scheduler) cleanupOrphanTasks() {
	var orphans []model.VideoDedupTask
	sql.Gdb.Model(&model.VideoDedupTask{}).
		Where("status = ? AND updated_at < ?", model.TaskStatusRunning, time.Now().Add(-5*time.Minute).Unix()).
		Find(&orphans)

	for _, t := range orphans {
		if !ws.GlobalWsHub.IsPCConnected(t.UserID, t.PCCode) {
			sql.Gdb.Model(&model.VideoDedupTask{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
				"status":     model.TaskStatusError,
				"error_msg":  "PC 断线超时，任务自动取消",
				"updated_at": time.Now().Unix(),
			})
			s.running[t.PCCode]--
			if s.running[t.PCCode] <= 0 {
				delete(s.running, t.PCCode)
			}
			log.Printf("[Scheduler] 清理孤儿任务 %d (PC %s 断线超时)", t.ID, t.PCCode)
		}
	}
}

// updateDailyStat 更新日统计
func updateDailyStat(userId int32, action string) {
	today := time.Now().Format("2006-01-02")

	// 尝试更新已有记录
	result := sql.Gdb.Model(&model.TaskDailyStat{}).
		Where("user_id = ? AND date = ?", userId, today).
		Updates(map[string]interface{}{
			"total":               gorm.Expr("total + 1"),
			getStatColumn(action): gorm.Expr(getStatColumn(action) + " + 1"),
		})

	if result.RowsAffected == 0 {
		// 不存在则创建
		stat := model.TaskDailyStat{
			UserID: userId,
			Date:   today,
			Total:  1,
		}
		switch action {
		case "complete":
			stat.Completed = 1
		case "fail":
			stat.Failed = 1
		case "cancel":
			stat.Cancelled = 1
		default:
			stat.Waiting = 1
		}
		sql.Gdb.Create(&stat)
	}
}

func getStatColumn(action string) string {
	switch action {
	case "complete":
		return "completed"
	case "fail":
		return "failed"
	case "cancel":
		return "cancelled"
	default:
		return "waiting"
	}
}
