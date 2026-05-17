package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/wuwuseo/cmf/config"
)

// =============================================================================
// NewViper 测试
// =============================================================================

// TestNewViper 测试创建默认 Viper 实例，验证配置名和路径
func TestNewViper(t *testing.T) {
	v := config.NewViper("testconfig")

	if v == nil {
		t.Fatal("NewViper 返回了 nil")
	}

	// NewViper 内部调用 NewViperWithOptions，默认 envPrefix 应为 "CMF"
	// 通过设置环境变量验证 envPrefix 生效
	envKey := "CMF_TEST_NEWVIPER_KEY"
	os.Setenv(envKey, "env_value")
	defer os.Unsetenv(envKey)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if got := v.GetString("test.newviper.key"); got != "env_value" {
		t.Errorf("环境变量读取失败（envPrefix=CMF）: 期望 %q, 得到 %q", "env_value", got)
	}

	// 通过创建配置文件验证配置名：在临时目录写入同名配置文件并读取
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "testconfig.yaml")
	os.WriteFile(configFile, []byte("app:\n  name: viper_from_file\n"), 0644)

	v2 := config.NewViper("testconfig")
	v2.AddConfigPath(tmpDir)
	v2.SetConfigType("yaml")
	if err := v2.ReadInConfig(); err != nil {
		t.Fatalf("配置文件读取失败（验证配置名）: %v", err)
	}
	if got := v2.GetString("app.name"); got != "viper_from_file" {
		t.Errorf("配置名验证失败: 期望 %q, 得到 %q", "viper_from_file", got)
	}
}

// =============================================================================
// NewViperWithOptions 测试
// =============================================================================

// TestNewViperWithOptions 测试使用自定义参数创建 Viper 实例
func TestNewViperWithOptions(t *testing.T) {
	const (
		customName   = "custom_config"
		customPrefix = "MYAPP"
	)

	v := config.NewViperWithOptions(customName, customPrefix)

	if v == nil {
		t.Fatal("NewViperWithOptions 返回了 nil")
	}

	// 验证自定义 envPrefix
	envKey := "MYAPP_CUSTOM_TEST_KEY"
	os.Setenv(envKey, "custom_env_value")
	defer os.Unsetenv(envKey)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if got := v.GetString("custom.test.key"); got != "custom_env_value" {
		t.Errorf("自定义环境变量读取失败: 期望 %q, 得到 %q", "custom_env_value", got)
	}

	// 通过创建配置文件验证自定义配置名
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, customName+".yaml")
	os.WriteFile(configFile, []byte("app:\n  name: custom_viper_file\n"), 0644)

	v2 := config.NewViperWithOptions(customName, customPrefix)
	v2.AddConfigPath(tmpDir)
	v2.SetConfigType("yaml")
	if err := v2.ReadInConfig(); err != nil {
		t.Fatalf("配置文件读取失败（验证自定义配置名）: %v", err)
	}
	if got := v2.GetString("app.name"); got != "custom_viper_file" {
		t.Errorf("自定义配置名验证失败: 期望 %q, 得到 %q", "custom_viper_file", got)
	}
}

// =============================================================================
// Config 结构体 Unmarshal 测试
// =============================================================================

