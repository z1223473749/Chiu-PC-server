package model

const TableNameAppUpdate = "app_updates"

// AppUpdate 更新包记录
type AppUpdate struct {
	ID          int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Version     string `gorm:"type:varchar(30);not null" json:"version"`
	UpdateType  string `gorm:"type:varchar(10);not null" json:"update_type"` // full=全量 patch=增量
	FileName    string `gorm:"type:varchar(255);not null" json:"file_name"`
	StorePath   string `gorm:"type:varchar(500);not null" json:"-"` // 服务器磁盘路径，不对外暴露
	FileList    string `gorm:"type:text" json:"file_list"`          // JSON数组：zip内文件相对路径列表
	Size        int64  `gorm:"" json:"size"`
	Checksum    string `gorm:"type:varchar(64)" json:"checksum"` // SHA256
	Description string `gorm:"type:text" json:"description"`
	CreatedBy   int32  `gorm:"" json:"created_by"`
	CreatedAt   int64  `gorm:"autoCreateTime" json:"created_at"`
}

func (*AppUpdate) TableName() string { return TableNameAppUpdate }
