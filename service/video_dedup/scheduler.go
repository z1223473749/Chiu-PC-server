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

	// 启动孤儿任务清理协程（1 分钟间隔，独立于调度主循环）
	go func() {
		cleanupTicker := time.NewTicker(1 * time.Minute)
		defer cleanupTicker.Stop()
		for range cleanupTicker.C {
			s.CleanupOrphanTasks()
		}
	}()

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
	// 1. 无锁：查询等待中的任务（最多 1000 条，防 OOM）
	var tasks []model.VideoDedupTask
	sql.Gdb.Where("status = ? AND deleted_at = 0", model.TaskStatusWaiting).
		Order("created_at ASC").
		Limit(1000).
		Find(&tasks)
	if len(tasks) == 0 {
		return
	}

	// 2. 无锁：提取去重后的 PCCode 列表
	pcCodeSet := make(map[string]struct{})
	var pcCodes []string
	for _, task := range tasks {
		if _, ok := pcCodeSet[task.PCCode]; !ok {
			pcCodeSet[task.PCCode] = struct{}{}
			pcCodes = append(pcCodes, task.PCCode)
		}
	}

	// 3. 无锁：只查询有等待任务的设备并发限制
	var devices []model.PcDevice
	sql.Gdb.Select("pc_code, concurrent_limit").
		Where("pc_code IN ?", pcCodes).Find(&devices)
	deviceLimits := make(map[string]int, len(devices))
	for _, d := range devices {
		limit := d.ConcurrentLimit
		if limit <= 0 {
			limit = 1
		}
		deviceLimits[d.PCCode] = limit
	}

	// 4. 锁阶段：调度决策 + DB 状态更新（闭包确保锁在 WS 推送前释放）
	var batch []model.VideoDedupTask
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, task := range tasks {
			if !s.canSchedule(task, deviceLimits) {
				continue
			}
			s.running[task.PCCode]++
			batch = append(batch, task)
		}

		if len(batch) > 0 {
			ids := make([]int64, len(batch))
			for i, t := range batch {
				ids[i] = t.ID
			}
			now := time.Now().Unix()
			sql.Gdb.Model(&model.VideoDedupTask{}).
				Where("id IN ? AND status = ?", ids, model.TaskStatusWaiting).
				Updates(map[string]interface{}{
					"status":     model.TaskStatusRunning,
					"updated_at": now,
				})
		}
	}()
	// 5. 无锁：WS 推送
	for _, task := range batch {
		msgPayload := map[string]interface{}{
			"task_id":       task.ID,
			"input_file":    task.InputFilePath,
			"output_dir":    task.OutputDir,
			"encrypted_arg": task.EncryptedArg,
			"trf_name":      task.TrfName,
		}
		ws.GlobalWsHub.PushToUserByPC(task.UserID, task.PCCode, "dedup_execute", msgPayload)
		log.Printf("[Scheduler] 调度任务 %d → PC:%s", task.ID, task.PCCode)
	}
}

// canSchedule 检查任务是否可以调度
func (s *Scheduler) canSchedule(task model.VideoDedupTask, deviceLimits map[string]int) bool {
	// 确保 PC 在线才调度
	if !ws.GlobalWsHub.IsPCConnected(task.UserID, task.PCCode) {
		return false
	}

	limit := deviceLimits[task.PCCode]
	if limit <= 0 {
		limit = 1
	}

	return s.running[task.PCCode] < limit
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

// CleanupOrphanTasks 清理孤儿任务（PC 断线期间一直卡在 running 状态的任务）
func (s *Scheduler) CleanupOrphanTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

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