// TestConfigUnmarshal 测试从 viper 中将配置 Unmarshal 到 Config 结构体
func TestConfigUnmarshal(t *testing.T) {
	v := viper.New()

	// 设置 App 嵌套结构体字段
	v.Set("app.name", "测试应用")
	v.Set("app.port", 8080)
	v.Set("app.debug", true)
	v.Set("app.idle_timeout", 120)
	v.Set("app.prefork", true)
	v.Set("app.swagger", true)
	v.Set("app.secret", "test-secret-key")
	v.Set("app.login_expires", 7200)
	v.Set("app.refresh_expires", 86400)
	v.Set("app.body_limit", 5242880)

	c := &config.Config{}
	if err := v.Unmarshal(c); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}

	// 验证 App 字段
	if c.App.Name != "测试应用" {
		t.Errorf("App.Name: 期望 %q, 得到 %q", "测试应用", c.App.Name)
	}
	if c.App.Port != 8080 {
		t.Errorf("App.Port: 期望 %d, 得到 %d", 8080, c.App.Port)
	}
	if !c.App.Debug {
		t.Error("App.Debug: 期望 true, 得到 false")
	}
	if c.App.IdleTimeout != 120 {
		t.Errorf("App.IdleTimeout: 期望 %d, 得到 %d", 120, c.App.IdleTimeout)
	}
	if !c.App.Prefork {
		t.Error("App.Prefork: 期望 true, 得到 false")
	}
	if !c.App.Swagger {
		t.Error("App.Swagger: 期望 true, 得到 false")
	}
	if c.App.Secret != "test-secret-key" {
		t.Errorf("App.Secret: 期望 %q, 得到 %q", "test-secret-key", c.App.Secret)
	}
	if c.App.LoginExpires != 7200 {
		t.Errorf("App.LoginExpires: 期望 %d, 得到 %d", 7200, c.App.LoginExpires)
	}
	if c.App.RefreshExpires != 86400 {
		t.Errorf("App.RefreshExpires: 期望 %d, 得到 %d", 86400, c.App.RefreshExpires)
	}
	if c.App.BodyLimit != 5242880 {
		t.Errorf("App.BodyLimit: 期望 %d, 得到 %d", 5242880, c.App.BodyLimit)
	}
}

// =============================================================================
// GetString / GetInt / GetBool 包级函数测试
// =============================================================================

// TestGetString 测试包级 GetString 函数（依赖 init() 初始化的 v）
func TestGetString(t *testing.T) {
	// init() 中已调用 InitConfig() 设置了默认值
	// app.name 默认值为 "app"
	got := config.GetString("app.name")
	if got != "app" {
		t.Errorf("GetString(app.name): 期望 %q, 得到 %q", "app", got)
	}

	// 未设置的值应返回空字符串
	got = config.GetString("nonexistent.key.xyz")
	if got != "" {
		t.Errorf("GetString 不存在的键: 期望 %q, 得到 %q", "", got)
	}
}

// TestGetInt 测试包级 GetInt 函数
func TestGetInt(t *testing.T) {
	// app.port 默认值为 3000
	got := config.GetInt("app.port")
	if got != 3000 {
		t.Errorf("GetInt(app.port): 期望 %d, 得到 %d", 3000, got)
	}

	// 未设置的值应返回 0
	got = config.GetInt("nonexistent.key.xyz")
	if got != 0 {
		t.Errorf("GetInt 不存在的键: 期望 %d, 得到 %d", 0, got)
	}
}

// TestGetBool 测试包级 GetBool 函数
func TestGetBool(t *testing.T) {
	// app.debug 默认值为 false
	got := config.GetBool("app.debug")
	if got != false {
		t.Errorf("GetBool(app.debug): 期望 %v, 得到 %v", false, got)
	}

	// swagger 默认值也为 false
	got = config.GetBool("app.swagger")
	if got != false {
		t.Errorf("GetBool(app.swagger): 期望 %v, 得到 %v", false, got)
	}
}

// =============================================================================
// Config 结构体方法委托测试
// =============================================================================

// TestConfigGetString 测试 Config.GetString 方法委托到包级函数
func TestConfigGetString(t *testing.T) {
	c := config.Conf
	if c == nil {
		t.Skip("config.Conf 未初始化，跳过测试")
	}

	got := c.GetString("app.name")
	if got != "app" {
		t.Errorf("Config.GetString(app.name): 期望 %q, 得到 %q", "app", got)
	}
}

// TestConfigGetInt 测试 Config.GetInt 方法委托到包级函数
func TestConfigGetInt(t *testing.T) {
	c := config.Conf
	if c == nil {
		t.Skip("config.Conf 未初始化，跳过测试")
	}

	got := c.GetInt("app.port")
	if got != 3000 {
		t.Errorf("Config.GetInt(app.port): 期望 %d, 得到 %d", 3000, got)
	}
}

