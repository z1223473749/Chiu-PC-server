package video_dedup

import (
	"log"
	"time"

	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"ffmpegserver/utils"
)

// TaskFileInput 单个任务输入
type TaskFileInput struct {
	File            string `json:"file"`
	EncryptedCmd    string `json:"encrypted_cmd"` // @XOR@<base64>
	TrfName         string `json:"trf_name"`      // vidstab_xxx.trf
	ConcurrentLimit int    `json:"concurrent_limit"`
}

// CreateTaskInput 创建任务 API 输入
type CreateTaskInput struct {
	PCCode          string          `json:"pc_code"`
	ConcurrentLimit int             `json:"concurrent_limit"`
	Tasks           []TaskFileInput `json:"tasks"`
	OutputDir       string          `json:"output_dir"`
}

// CreateTaskResult 创建结果
type CreateTaskResult struct {
	Task    *model.VideoDedupTask `json:"task"`
	Warning string                `json:"warning,omitempty"`
}

// CreateTasks 批量创建任务（每个文件一条记录）
func CreateTasks(userID int32, input CreateTaskInput) ([]*model.VideoDedupTask, error) {
	now := time.Now().Unix()
	var results []*model.VideoDedupTask

	// 获取设备名称
	var deviceName string
	var dev model.PcDevice
	if err := sql.Gdb.Where("pc_code = ?", input.PCCode).First(&dev).Error; err == nil {
		deviceName = dev.DeviceName
	}

	for _, t := range input.Tasks {
		// 1. 解密传输加密 → 得到完整 ffmpeg 命令明文
		cmdPlain, err := utils.DecryptTransport(t.EncryptedCmd)
		if err != nil {
			log.Printf("[TaskCreate] 传输解密失败 file=%s: %v", t.File, err)
			continue
		}

		// 2. 用服务端 AES 重新加密 → @ENC@<base64>
		encryptedArg, err := EncryptCommand(string(cmdPlain))
		if err != nil {
			log.Printf("[TaskCreate] AES 加密失败 file=%s: %v", t.File, err)
			continue
		}

		// 3. 创建任务记录
		concurrentLimit := input.ConcurrentLimit
		if concurrentLimit <= 0 {
			concurrentLimit = 1
		}
		task := &model.VideoDedupTask{
			UserID:          userID,
			PCCode:          input.PCCode,
			InputFilePath:   t.File,
			OutputDir:       input.OutputDir,
			EncryptedArg:    encryptedArg,
			TrfName:         t.TrfName,
			ConcurrentLimit: concurrentLimit,
			Status:          model.TaskStatusWaiting,
			DeviceName:      deviceName,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := sql.Gdb.Create(task).Error; err != nil {
			log.Printf("[TaskCreate] 创建任务失败 file=%s: %v", t.File, err)
			continue
		}

		results = append(results, task)
	}

	// 更新设备的并发限制
	if input.ConcurrentLimit > 0 {
		sql.Gdb.Model(&model.PcDevice{}).Where("pc_code = ?", input.PCCode).
			Update("concurrent_limit", input.ConcurrentLimit)
	}
	return results, nil
}
