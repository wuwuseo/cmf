package orm

import (
	"fmt"

	"github.com/wuwuseo/cmf/config"
)


// GetDatabaseSourceDns 根据配置生成数据库连接字符串
// connectionName 参数用于指定要使用的数据库连接名称，默认值为空字符串
func GetDatabaseSourceDns(config *config.Config, connectionName ...string) string {
	// 获取连接名称，如果提供了参数则使用参数值，否则使用配置中的默认值
	defaultConnection := ""
	if len(connectionName) > 0 && connectionName[0] != "" {
		defaultConnection = connectionName[0]
	} else {
		defaultConnection = config.Database.Default
		if defaultConnection == "" {
			defaultConnection = "default"
		}
	}

	// 获取数据库配置
	dbConfig, exists := config.Database.Connections[defaultConnection]
	if !exists {
		// 如果指定的连接不存在，使用第一个可用的连接
		for _, conn := range config.Database.Connections {
			dbConfig = conn
			break
		}
	}

	switch dbConfig.Driver {
	case "postgres":
		return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s", dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name)
	case "sqlite3":
		return dbConfig.Host
	default:
		return ""
	}
}