// TestConfigGetBool 测试 Config.GetBool 方法委托到包级函数
func TestConfigGetBool(t *testing.T) {
	c := config.Conf
	if c == nil {
		t.Skip("config.Conf 未初始化，跳过测试")
	}

	got := c.GetBool("app.debug")
	if got != false {
		t.Errorf("Config.GetBool(app.debug): 期望 %v, 得到 %v", false, got)
	}
}

// =============================================================================
// SaveConfig 测试（写入文件后重新读取验证）
// =============================================================================

// TestSaveConfig 测试 SaveConfig 保存配置到文件并验证
func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configName := "test_save_config"
	configFile := filepath.Join(tmpDir, configName+".yaml")

	// 创建初始配置文件
	initialYAML := []byte("app:\n  name: oldapp\n  port: 3000\n")
	if err := os.WriteFile(configFile, initialYAML, 0644); err != nil {
		t.Fatalf("写入初始配置文件失败: %v", err)
	}

	// 创建 viper 实例并读取配置
	vv := viper.New()
	vv.SetConfigName(configName)
	vv.SetConfigType("yaml")
	vv.AddConfigPath(tmpDir)

	if err := vv.ReadInConfig(); err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	// 调用 SaveConfig 修改 app.name
	if err := config.SaveConfig(vv, "app", "name", "newapp", "default"); err != nil {
		t.Fatalf("SaveConfig 失败: %v", err)
	}

	// 重新读取文件内容验证
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("重新读取配置文件失败: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "newapp") {
		t.Errorf("配置文件中未找到 'newapp'，文件内容:\n%s", contentStr)
	}
	if strings.Contains(contentStr, "oldapp") {
		t.Errorf("配置文件中不应再包含 'oldapp'，文件内容:\n%s", contentStr)
	}

	// 用新的 viper 读取验证
	v2 := viper.New()
	v2.SetConfigName(configName)
	v2.SetConfigType("yaml")
	v2.AddConfigPath(tmpDir)
	if err := v2.ReadInConfig(); err != nil {
		t.Fatalf("二次读取配置文件失败: %v", err)
	}

	if got := v2.GetString("app.name"); got != "newapp" {
		t.Errorf("二次读取 app.name: 期望 %q, 得到 %q", "newapp", got)
	}
}

// TestSaveConfigNewSection 测试 SaveConfig 创建新的 section
func TestSaveConfigNewSection(t *testing.T) {
	tmpDir := t.TempDir()
	configName := "test_new_section"
	configFile := filepath.Join(tmpDir, configName+".yaml")

	// 创建只有 app section 的初始配置
	initialYAML := []byte("app:\n  name: testapp\n")
	if err := os.WriteFile(configFile, initialYAML, 0644); err != nil {
		t.Fatalf("写入初始配置文件失败: %v", err)
	}

	vv := viper.New()
	vv.SetConfigName(configName)
	vv.SetConfigType("yaml")
	vv.AddConfigPath(tmpDir)
	if err := vv.ReadInConfig(); err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	// 添加全新的 log section
	if err := config.SaveConfig(vv, "log", "level", "debug", "info"); err != nil {
		t.Fatalf("SaveConfig 新 section 失败: %v", err)
	}

	// 验证
	v2 := viper.New()
	v2.SetConfigName(configName)
	v2.SetConfigType("yaml")
	v2.AddConfigPath(tmpDir)
	if err := v2.ReadInConfig(); err != nil {
		t.Fatalf("二次读取配置文件失败: %v", err)
	}

	if got := v2.GetString("log.level"); got != "debug" {
		t.Errorf("新 section log.level: 期望 %q, 得到 %q", "debug", got)
	}
}

