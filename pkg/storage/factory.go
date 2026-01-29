package storage

import (
	"fmt"
	"pve-traffic-monitor/pkg/models"
	"strings"
)

// NewStorage 根据配置创建存储实例(工厂函数)
func NewStorageFromConfig(config *models.StorageConfig) (Interface, error) {
	if config == nil {
		return nil, fmt.Errorf("存储配置不能为空")
	}

	storageType := strings.ToLower(config.Type)

	switch storageType {
	case "file", "":
		// 文件存储(默认)
		if config.FilePath == "" {
			return nil, fmt.Errorf("文件存储路径不能为空")
		}
		return NewFileStorage(config.FilePath)

	case "mysql":
		// MySQL 存储
		if config.DSN == "" {
			return nil, fmt.Errorf("MySQL 连接字符串不能为空")
		}
		return NewDatabaseStorage("mysql", config.DSN, config.MaxOpenConns, config.MaxIdleConns, config.ConnMaxLifetime)

	case "postgres", "postgresql":
		// PostgreSQL 存储
		if config.DSN == "" {
			return nil, fmt.Errorf("PostgreSQL 连接字符串不能为空")
		}
		return NewDatabaseStorage("postgres", config.DSN, config.MaxOpenConns, config.MaxIdleConns, config.ConnMaxLifetime)

	case "sqlite", "sqlite3":
		// SQLite 存储
		if config.DSN == "" {
			return nil, fmt.Errorf("SQLite 数据库路径不能为空")
		}
		return NewDatabaseStorage("sqlite3", config.DSN, config.MaxOpenConns, config.MaxIdleConns, config.ConnMaxLifetime)

	default:
		return nil, fmt.Errorf("不支持的存储类型: %s (支持: file, mysql, postgresql, sqlite)", config.Type)
	}
}

// ValidateStorageConfig 验证存储配置
func ValidateStorageConfig(config *models.StorageConfig) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	storageType := strings.ToLower(config.Type)

	switch storageType {
	case "file", "":
		if config.FilePath == "" {
			return fmt.Errorf("文件存储路径不能为空")
		}

	case "mysql", "postgres", "postgresql", "sqlite", "sqlite3":
		if config.DSN == "" {
			return fmt.Errorf("数据库连接字符串不能为空")
		}

	default:
		return fmt.Errorf("不支持的存储类型: %s (支持: file, mysql, postgresql, sqlite)", config.Type)
	}

	return nil
}
