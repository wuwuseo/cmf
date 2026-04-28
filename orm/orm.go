package orm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/wuwuseo/cmf/config"
)

// ProviderSet 是 orm 包的 Wire provider 集合，用于依赖注入
var ProviderSet = wire.NewSet(NewDBManager)

// DBManager 数据库连接池管理器
type DBManager struct {
	db     *sql.DB
	config config.Database
}

// NewDBManager 创建并配置数据库连接池管理器
// 接收 *config.Config 作为显式依赖，便于 Wire 进行依赖注入
func NewDBManager(cfg *config.Config) (*DBManager, error) {
	dbConfig := GetDatabaseConfig(nil, cfg)
	dsn := getDSNFromDatabase(dbConfig)
	db, err := GetSqlDb(dbConfig.Driver, dsn)
	if err != nil {
		return nil, err
	}

	// 配置连接池参数
	if dbConfig.MaxOpenConns > 0 {
		db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	}
	if dbConfig.MaxIdleConns > 0 {
		db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	}
	if dbConfig.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Second)
	}
	if dbConfig.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(dbConfig.ConnMaxIdleTime) * time.Second)
	}

	return &DBManager{
		db:     db,
		config: dbConfig,
	}, nil
}

// GetDB 获取 *sql.DB 实例
func (m *DBManager) GetDB() *sql.DB {
	return m.db
}

// Ping 执行数据库健康检查
func (m *DBManager) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

// Close 优雅关闭数据库连接
func (m *DBManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

func GetSqlDb(driver string, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return db, err
	}
	return db, nil
}

// GetDatabaseSourceDns 根据配置生成数据库连接字符串
// connectionName 参数用于指定要使用的数据库连接名称，默认值为空字符串
func GetDatabaseSourceDns(config *config.Config, connectionName ...string) string {
	// 获取连接名称，如果提供了参数则使用参数值，否则使用配置中的默认值
	dbConfig := GetDatabaseConfig(connectionName, config)

	return getDSNFromDatabase(dbConfig)
}

func getDSNFromDatabase(dbConfig config.Database) string {
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

func GetDatabaseConfig(connectionName []string, config *config.Config) config.Database {
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
	return dbConfig
}

func GetTablePrefix(options ...string) string {
	dbConfig := GetDatabaseConfig(options, config.Conf)
	return dbConfig.TablePrefix
}