// TestSaveConfigDefaultValue 测试 SaveConfig 使用默认值
func TestSaveConfigDefaultValue(t *testing.T) {
	tmpDir := t.TempDir()
	configName := "test_default_value"
	configFile := filepath.Join(tmpDir, configName+".yaml")

	initialYAML := []byte("app:\n  name: testapp\n")
	if err := os.WriteFile(configFile, initialYAML, 0644); err != nil {
		t.Fatalf("写入初始配置文件失败: %v", err)
	}

	vv := viper.New()
	vv.SetConfigName(configName)
	vv.SetConfigType("yaml")
	vv.AddConfigPath(tmpDir)
	if err := vv.ReadInConfig(); err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	// value 为 nil，应使用 defaultValue
	if err := config.SaveConfig(vv, "app", "port", nil, 9999); err != nil {
		t.Fatalf("SaveConfig 默认值失败: %v", err)
	}

	v2 := viper.New()
	v2.SetConfigName(configName)
	v2.SetConfigType("yaml")
	v2.AddConfigPath(tmpDir)
	if err := v2.ReadInConfig(); err != nil {
		t.Fatalf("二次读取配置文件失败: %v", err)
	}

	if got := v2.GetInt("app.port"); got != 9999 {
		t.Errorf("默认值 app.port: 期望 %d, 得到 %d", 9999, got)
	}
}

// =============================================================================
// Config 嵌套结构体赋值和读取测试
// =============================================================================

