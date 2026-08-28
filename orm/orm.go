package orm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/manager"
)

// ProviderSet 是 orm 包的 Wire provider 集合，用于依赖注入
var ProviderSet = wire.NewSet(NewDBManager)

// DBManager 数据库连接池管理器：支持按名管理多个数据库连接（惰性创建 + 并发安全单例）
type DBManager struct {
	mgr *manager.Manager[*sql.DB, config.Database]
}

// NewDBManager 创建并配置数据库连接池管理器
// 接收 *config.Config 作为显式依赖，便于 Wire 进行依赖注入
// 启动时立即创建默认连接，连接失败尽早暴露
func NewDBManager(cfg *config.Config) (*DBManager, error) {
	def := cfg.Database.Default
	if def == "" {
		def = "default"
	}

	m := &DBManager{
		mgr: manager.New[*sql.DB, config.Database](
			def,
			func() map[string]config.Database { return cfg.Database.Connections },
			func(name string, c config.Database) (*sql.DB, error) {
				db, err := GetSqlDb(c.Driver, getDSNFromDatabase(c))
				if err != nil {
					return nil, err
				}
				// 配置连接池参数
				if c.MaxOpenConns > 0 {
					db.SetMaxOpenConns(c.MaxOpenConns)
				}
				if c.MaxIdleConns > 0 {
					db.SetMaxIdleConns(c.MaxIdleConns)
				}
				if c.ConnMaxLifetime > 0 {
					db.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetime) * time.Second)
				}
				if c.ConnMaxIdleTime > 0 {
					db.SetConnMaxIdleTime(time.Duration(c.ConnMaxIdleTime) * time.Second)
				}
				return db, nil
			},
		),
	}

	// 启动即创建默认连接，失败尽早暴露
	if _, err := m.mgr.Get(); err != nil {
		return nil, err
	}
	return m, nil
}

// DB 按名获取数据库连接（无参 = 默认连接），首次访问惰性创建
// 配置了多个 database.connections 时，可用此方法切换不同连接（如读写分离、多库）
func (m *DBManager) DB(name ...string) (*sql.DB, error) {
	return m.mgr.Get(name...)
}

// DefaultName 返回默认连接名
func (m *DBManager) DefaultName() string {
	return m.mgr.Default()
}

// GetDB 获取默认 *sql.DB 实例（兼容旧 API）
func (m *DBManager) GetDB() *sql.DB {
	return m.mgr.MustGet()
}

// Ping 执行数据库健康检查（默认连接）
func (m *DBManager) Ping(ctx context.Context) error {
	db, err := m.mgr.Get()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

// Close 优雅关闭所有已创建的数据库连接
func (m *DBManager) Close() error {
	return m.mgr.Close()
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
	if config.Conf == nil {
		return ""
	}
	dbConfig := GetDatabaseConfig(options, config.Conf)
	return dbConfig.TablePrefix
}
