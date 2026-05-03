package model

const TableNamePcDevice = "pc_devices"

// PcDevice 设备表
type PcDevice struct {
	ID         int64  `gorm:"column:id;type:bigint;primaryKey;autoIncrement:true" json:"id"`
	UserID     int32  `gorm:"column:user_id;type:int;index;not null;comment:所属用户" json:"user_id"`
	PCCode     string `gorm:"column:pc_code;type:varchar(64);uniqueIndex;not null;comment:设备唯一码" json:"pc_code"`
	DeviceName string `gorm:"column:device_name;type:varchar(128);comment:设备备注名" json:"device_name"`
	IP         string `gorm:"column:ip;type:varchar(45);comment:最近IP" json:"ip"`
	IsCurrent  bool   `gorm:"column:is_current;type:tinyint(1);default:0;comment:是否为当前设备" json:"is_current"`
	LastActive int64  `gorm:"column:last_active;type:bigint;comment:最后活跃时间" json:"last_active"`
	CreatedAt  int64  `gorm:"column:created_at;type:bigint;not null;comment:创建时间" json:"created_at"`
}

func (*PcDevice) TableName() string {
	return TableNamePcDevice
}
