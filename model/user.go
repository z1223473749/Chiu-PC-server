package model

const TableNameUser = "users"

// User 用户模型
type User struct {
	ID           int32  `gorm:"column:id;type:int;primaryKey;autoIncrement:true" json:"id"`
	Account      string `gorm:"column:account;type:varchar(50);not null;uniqueIndex;comment:账号" json:"account"`
	Password     string `gorm:"column:password;type:varchar(255);not null;comment:bcrypt加密密码" json:"-"`
	NickName     string `gorm:"column:nick_name;type:varchar(50);comment:用户昵称" json:"nick_name"`
	Avatar       string `gorm:"column:avatar;type:varchar(255);comment:头像URL" json:"avatar"`
	Role         int32  `gorm:"column:role;type:int;not null;default:0;comment:角色: 0-普通用户 66-管理员 888-超级管理员" json:"role"`
	LoginIP      string `gorm:"column:login_ip;type:varchar(45);comment:最近登录IP" json:"login_ip"`
	LoginTime    int64  `gorm:"column:login_time;type:bigint;comment:最近登录时间" json:"login_time"`
	CreationTime int64  `gorm:"column:creation_time;type:bigint;not null;comment:创建时间" json:"creation_time"`
	UpdateTime   int64  `gorm:"column:update_time;type:bigint;not null;comment:更新时间" json:"update_time"`
}

// TableName 表名
func (*User) TableName() string {
	return TableNameUser
}
