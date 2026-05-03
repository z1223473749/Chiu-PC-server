package model

const TableNameTaskDailyStat = "task_daily_stats"

// TaskDailyStat 任务日统计表（按用户、日期聚合）
type TaskDailyStat struct {
	ID        int64  `gorm:"column:id;type:bigint;primaryKey;autoIncrement:true" json:"id"`
	UserID    int32  `gorm:"column:user_id;type:int;index:idx_user_date;not null;comment:所属用户" json:"user_id"`
	Date      string `gorm:"column:date;type:varchar(10);index:idx_user_date;not null;comment:统计日期(2026-05-02)" json:"date"`
	Total     int64  `gorm:"column:total;type:bigint;default:0;comment:总任务数" json:"total"`
	Completed int64  `gorm:"column:completed;type:bigint;default:0;comment:完成数" json:"completed"`
	Failed    int64  `gorm:"column:failed;type:bigint;default:0;comment:失败数" json:"failed"`
	Running   int64  `gorm:"column:running;type:bigint;default:0;comment:运行中" json:"running"`
	Waiting   int64  `gorm:"column:waiting;type:bigint;default:0;comment:等待中" json:"waiting"`
	Cancelled int64  `gorm:"column:cancelled;type:bigint;default:0;comment:已取消" json:"cancelled"`
}

func (*TaskDailyStat) TableName() string {
	return TableNameTaskDailyStat
}
