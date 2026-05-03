package sql

import (
	"ffmpegserver/model"
	"fmt"

	"gorm.io/gorm"
)

// allModels 注册所有需要校验的模型
// 新增模型时只需在此处添加即可
var allModels = []interface{}{
	&model.User{},
	&model.VideoDedupTask{},
	&model.PcDevice{},
	&model.TaskDailyStat{},
}

// AutoMigrateDB 启动时自动校验数据库表结构
//  1. 缺失的表 → 创建
//  2. 已有的表缺字段 → 添加
//  3. 已有的表缺索引 → 创建
//  4. 已有的表多余字段 → 删除
//  5. 不在注册列表中的多余表 → 删除
func AutoMigrateDB() {
	fmt.Println("=== 开始数据库结构校验 ===")

	// 临时放宽 sql_mode，避免 CURRENT_TIMESTAMP 等默认值在严格模式下报错
	var originalMode string
	Gdb.Raw("SELECT @@SESSION.sql_mode").Scan(&originalMode)
	Gdb.Exec("SET SESSION sql_mode = ''")
	defer Gdb.Exec("SET SESSION sql_mode = ?", originalMode)

	migrator := Gdb.Migrator()

	for _, m := range allModels {
		stmt := &gorm.Statement{DB: Gdb}
		if err := stmt.Parse(m); err != nil {
			fmt.Printf("[数据库校验] 解析模型 %T 失败: %v\n", m, err)
			continue
		}
		tableName := stmt.Schema.Table

		if !migrator.HasTable(m) {
			// 表不存在 → 整表创建（包含字段和索引）
			if err := migrator.CreateTable(m); err != nil {
				fmt.Printf("[数据库校验] 创建表 %s 失败: %v\n", tableName, err)
			} else {
				fmt.Printf("[数据库校验] 已创建表: %s\n", tableName)
			}
			continue
		}

		// 表已存在 → 补缺失的字段
		for _, field := range stmt.Schema.Fields {
			if field.DBName == "" {
				continue
			}
			if !migrator.HasColumn(m, field.DBName) {
				if err := migrator.AddColumn(m, field.DBName); err != nil {
					fmt.Printf("[数据库校验] 添加字段 %s.%s 失败: %v\n", tableName, field.DBName, err)
				} else {
					fmt.Printf("[数据库校验] 已添加字段: %s.%s\n", tableName, field.DBName)
				}
			}
		}

		// 表已存在 → 补缺失的索引
		for _, idx := range stmt.Schema.ParseIndexes() {
			if !migrator.HasIndex(m, idx.Name) {
				if err := migrator.CreateIndex(m, idx.Name); err != nil {
					fmt.Printf("[数据库校验] 创建索引 %s.%s 失败: %v\n", tableName, idx.Name, err)
				} else {
					fmt.Printf("[数据库校验] 已创建索引: %s.%s\n", tableName, idx.Name)
				}
			}
		}

		// 删除表中不在模型中的多余字段
		modelFields := make(map[string]bool)
		for _, field := range stmt.Schema.Fields {
			if field.DBName != "" {
				modelFields[field.DBName] = true
			}
		}

		// 获取实际表的所有列
		columns, _ := migrator.ColumnTypes(m)
		for _, col := range columns {
			if !modelFields[col.Name()] {
				// 跳过默认字段（如 id）
				if col.Name() == "id" {
					continue
				}
				if err := migrator.DropColumn(m, col.Name()); err != nil {
					fmt.Printf("[数据库校验] 删除多余字段 %s.%s 失败: %v\n", tableName, col.Name(), err)
				} else {
					fmt.Printf("[数据库校验] 已删除多余字段: %s.%s\n", tableName, col.Name())
				}
			}
		}
	}

	// 清理不在注册列表中的多余表
	registeredTables := make(map[string]bool)
	for _, m := range allModels {
		stmt := &gorm.Statement{DB: Gdb}
		stmt.Parse(m)
		registeredTables[stmt.Schema.Table] = true
	}

	tables, _ := migrator.GetTables()
	for _, table := range tables {
		if !registeredTables[table] {
			if err := migrator.DropTable(table); err != nil {
				fmt.Printf("[数据库校验] 删除多余表 %s 失败: %v\n", table, err)
			} else {
				fmt.Printf("[数据库校验] 已删除多余表: %s\n", table)
			}
		}
	}

	fmt.Println("=== 数据库结构校验完成 ===")
}
