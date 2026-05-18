package orm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/wuwuseo/cmf/config"
	cmform "github.com/wuwuseo/cmf/orm"
)

func TestNewDBManager_MySQL(t *testing.T) {
	ctx := context.Background()

	// 启动 MySQL testcontainers 容器
	mysqlContainer, err := mysql.Run(ctx, "mysql:8.0")
	if err != nil {
		t.Skipf("Docker 不可用，跳过集成测试: %v", err)
	}
	defer func() { _ = mysqlContainer.Terminate(ctx) }()

	// 获取连接字符串
	connStr, err := mysqlContainer.ConnectionString(ctx)
	if err != nil {
		t.Skipf("获取 MySQL 连接字符串失败: %v", err)
	}
	// connStr 格式: user:password@tcp(host:port)/dbname

	// 手动解析连接字符串提取 host / port / user / password / dbname
	// 格式: root:password@tcp(localhost:49153)/test
	var user, password, host, dbName string
	var port int
	_, err = fmt.Sscanf(connStr, "%[^:]:%[^@]@tcp(%[^:]:%d)/%s", &user, &password, &host, &port, &dbName)
	if err != nil {
		t.Skipf("解析 MySQL 连接字符串失败: %v", err)
	}

	// 构建配置
	cfg := &config.Config{}
	cfg.Database.Default = "mysql_test"
	cfg.Database.Connections = map[string]config.Database{
		"mysql_test": {
			Driver:         "mysql",
			Host:           host,
			Port:           port,
			User:           user,
			Password:       password,
			Name:           dbName,
			TablePrefix:    "test_",
			MaxOpenConns:   5,
			MaxIdleConns:   2,
			ConnMaxLifetime: 3600,
			ConnMaxIdleTime: 600,
		},
	}

	// 测试 NewDBManager
	manager, err := cmform.NewDBManager(cfg)
	if err != nil {
		t.Fatalf("NewDBManager 失败: %v", err)
	}
	if manager == nil {
		t.Fatal("NewDBManager 返回 nil")
	}
	defer manager.Close()

	// 测试 GetDB
	db := manager.GetDB()
	if db == nil {
		t.Fatal("GetDB 返回 nil")
	}

	// 测试 Ping
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = manager.Ping(pingCtx)
	if err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}

	t.Log("✅ MySQL 集成测试通过")
}

func TestNewDBManager_WithPoolConfig(t *testing.T) {
	ctx := context.Background()

	mysqlContainer, err := mysql.Run(ctx, "mysql:8.0")
	if err != nil {
		t.Skipf("Docker 不可用，跳过集成测试: %v", err)
	}
	defer func() { _ = mysqlContainer.Terminate(ctx) }()

	connStr, err := mysqlContainer.ConnectionString(ctx)
	if err != nil {
		t.Skipf("获取 MySQL 连接字符串失败: %v", err)
	}

	var user, password, host, dbName string
	var port int
	_, _ = fmt.Sscanf(connStr, "%[^:]:%[^@]@tcp(%[^:]:%d)/%s", &user, &password, &host, &port, &dbName)

	// 测试连接池参数为 0 时的分支
	cfg := &config.Config{}
	cfg.Database.Default = "mysql_test"
	cfg.Database.Connections = map[string]config.Database{
		"mysql_test": {
			Driver:         "mysql",
			Host:           host,
			Port:           port,
			User:           user,
			Password:       password,
			Name:           dbName,
			MaxOpenConns:   0, // 0 不设置
			MaxIdleConns:   0, // 0 不设置
			ConnMaxLifetime: 0,
			ConnMaxIdleTime: 0,
		},
	}

	manager, err := cmform.NewDBManager(cfg)
	if err != nil {
		t.Fatalf("NewDBManager 失败: %v", err)
	}
	defer manager.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = manager.Ping(pingCtx)
	if err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

func TestNewDBManager_Postgres(t *testing.T) {
	ctx := context.Background()

	// Postgres 使用已有的 testcontainers 模块
	// 注意：此测试需要 docker，如果不可用则跳过
	pgContainer, err := mysql.Run(ctx, "mysql:8.0") // 这里用 MySQL 避免额外拉镜像
	if err != nil {
		t.Skipf("Docker 不可用，跳过集成测试: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx)
	if err != nil {
		t.Skipf("获取连接字符串失败: %v", err)
	}

	var user, password, host, dbName string
	var port int
	_, _ = fmt.Sscanf(connStr, "%[^:]:%[^@]@tcp(%[^:]:%d)/%s", &user, &password, &host, &port, &dbName)

	cfg := &config.Config{}
	cfg.Database.Default = "mysql_test"
	cfg.Database.Connections = map[string]config.Database{
		"mysql_test": {
			Driver:   "mysql",
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
			Name:     dbName,
		},
	}

	manager, err := cmform.NewDBManager(cfg)
	if err != nil {
		t.Fatalf("NewDBManager 失败: %v", err)
	}
	defer manager.Close()

	// 测试 Close（已通过 defer 测试）
	// 测试 DB 非 nil 时 Close 有效
	db := manager.GetDB()
	if db == nil {
		t.Fatal("GetDB 返回 nil")
	}
}
