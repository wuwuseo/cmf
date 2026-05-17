package orm_test

import (
	"testing"

	"github.com/wuwuseo/cmf/config"
	cmform "github.com/wuwuseo/cmf/orm"
)

// ======================== 测试辅助函数 ========================

// newTestConfig 构建一个只有单个默认连接的测试配置
func newTestConfig(driver, host string, port int, user, password, name, tablePrefix string) *config.Config {
	cfg := &config.Config{}
	cfg.Database.Default = "default"
	cfg.Database.Connections = map[string]config.Database{
		"default": {
			Driver:      driver,
			Host:        host,
			Port:        port,
			User:        user,
			Password:    password,
			Name:        name,
			TablePrefix: tablePrefix,
		},
	}
	return cfg
}

// ======================== getDSNFromDatabase MySQL 连接字符串格式 ========================

// TestGetDatabaseSourceDns_MySQL 通过 GetDatabaseSourceDns 间接测试 MySQL DSN 格式
func TestGetDatabaseSourceDns_MySQL(t *testing.T) {
	cfg := newTestConfig("mysql", "127.0.0.1", 3306, "root", "123456", "testdb", "cmf_")
	dsn := cmform.GetDatabaseSourceDns(cfg)

	expected := "root:123456@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != expected {
		t.Errorf("MySQL DSN 格式不正确\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== getDSNFromDatabase PostgreSQL 连接字符串格式 ========================

// TestGetDatabaseSourceDns_PostgreSQL 通过 GetDatabaseSourceDns 间接测试 PostgreSQL DSN 格式
func TestGetDatabaseSourceDns_PostgreSQL(t *testing.T) {
	cfg := newTestConfig("postgres", "localhost", 5432, "postgres", "secret", "mydb", "")
	dsn := cmform.GetDatabaseSourceDns(cfg)

	expected := "user=postgres password=secret host=localhost port=5432 dbname=mydb"
	if dsn != expected {
		t.Errorf("PostgreSQL DSN 格式不正确\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== getDSNFromDatabase SQLite 连接字符串 ========================

// TestGetDatabaseSourceDns_SQLite 通过 GetDatabaseSourceDns 间接测试 SQLite DSN
func TestGetDatabaseSourceDns_SQLite(t *testing.T) {
	cfg := newTestConfig("sqlite3", "/path/to/database.db", 0, "", "", "", "")
	dsn := cmform.GetDatabaseSourceDns(cfg)

	expected := "/path/to/database.db"
	if dsn != expected {
		t.Errorf("SQLite DSN 不正确\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== getDSNFromDatabase 未知驱动返回空字符串 ========================

// TestGetDatabaseSourceDns_UnknownDriver 通过 GetDatabaseSourceDns 间接测试未知驱动返回空字符串
func TestGetDatabaseSourceDns_UnknownDriver(t *testing.T) {
	cfg := newTestConfig("unknown_driver", "localhost", 3306, "root", "pass", "db", "")
	dsn := cmform.GetDatabaseSourceDns(cfg)

	if dsn != "" {
		t.Errorf("未知驱动应返回空字符串，实际: %s", dsn)
	}
}

// ======================== GetDatabaseConfig 使用 connectionName 参数 ========================

// TestGetDatabaseSourceDns_WithConnectionName 通过 GetDatabaseSourceDns 间接测试使用指定连接名
func TestGetDatabaseSourceDns_WithConnectionName(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Default = "default"
	cfg.Database.Connections = map[string]config.Database{
		"default": {
			Driver:   "mysql",
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Name:     "default_db",
		},
		"pg": {
			Driver:   "postgres",
			Host:     "192.168.1.1",
			Port:     5432,
			User:     "admin",
			Password: "admin123",
			Name:     "pg_db",
		},
	}

	dsn := cmform.GetDatabaseSourceDns(cfg, "pg")

	expected := "user=admin password=admin123 host=192.168.1.1 port=5432 dbname=pg_db"
	if dsn != expected {
		t.Errorf("使用连接名称的 DSN 不正确\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== GetDatabaseConfig connectionName 为空时使用默认连接 ========================

// TestGetDatabaseSourceDns_EmptyConnectionName 通过 GetDatabaseSourceDns 间接测试空连接名使用默认连接
func TestGetDatabaseSourceDns_EmptyConnectionName(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Default = "mysql_conn"
	cfg.Database.Connections = map[string]config.Database{
		"mysql_conn": {
			Driver:   "mysql",
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Name:     "my_app_db",
		},
		"other": {
			Driver:   "postgres",
			Host:     "10.0.0.1",
			Port:     5432,
			User:     "pguser",
			Password: "pgpass",
			Name:     "other_db",
		},
	}

	dsn := cmform.GetDatabaseSourceDns(cfg, "")

	// 空连接名应回退到 Default("mysql_conn")，使用 MySQL DSN
	expected := "root:123456@tcp(127.0.0.1:3306)/my_app_db?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != expected {
		t.Errorf("空连接名未使用默认连接\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== GetDatabaseConfig 连接不存在时返回第一个可用连接 ========================

// TestGetDatabaseSourceDns_NonExistentConnection 通过 GetDatabaseSourceDns 间接测试连接不存在时回退
func TestGetDatabaseSourceDns_NonExistentConnection(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Default = "nonexistent"
	cfg.Database.Connections = map[string]config.Database{
		"fallback_db": {
			Driver:   "mysql",
			Host:     "10.0.0.1",
			Port:     3306,
			User:     "user",
			Password: "pass",
			Name:     "fallback_db_name",
		},
	}

	dsn := cmform.GetDatabaseSourceDns(cfg, "missing_connection")

	// 连接不存在，应使用 map 中的第一个（也是唯一的）连接
	expected := "user:pass@tcp(10.0.0.1:3306)/fallback_db_name?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != expected {
		t.Errorf("连接不存在时未回退到可用连接\n期望: %s\n实际: %s", expected, dsn)
	}
}

// ======================== GetTablePrefix 返回正确的表前缀 ========================

// TestGetTablePrefix 测试 GetTablePrefix 返回正确的表前缀
func TestGetTablePrefix(t *testing.T) {
	// 构造配置放入全局 config.Conf，非 nil 时走配置逻辑
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	cfg := &config.Config{}
	cfg.Database.Default = "default"
	cfg.Database.Connections = map[string]config.Database{
		"default": {
			Driver:      "mysql",
			Host:        "localhost",
			Port:        3306,
			User:        "root",
			Password:    "123456",
			Name:        "testdb",
			TablePrefix: "cmf_",
		},
	}
	config.Conf = cfg

	result := cmform.GetTablePrefix()
	if result != "cmf_" {
		t.Errorf("期望表前缀为 'cmf_'，实际: %s", result)
	}
}

// TestGetTablePrefix_WithConnectionName 测试通过连接名称获取指定连接的表前缀
func TestGetTablePrefix_WithConnectionName(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	cfg := &config.Config{}
	cfg.Database.Default = "default"
	cfg.Database.Connections = map[string]config.Database{
		"default": {
			TablePrefix: "cmf_",
		},
		"custom": {
			TablePrefix: "custom_",
		},
	}
	config.Conf = cfg

	result := cmform.GetTablePrefix("custom")
	if result != "custom_" {
		t.Errorf("期望表前缀为 'custom_'，实际: %s", result)
	}
}

// ======================== GetTablePrefix 配置为 nil 时返回空字符串 ========================

// TestGetTablePrefix_NilConfig 测试全局配置为 nil 时 GetTablePrefix 返回空字符串
func TestGetTablePrefix_NilConfig(t *testing.T) {
	oldConf := config.Conf
	config.Conf = nil
	defer func() { config.Conf = oldConf }()

	result := cmform.GetTablePrefix()
	if result != "" {
		t.Errorf("配置为 nil 时应返回空字符串，实际: %s", result)
	}
}
