package device

import (
	"log"
)

// OnDeviceConnected 设备连接回调
func OnDeviceConnected(userId int32, pcCode, ip string) {
	log.Printf("[Device] 设备上线: userId=%d, pcCode=%s, ip=%s", userId, pcCode, ip)
}

// OnTaskStatusChanged 任务状态变更回调（用于设备页展示统计）
func OnTaskStatusChanged(userId int32, pcCode string) {
	// 预留：可更新设备的任务计数
}

// CountRunningTasks 统计指定设备正在运行的任务数
func CountRunningTasks(pcCode string) int {
	// 由调度器维护内存计数
	return 0
}
