package ws

import (
	"encoding/json"
	"log"
)

// OnTaskCompleted 任务完成回调（由 scheduler 注册，避免循环导入）
var OnTaskCompleted func(userID int32, taskID int64, outputPath string)

// OnTaskError 任务失败回调（由 scheduler 注册，避免循环导入）
var OnTaskError func(userID int32, taskID int64, errMsg string)

// handleDedupProgress 处理去重任务进度上报
func handleDedupProgress(client *WsClient, payload json.RawMessage) {
	var progress struct {
		TaskID  int64   `json:"task_id"`
		Stage   string  `json:"stage"`
		Percent int     `json:"percent"`
		Frame   int     `json:"frame"`
		Speed   float64 `json:"speed"`
		Log     string  `json:"log,omitempty"`
	}
	if err := json.Unmarshal(payload, &progress); err != nil {
		log.Printf("[WsRouter] dedup_progress 解析失败: %v", err)
		return
	}

	log.Printf("[WsRouter] 进度: task=%d, %d%% (%s)", progress.TaskID, progress.Percent, progress.Stage)

	// 广播进度给该用户其他设备（不广播给发送者 PC）
	GlobalWsHub.PushToAllExceptPC(client.UserID, client.PCCode, "dedup_progress", map[string]interface{}{
		"task_id": progress.TaskID,
		"stage":   progress.Stage,
		"percent": progress.Percent,
		"frame":   progress.Frame,
		"speed":   progress.Speed,
	})
}

// handleDedupComplete 处理去重任务完成
func handleDedupComplete(client *WsClient, payload json.RawMessage) {
	var complete struct {
		TaskID     int64  `json:"task_id"`
		OutputPath string `json:"output_path,omitempty"`
	}
	if err := json.Unmarshal(payload, &complete); err != nil {
		log.Printf("[WsRouter] dedup_complete 解析失败: %v", err)
		return
	}

	log.Printf("[WsRouter] 任务 %d 完成 (PC:%s)", complete.TaskID, client.PCCode)

	// 回调 scheduler 更新 DB + 释放计数
	if OnTaskCompleted != nil {
		OnTaskCompleted(client.UserID, complete.TaskID, complete.OutputPath)
	}

	// 广播给该用户其他设备
	GlobalWsHub.PushToAllExceptPC(client.UserID, client.PCCode, "dedup_complete", map[string]interface{}{
		"task_id": complete.TaskID,
	})
}

// handleDedupError 处理去重任务失败
func handleDedupError(client *WsClient, payload json.RawMessage) {
	var errMsg struct {
		TaskID int64  `json:"task_id"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(payload, &errMsg); err != nil {
		log.Printf("[WsRouter] dedup_error 解析失败: %v", err)
		return
	}

	log.Printf("[WsRouter] 任务 %d 失败: %s (PC:%s)", errMsg.TaskID, errMsg.Error, client.PCCode)

	// 回调 scheduler 更新 DB + 释放计数
	if OnTaskError != nil {
		OnTaskError(client.UserID, errMsg.TaskID, errMsg.Error)
	}

	// 广播给该用户其他设备
	GlobalWsHub.PushToAllExceptPC(client.UserID, client.PCCode, "dedup_error", map[string]interface{}{
		"task_id": errMsg.TaskID,
		"error":   errMsg.Error,
	})
}

// handleDedupLog 处理实时日志
func handleDedupLog(client *WsClient, payload json.RawMessage) {
	var logMsg struct {
		TaskID int64  `json:"task_id"`
		Line   string `json:"line"`
	}
	if err := json.Unmarshal(payload, &logMsg); err != nil {
		return
	}

	// 广播日志给该用户其他设备
	GlobalWsHub.PushToAllExceptPC(client.UserID, client.PCCode, "dedup_log", map[string]interface{}{
		"task_id": logMsg.TaskID,
		"line":    logMsg.Line,
	})
}
