package redis_test

import (
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/wuwuseo/cmf/config"
	cmfredis "github.com/wuwuseo/cmf/redis"
)

// 测试级别全局变量，由 TestMain 初始化
var (
	testRedisContainer *redis.RedisContainer
	testRedisAddr      string
)

// TestMain 在所有测试运行前启动 Redis 容器，测试结束后终止容器
// 如果 Docker 环境不可用，则跳过所有测试
func TestMain(m *testing.M) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		println("跳过 Redis 测试：无法启动 testcontainers Redis 容器，", err.Error())
		os.Exit(0)
	}

	connStr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		redisContainer.Terminate(ctx)
		println("跳过 Redis 测试：无法获取 Redis 连接地址，", err.Error())
		os.Exit(0)
	}

	testRedisContainer = redisContainer
	testRedisAddr = connStr

	code := m.Run()

	if err := testRedisContainer.Terminate(ctx); err != nil {
		println("终止 Redis 容器失败:", err.Error())
	}

	os.Exit(code)
}

// =============================================================================
// 测试辅助函数
// =============================================================================

// newTestConfig 创建一个用于测试的 Config 对象，包含测试 Redis 容器的连接信息
func newTestConfig(storeName string) *config.Config {
	cfg := &config.Config{}
	cfg.Redis.Default = storeName
	cfg.Redis.Connections = map[string]config.Redis{
		storeName: {
			Addr: testRedisAddr,
		},
	}
	return cfg
}

// =============================================================================
// NewClient 测试
// =============================================================================

// TestNewClient_Ping 测试通过 NewClient 创建客户端后可以 Ping 通 Redis
func TestNewClient_Ping(t *testing.T) {
	ctx := context.Background()

	client := cmfredis.NewClient(&cmfredis.Options{
		Addr: testRedisAddr,
	})
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

// TestNewClient_SetGet 测试通过 NewClient 创建客户端后可以 Set/Get key
func TestNewClient_SetGet(t *testing.T) {
	ctx := context.Background()

	client := cmfredis.NewClient(&cmfredis.Options{
		Addr: testRedisAddr,
	})
	defer client.Close()

	key := "test_newclient_key"
	value := "hello_cmf"

	// 设置值
	if err := client.Set(ctx, key, value, 0).Err(); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	// 读取值并验证
	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got != value {
		t.Errorf("Get 值不匹配: 期望 %q, 得到 %q", value, got)
	}

	// 清理测试数据
	client.Del(ctx, key)
}

// =============================================================================
// NewClientFromConfig 测试
// =============================================================================

// TestNewClientFromConfig_Success 测试从 Config 创建 Redis 客户端并验证连接
func TestNewClientFromConfig_Success(t *testing.T) {
	ctx := context.Background()
	storeName := "test-cfc-success"
	cfg := newTestConfig(storeName)

	client, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName)
	if err != nil {
		t.Fatalf("NewClientFromConfig 失败: %v", err)
	}
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

// TestNewClientFromConfig_DefaultStoreName 测试不指定 storeName 时使用配置中的默认值
func TestNewClientFromConfig_DefaultStoreName(t *testing.T) {
	ctx := context.Background()
	storeName := "test-cfc-default"
	cfg := newTestConfig(storeName)

	// 不传 storeName 参数，应使用 cfg.Redis.Default
	client, err := cmfredis.NewClientFromConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig（无 storeName）失败: %v", err)
	}
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

// TestNewClientFromConfig_SameStoreNameReturnsSameInstance 测试相同 storeName 返回同一实例（单例验证）
func TestNewClientFromConfig_SameStoreNameReturnsSameInstance(t *testing.T) {
	ctx := context.Background()
	storeName := "test-cfc-singleton-same"
	cfg := newTestConfig(storeName)

	client1, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName)
	if err != nil {
		t.Fatalf("第一次调用 NewClientFromConfig 失败: %v", err)
	}
	defer client1.Close()

	client2, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName)
	if err != nil {
		t.Fatalf("第二次调用 NewClientFromConfig 失败: %v", err)
	}

	// 验证返回的是同一个指针实例
	if client1 != client2 {
		t.Error("相同 storeName 应返回同一实例（单例模式），但得到了不同实例")
	}
}

// TestNewClientFromConfig_DifferentStoreNameReturnsDifferentInstance 测试不同 storeName 返回不同实例
func TestNewClientFromConfig_DifferentStoreNameReturnsDifferentInstance(t *testing.T) {
	ctx := context.Background()
	storeName1 := "test-cfc-diff-1"
	storeName2 := "test-cfc-diff-2"

	// 构造包含两个连接的配置
	cfg := &config.Config{}
	cfg.Redis.Connections = map[string]config.Redis{
		storeName1: {Addr: testRedisAddr},
		storeName2: {Addr: testRedisAddr},
	}

	client1, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName1)
	if err != nil {
		t.Fatalf("创建 client1 失败: %v", err)
	}
	defer client1.Close()

	client2, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName2)
	if err != nil {
		t.Fatalf("创建 client2 失败: %v", err)
	}
	defer client2.Close()

	// 验证返回的是不同实例
	if client1 == client2 {
		t.Error("不同 storeName 应返回不同实例，但得到了同一个实例")
	}
}

// =============================================================================
// 边界条件测试
// =============================================================================

// TestNewClientFromConfig_NonExistentStoreName 测试使用不存在的 storeName 返回错误
func TestNewClientFromConfig_NonExistentStoreName(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Redis.Default = "default-redis"
	cfg.Redis.Connections = map[string]config.Redis{
		"default-redis": {Addr: testRedisAddr},
	}

	_, err := cmfredis.NewClientFromConfig(ctx, cfg, "non-existent-store")
	if err == nil {
		t.Fatal("使用不存在的 storeName 应返回错误，但没有返回错误")
	}
}

// TestNewClientFromConfig_DefaultStoreNotFound 测试 Default 指向不存在的连接时返回错误
func TestNewClientFromConfig_DefaultStoreNotFound(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Redis.Default = "ghost-redis"
	cfg.Redis.Connections = map[string]config.Redis{
		"real-redis": {Addr: testRedisAddr},
	}

	_, err := cmfredis.NewClientFromConfig(ctx, cfg)
	if err == nil {
		t.Fatal("Default 指向不存在的连接时应返回错误，但没有返回错误")
	}
}

// TestNewClientFromConfig_TLS 测试 TLS 配置
// 由于 testcontainers Redis 默认不支持 TLS，启用 TLS 后连接会失败，验证错误不为 nil 即可
func TestNewClientFromConfig_TLS(t *testing.T) {
	ctx := context.Background()
	storeName := "test-cfc-tls"
	cfg := &config.Config{}
	cfg.Redis.Connections = map[string]config.Redis{
		storeName: {
			Addr:   testRedisAddr,
			UseTLS: true,
		},
	}

	_, err := cmfredis.NewClientFromConfig(ctx, cfg, storeName)
	if err == nil {
		t.Fatal("TLS 连接到非 TLS Redis 应返回错误，但没有返回错误")
	}
}