// TestConfigNestedStructs 测试 Config 所有嵌套结构体的赋值和读取
func TestConfigNestedStructs(t *testing.T) {
	v := viper.New()

	// --- App 嵌套结构体 ---
	v.Set("app.name", "嵌套测试应用")
	v.Set("app.port", 9090)
	v.Set("app.debug", true)
	v.Set("app.idle_timeout", 30)
	v.Set("app.prefork", false)
	v.Set("app.swagger", true)
	v.Set("app.secret", "nested-secret")
	v.Set("app.login_expires", 3600)
	v.Set("app.refresh_expires", 604800)
	v.Set("app.body_limit", 10485760)

	// --- Log 嵌套结构体 ---
	v.Set("log.level", "debug")
	v.Set("log.format", "text")
	v.Set("log.file_path", "/var/log/app.log")
	v.Set("log.console_output", true)
	v.Set("log.file_output", false)
	v.Set("log.max_size", 50)
	v.Set("log.max_backups", 5)
	v.Set("log.max_age", 30)

	// --- Database 嵌套结构体 ---
	v.Set("database.default", "mysql_conn")
	v.Set("database.connections.mysql_conn.driver", "mysql")
	v.Set("database.connections.mysql_conn.host", "192.168.1.100")
	v.Set("database.connections.mysql_conn.port", 3307)
	v.Set("database.connections.mysql_conn.user", "admin")
	v.Set("database.connections.mysql_conn.password", "admin123")
	v.Set("database.connections.mysql_conn.name", "testdb")
	v.Set("database.connections.mysql_conn.ssl_mode", "require")
	v.Set("database.connections.mysql_conn.table_prefix", "tb_")
	v.Set("database.connections.mysql_conn.max_open_conns", 50)
	v.Set("database.connections.mysql_conn.max_idle_conns", 20)
	v.Set("database.connections.mysql_conn.conn_max_lifetime", 1800)
	v.Set("database.connections.mysql_conn.conn_max_idle_time", 300)

	// --- Cache 嵌套结构体 ---
	v.Set("cache.default", "redis")
	v.Set("cache.stores.memory.driver", "memory")
	v.Set("cache.stores.memory.default_ttl", 1800)
	v.Set("cache.stores.redis.driver", "redis")
	v.Set("cache.stores.redis.default_ttl", 7200)

	// --- Redis 嵌套结构体 ---
	v.Set("redis.default", "cache_redis")
	v.Set("redis.connections.cache_redis.addr", "redis-cluster:6379")
	v.Set("redis.connections.cache_redis.username", "redisuser")
	v.Set("redis.connections.cache_redis.password", "redispwd")
	v.Set("redis.connections.cache_redis.db", 1)
	v.Set("redis.connections.cache_redis.dial_timeout", 10)
	v.Set("redis.connections.cache_redis.read_timeout", 5)
	v.Set("redis.connections.cache_redis.write_timeout", 5)
	v.Set("redis.connections.cache_redis.pool_size", 20)
	v.Set("redis.connections.cache_redis.min_idle_conns", 10)
	v.Set("redis.connections.cache_redis.max_idle_conns", 20)
	v.Set("redis.connections.cache_redis.conn_max_idle_time", 15)
	v.Set("redis.connections.cache_redis.conn_max_lifetime", 12)
	v.Set("redis.connections.cache_redis.use_tls", true)

	// --- Filesystem 嵌套结构体 ---
	v.Set("filesystem.default", "s3")
	v.Set("filesystem.is_and_local", true)
	v.Set("filesystem.disks.local.driver", "local")
	v.Set("filesystem.disks.local.options.root", "/data/uploads")
	v.Set("filesystem.disks.s3.driver", "s3")
	v.Set("filesystem.disks.s3.options.access_key", "AKIATEST")
	v.Set("filesystem.disks.s3.options.secret_key", "SECRETTEST")
	v.Set("filesystem.disks.s3.options.region", "us-east-1")
	v.Set("filesystem.disks.s3.options.bucket", "my-bucket")
	v.Set("filesystem.disks.s3.options.endpoint", "https://s3.amazonaws.com")

	// --- Casbin 嵌套结构体 ---
	v.Set("casbin.domains_default", "app_domain")
	v.Set("casbin.domains", []map[string]any{
		{
			"name":       "app_domain",
			"auto_load":  true,
			"model_path": "./config/casbin_model.conf",
			"model_text": "[request_definition]\nr = sub, obj, act",
		},
		{
			"name":       "admin_domain",
			"auto_load":  false,
			"model_path": "./config/admin_model.conf",
		},
	})

	c := &config.Config{}
	if err := v.Unmarshal(c); err != nil {
		t.Fatalf("嵌套结构体 Unmarshal 失败: %v", err)
	}

	// ============================
	// 验证 App 嵌套结构体
	// ============================
	t.Run("App", func(t *testing.T) {
		if c.App.Name != "嵌套测试应用" {
			t.Errorf("App.Name: 期望 %q, 得到 %q", "嵌套测试应用", c.App.Name)
		}
		if c.App.Port != 9090 {
			t.Errorf("App.Port: 期望 %d, 得到 %d", 9090, c.App.Port)
		}
		if !c.App.Debug {
			t.Error("App.Debug: 期望 true")
		}
		if c.App.IdleTimeout != 30 {
			t.Errorf("App.IdleTimeout: 期望 %d, 得到 %d", 30, c.App.IdleTimeout)
		}
		if c.App.Prefork {
			t.Error("App.Prefork: 期望 false")
		}
		if !c.App.Swagger {
			t.Error("App.Swagger: 期望 true")
		}
		if c.App.Secret != "nested-secret" {
			t.Errorf("App.Secret: 期望 %q, 得到 %q", "nested-secret", c.App.Secret)
		}
		if c.App.LoginExpires != 3600 {
			t.Errorf("App.LoginExpires: 期望 %d, 得到 %d", 3600, c.App.LoginExpires)
		}
		if c.App.RefreshExpires != 604800 {
			t.Errorf("App.RefreshExpires: 期望 %d, 得到 %d", 604800, c.App.RefreshExpires)
		}
		if c.App.BodyLimit != 10485760 {
			t.Errorf("App.BodyLimit: 期望 %d, 得到 %d", 10485760, c.App.BodyLimit)
		}
	})

	// ============================
	// 验证 Log 嵌套结构体
	// ============================
	t.Run("Log", func(t *testing.T) {
		if c.Log.Level != "debug" {
			t.Errorf("Log.Level: 期望 %q, 得到 %q", "debug", c.Log.Level)
		}
		if c.Log.Format != "text" {
			t.Errorf("Log.Format: 期望 %q, 得到 %q", "text", c.Log.Format)
		}
		if c.Log.FilePath != "/var/log/app.log" {
			t.Errorf("Log.FilePath: 期望 %q, 得到 %q", "/var/log/app.log", c.Log.FilePath)
		}
		if !c.Log.ConsoleOutput {
			t.Error("Log.ConsoleOutput: 期望 true")
		}
		if c.Log.FileOutput {
			t.Error("Log.FileOutput: 期望 false")
		}
		if c.Log.MaxSize != 50 {
			t.Errorf("Log.MaxSize: 期望 %d, 得到 %d", 50, c.Log.MaxSize)
		}
		if c.Log.MaxBackups != 5 {
			t.Errorf("Log.MaxBackups: 期望 %d, 得到 %d", 5, c.Log.MaxBackups)
		}
		if c.Log.MaxAge != 30 {
			t.Errorf("Log.MaxAge: 期望 %d, 得到 %d", 30, c.Log.MaxAge)
		}
	})

	// ============================
	// 验证 Database 嵌套结构体
	// ============================
	t.Run("Database", func(t *testing.T) {
		if c.Database.Default != "mysql_conn" {
			t.Errorf("Database.Default: 期望 %q, 得到 %q", "mysql_conn", c.Database.Default)
		}
		conn, ok := c.Database.Connections["mysql_conn"]
		if !ok {
			t.Fatal("Database.Connections 中未找到 'mysql_conn'")
		}
		if conn.Driver != "mysql" {
			t.Errorf("Database.Connections[mysql_conn].Driver: 期望 %q, 得到 %q", "mysql", conn.Driver)
		}
		if conn.Host != "192.168.1.100" {
			t.Errorf("Database.Connections[mysql_conn].Host: 期望 %q, 得到 %q", "192.168.1.100", conn.Host)
		}
		if conn.Port != 3307 {
			t.Errorf("Database.Connections[mysql_conn].Port: 期望 %d, 得到 %d", 3307, conn.Port)
		}
		if conn.User != "admin" {
			t.Errorf("Database.Connections[mysql_conn].User: 期望 %q, 得到 %q", "admin", conn.User)
		}
		if conn.Password != "admin123" {
			t.Errorf("Database.Connections[mysql_conn].Password: 期望 %q, 得到 %q", "admin123", conn.Password)
		}
		if conn.Name != "testdb" {
			t.Errorf("Database.Connections[mysql_conn].Name: 期望 %q, 得到 %q", "testdb", conn.Name)
		}
		if conn.SSLMode != "require" {
			t.Errorf("Database.Connections[mysql_conn].SSLMode: 期望 %q, 得到 %q", "require", conn.SSLMode)
		}
		if conn.TablePrefix != "tb_" {
			t.Errorf("Database.Connections[mysql_conn].TablePrefix: 期望 %q, 得到 %q", "tb_", conn.TablePrefix)
		}
		if conn.MaxOpenConns != 50 {
			t.Errorf("Database.Connections[mysql_conn].MaxOpenConns: 期望 %d, 得到 %d", 50, conn.MaxOpenConns)
		}
		if conn.MaxIdleConns != 20 {
			t.Errorf("Database.Connections[mysql_conn].MaxIdleConns: 期望 %d, 得到 %d", 20, conn.MaxIdleConns)
		}
		if conn.ConnMaxLifetime != 1800 {
			t.Errorf("Database.Connections[mysql_conn].ConnMaxLifetime: 期望 %d, 得到 %d", 1800, conn.ConnMaxLifetime)
		}
		if conn.ConnMaxIdleTime != 300 {
			t.Errorf("Database.Connections[mysql_conn].ConnMaxIdleTime: 期望 %d, 得到 %d", 300, conn.ConnMaxIdleTime)
		}
	})

	// ============================
	// 验证 Cache 嵌套结构体
	// ============================
	t.Run("Cache", func(t *testing.T) {
		if c.Cache.Default != "redis" {
			t.Errorf("Cache.Default: 期望 %q, 得到 %q", "redis", c.Cache.Default)
		}

		memStore, ok := c.Cache.Stores["memory"]
		if !ok {
			t.Fatal("Cache.Stores 中未找到 'memory'")
		}
		if memStore.Driver != "memory" {
			t.Errorf("Cache.Stores[memory].Driver: 期望 %q, 得到 %q", "memory", memStore.Driver)
		}
		if memStore.DefaultTTL != 1800 {
			t.Errorf("Cache.Stores[memory].DefaultTTL: 期望 %d, 得到 %d", 1800, memStore.DefaultTTL)
		}

		redisStore, ok := c.Cache.Stores["redis"]
		if !ok {
			t.Fatal("Cache.Stores 中未找到 'redis'")
		}
		if redisStore.Driver != "redis" {
			t.Errorf("Cache.Stores[redis].Driver: 期望 %q, 得到 %q", "redis", redisStore.Driver)
		}
		if redisStore.DefaultTTL != 7200 {
			t.Errorf("Cache.Stores[redis].DefaultTTL: 期望 %d, 得到 %d", 7200, redisStore.DefaultTTL)
		}
	})

	// ============================
	// 验证 Redis 嵌套结构体
	// ============================
	t.Run("Redis", func(t *testing.T) {
		if c.Redis.Default != "cache_redis" {
			t.Errorf("Redis.Default: 期望 %q, 得到 %q", "cache_redis", c.Redis.Default)
		}

		r, ok := c.Redis.Connections["cache_redis"]
		if !ok {
			t.Fatal("Redis.Connections 中未找到 'cache_redis'")
		}
		if r.Addr != "redis-cluster:6379" {
			t.Errorf("Redis.Addr: 期望 %q, 得到 %q", "redis-cluster:6379", r.Addr)
		}
		if r.Username != "redisuser" {
			t.Errorf("Redis.Username: 期望 %q, 得到 %q", "redisuser", r.Username)
		}
		if r.Password != "redispwd" {
			t.Errorf("Redis.Password: 期望 %q, 得到 %q", "redispwd", r.Password)
		}
		if r.DB != 1 {
			t.Errorf("Redis.DB: 期望 %d, 得到 %d", 1, r.DB)
		}
		if r.DialTimeout != 10 {
			t.Errorf("Redis.DialTimeout: 期望 %d, 得到 %d", 10, r.DialTimeout)
		}
		if r.ReadTimeout != 5 {
			t.Errorf("Redis.ReadTimeout: 期望 %d, 得到 %d", 5, r.ReadTimeout)
		}
		if r.WriteTimeout != 5 {
			t.Errorf("Redis.WriteTimeout: 期望 %d, 得到 %d", 5, r.WriteTimeout)
		}
		if r.PoolSize != 20 {
			t.Errorf("Redis.PoolSize: 期望 %d, 得到 %d", 20, r.PoolSize)
		}
		if r.MinIdleConns != 10 {
			t.Errorf("Redis.MinIdleConns: 期望 %d, 得到 %d", 10, r.MinIdleConns)
		}
		if r.MaxIdleConns != 20 {
			t.Errorf("Redis.MaxIdleConns: 期望 %d, 得到 %d", 20, r.MaxIdleConns)
		}
		if r.ConnMaxIdleTime != 15 {
			t.Errorf("Redis.ConnMaxIdleTime: 期望 %d, 得到 %d", 15, r.ConnMaxIdleTime)
		}
		if r.ConnMaxLifetime != 12 {
			t.Errorf("Redis.ConnMaxLifetime: 期望 %d, 得到 %d", 12, r.ConnMaxLifetime)
		}
		if !r.UseTLS {
			t.Error("Redis.UseTLS: 期望 true")
		}
	})

	// ============================
	// 验证 Filesystem 嵌套结构体
	// ============================
	t.Run("Filesystem", func(t *testing.T) {
		if c.Filesystem.Default != "s3" {
			t.Errorf("Filesystem.Default: 期望 %q, 得到 %q", "s3", c.Filesystem.Default)
		}
		if !c.Filesystem.IsAndLocal {
			t.Error("Filesystem.IsAndLocal: 期望 true")
		}

		localDisk, ok := c.Filesystem.Disks["local"]
		if !ok {
			t.Fatal("Filesystem.Disks 中未找到 'local'")
		}
		if localDisk.Driver != "local" {
			t.Errorf("Filesystem.Disks[local].Driver: 期望 %q, 得到 %q", "local", localDisk.Driver)
		}

		s3Disk, ok := c.Filesystem.Disks["s3"]
		if !ok {
			t.Fatal("Filesystem.Disks 中未找到 's3'")
		}
		if s3Disk.Driver != "s3" {
			t.Errorf("Filesystem.Disks[s3].Driver: 期望 %q, 得到 %q", "s3", s3Disk.Driver)
		}
	})

	// ============================
	// 验证 Casbin 嵌套结构体
	// ============================
	t.Run("Casbin", func(t *testing.T) {
		if c.Casbin.DomainsDefault != "app_domain" {
			t.Errorf("Casbin.DomainsDefault: 期望 %q, 得到 %q", "app_domain", c.Casbin.DomainsDefault)
		}
		if len(c.Casbin.Domains) != 2 {
			t.Fatalf("Casbin.Domains 长度: 期望 %d, 得到 %d", 2, len(c.Casbin.Domains))
		}

		d0 := c.Casbin.Domains[0]
		if d0.Name != "app_domain" {
			t.Errorf("Casbin.Domains[0].Name: 期望 %q, 得到 %q", "app_domain", d0.Name)
		}
		if !d0.AutoLoad {
			t.Error("Casbin.Domains[0].AutoLoad: 期望 true")
		}
		if d0.ModelPath != "./config/casbin_model.conf" {
			t.Errorf("Casbin.Domains[0].ModelPath: 期望 %q, 得到 %q", "./config/casbin_model.conf", d0.ModelPath)
		}
		if d0.ModelText != "[request_definition]\nr = sub, obj, act" {
			t.Errorf("Casbin.Domains[0].ModelText: 不匹配")
		}

		d1 := c.Casbin.Domains[1]
		if d1.Name != "admin_domain" {
			t.Errorf("Casbin.Domains[1].Name: 期望 %q, 得到 %q", "admin_domain", d1.Name)
		}
		if d1.AutoLoad {
			t.Error("Casbin.Domains[1].AutoLoad: 期望 false")
		}
		if d1.ModelPath != "./config/admin_model.conf" {
			t.Errorf("Casbin.Domains[1].ModelPath: 期望 %q, 得到 %q", "./config/admin_model.conf", d1.ModelPath)
		}
	})
}

// =============================================================================
// NewConfig 测试
// =============================================================================

// TestNewConfig 测试 NewConfig 创建 Config 实例
func TestNewConfig(t *testing.T) {
	c := config.NewConfig()

	if c == nil {
		t.Fatal("NewConfig 返回了 nil")
	}

	// NewConfig 内部调用 InitConfig，应包含默认值
	if c.App.Name != "app" {
		t.Errorf("App.Name 默认值: 期望 %q, 得到 %q", "app", c.App.Name)
	}
	if c.App.Port != 3000 {
		t.Errorf("App.Port 默认值: 期望 %d, 得到 %d", 3000, c.App.Port)
	}
	if c.Cache.Default != "memory" {
		t.Errorf("Cache.Default 默认值: 期望 %q, 得到 %q", "memory", c.Cache.Default)
	}
	if c.Redis.Default != "redis" {
		t.Errorf("Redis.Default 默认值: 期望 %q, 得到 %q", "redis", c.Redis.Default)
	}
}


