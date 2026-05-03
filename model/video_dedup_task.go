package model

const TableNameVideoDedupTask = "video_dedup_tasks"

// 任务状态常量
const (
	TaskStatusWaiting   int32 = 0
	TaskStatusRunning   int32 = 1
	TaskStatusDone      int32 = 2
	TaskStatusError     int32 = 3
	TaskStatusCancelled int32 = 4
)

// VideoDedupTask 视频去重任务表
type VideoDedupTask struct {
	ID             int64  `gorm:"column:id;type:bigint;primaryKey;autoIncrement:true" json:"id"`
	UserID         int32  `gorm:"column:user_id;type:int;index;not null;comment:所属用户" json:"user_id"`
	PCCode         string `gorm:"column:pc_code;type:varchar(64);index;comment:目标设备PC码" json:"pc_code"`
	InputFilePath  string `gorm:"column:input_file_path;type:text;not null;comment:输入文件路径" json:"input_file_path"`
	OutputDir      string `gorm:"column:output_dir;type:varchar(512);comment:输出目录" json:"output_dir"`
	OutputPath     string `gorm:"column:output_path;type:varchar(512);comment:实际输出文件路径" json:"output_path"`
	ConfigJSON     string `gorm:"column:config_json;type:text;comment:处理参数JSON(加密前)" json:"-"`
	EncryptedArg   string `gorm:"column:encrypted_arg;type:text;comment:AES加密后的命令参数" json:"-"`
	Status         int32  `gorm:"column:status;type:int;default:0;comment:0等待 1运行 2完成 3错误 4取消" json:"status"`
	Progress       int32  `gorm:"column:progress;type:int;default:0;comment:进度0-100" json:"progress"`
	Stage          string `gorm:"column:stage;type:varchar(64);comment:当前阶段" json:"stage"`
	ConcurrentLock bool   `gorm:"column:concurrent_lock;type:tinyint(1);default:0;comment:是否禁并发" json:"concurrent_lock"`
	ErrorMsg       string `gorm:"column:error_msg;type:text;comment:错误信息" json:"error_msg"`
	DeviceName     string `gorm:"column:device_name;type:varchar(128);comment:设备名称(冗余)" json:"device_name"`
	CreatedAt      int64  `gorm:"column:created_at;type:bigint;not null;comment:创建时间" json:"created_at"`
	UpdatedAt      int64  `gorm:"column:updated_at;type:bigint;not null;comment:更新时间" json:"updated_at"`
	DeletedAt      int64  `gorm:"column:deleted_at;type:bigint;default:0;index;comment:删除时间(软删)" json:"deleted_at"`
}

func (*VideoDedupTask) TableName() string {
	return TableNameVideoDedupTask
}
