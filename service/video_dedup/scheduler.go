package video_dedup

import (
	"log"
	"sync"
	"time"

	"ffmpegserver/config"
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
	locks   map[string]int // pcCode → 禁并发任务数
}

// GetScheduler 获取调度器单例
func GetScheduler() *Scheduler {
	schedulerOnce.Do(func() {
		scheduler = &Scheduler{
			running: make(map[string]int),
			locks:   make(map[string]int),
		}
	})
	return scheduler
}

// StartScheduler 启动调度器循环
func StartScheduler() {
	s := GetScheduler()
	log.Println("[Scheduler] 启动任务调度器")

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			s.pollOnce()
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
		}

		ws.GlobalWsHub.PushToUserByPC(task.UserID, task.PCCode, "dedup_execute", msgPayload)

		// 更新状态
		now := time.Now().Unix()
		sql.Gdb.Model(&task).Updates(map[string]interface{}{
			"status":     model.TaskStatusRunning,
			"updated_at": now,
		})

		s.running[task.PCCode]++
		if task.ConcurrentLock {
			s.locks[task.PCCode]++
		}

		log.Printf("[Scheduler] 调度任务 %d → PC:%s", task.ID, task.PCCode)
	}
}

// canSchedule 检查任务是否可以调度
func (s *Scheduler) canSchedule(task model.VideoDedupTask) bool {
	concurrentLimit := config.Config.Task.DefaultConcurrent
	if concurrentLimit <= 0 {
		concurrentLimit = 2
	}

	runningCount := s.running[task.PCCode]
	lockCount := s.locks[task.PCCode]

	if task.ConcurrentLock {
		// 禁并发任务：同一设备只允许 1 个
		// 假设单 GPU
		if lockCount >= 1 {
			return false
		}
		// 如果已有普通任务在运行，禁并发任务也需等待
		if runningCount > 0 {
			return false
		}
	} else {
		// 如果已有禁并发任务在运行，普通任务需要等
		if lockCount > 0 {
			return false
		}
		if runningCount >= concurrentLimit {
			return false
		}
	}

	return true
}

// OnTaskCompleted 任务完成回调
func OnTaskCompleted(userId int32, taskId int64) {
	s := GetScheduler()
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从 DB 获取任务信息（释放计数）
	var task model.VideoDedupTask
	if err := sql.Gdb.Where("id = ?", taskId).First(&task).Error; err != nil {
		return
	}

	s.running[task.PCCode]--
	if s.running[task.PCCode] <= 0 {
		delete(s.running, task.PCCode)
	}
	if task.ConcurrentLock {
		s.locks[task.PCCode]--
		if s.locks[task.PCCode] <= 0 {
			delete(s.locks, task.PCCode)
		}
	}

	// 更新日统计
	updateDailyStat(userId, "complete")
}

// OnTaskError 任务失败回调
func OnTaskError(userId int32, taskId int64) {
	s := GetScheduler()
	s.mu.Lock()
	defer s.mu.Unlock()

	var task model.VideoDedupTask
	if err := sql.Gdb.Where("id = ?", taskId).First(&task).Error; err != nil {
		return
	}

	s.running[task.PCCode]--
	if s.running[task.PCCode] <= 0 {
		delete(s.running, task.PCCode)
	}
	if task.ConcurrentLock {
		s.locks[task.PCCode]--
		if s.locks[task.PCCode] <= 0 {
			delete(s.locks, task.PCCode)
		}
	}

	updateDailyStat(userId, "fail")
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
