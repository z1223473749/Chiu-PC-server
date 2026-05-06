package update

import (
	"crypto/sha256"
	"encoding/hex"
	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

// SavePackage 将上传的 zip 文件存盘并写入数据库
// storageRoot: 服务器存储根目录，如 "public/updates"
// header:      multipart 文件头
// version:     版本号
// updateType:  "full" 或 "patch"
// fileList:    zip 内文件相对路径 JSON 字符串
// description: 更新说明
// createdBy:   上传者 UserID
func SavePackage(storageRoot string, header *multipart.FileHeader, version, updateType, fileList, description string, createdBy int32) (*model.AppUpdate, error) {
	// 打开上传文件
	src, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	// 读取全部内容（用于 SHA256 和写盘）
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("读取上传内容失败: %w", err)
	}

	// 计算 SHA256
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	// 确保目录存在
	dir := filepath.Join(storageRoot, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 写文件
	fileName := header.Filename
	storePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(storePath, data, 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 写数据库
	record := &model.AppUpdate{
		Version:     version,
		UpdateType:  updateType,
		FileName:    fileName,
		StorePath:   storePath,
		FileList:    fileList,
		Size:        int64(len(data)),
		Checksum:    checksum,
		Description: description,
		CreatedBy:   createdBy,
	}
	if err := sql.Gdb.Create(record).Error; err != nil {
		// 写库失败时回滚磁盘文件
		os.Remove(storePath)
		return nil, fmt.Errorf("数据库写入失败: %w", err)
	}

	return record, nil
}

// ListPackages 返回所有更新包，按 created_at 降序
func ListPackages() ([]model.AppUpdate, error) {
	var list []model.AppUpdate
	if err := sql.Gdb.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("查询更新包列表失败: %w", err)
	}
	return list, nil
}

// DeletePackage 删除数据库记录和磁盘文件
func DeletePackage(id int64) error {
	var record model.AppUpdate
	if err := sql.Gdb.First(&record, id).Error; err != nil {
		return fmt.Errorf("更新包不存在: %w", err)
	}

	// 删磁盘文件
	if err := os.Remove(record.StorePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除磁盘文件失败: %w", err)
	}

	// 删数据库
	if err := sql.Gdb.Delete(&record).Error; err != nil {
		return fmt.Errorf("数据库删除失败: %w", err)
	}

	return nil
}
