package video_dedup

import (
	"encoding/json"
	"log"
	"time"

	"ffmpegserver/model"
	"ffmpegserver/public/sql"
)

// CreateTask 创建去重任务
// 输入参数来自前端 API 请求
type CreateTaskInput struct {
	PCCode    string          `json:"pc_code"`
	Files     []string        `json:"files"`
	OutputDir string          `json:"output_dir"`
	Config    json.RawMessage `json:"config"`
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

	for _, file := range input.Files {
		task := &model.VideoDedupTask{
			UserID:        userID,
			PCCode:        input.PCCode,
			InputFilePath: file,
			OutputDir:     input.OutputDir,
			ConfigJSON:    string(input.Config),
			Status:        model.TaskStatusWaiting,
			DeviceName:    deviceName,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		// 检查是否禁并发
		task.ConcurrentLock = checkConcurrentLock(input.Config)

		// AES 加密命令参数
		encrypted, err := EncryptCommand(string(input.Config))
		if err != nil {
			log.Printf("[TaskCreate] 加密失败 file=%s: %v", file, err)
			// 即使加密失败仍创建任务，可后续补充
		} else {
			task.EncryptedArg = encrypted
		}

		if err := sql.Gdb.Create(task).Error; err != nil {
			log.Printf("[TaskCreate] 创建任务失败 file=%s: %v", file, err)
			// 跳过失败的文件，继续处理其他
			continue
		}

		results = append(results, task)
	}

	return results, nil
}

// checkConcurrentLock 检查配置是否包含禁并发功能
func checkConcurrentLock(configData json.RawMessage) bool {
	var cfg struct {
		SuperRes    bool `json:"superRes"`
		Interpolate bool `json:"interpolate"`
		Stab        bool `json:"stab"`
	}
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return false
	}
	return cfg.SuperRes || cfg.Interpolate || cfg.Stab
}

// GetEncryptedArg 使用 AES-256-CBC 加密配置参数
// 返回 @ENC@<base64> 格式的密文字符串
func GetEncryptedArg(configJSON string) (string, error) {
	return EncryptCommand(configJSON)
}
