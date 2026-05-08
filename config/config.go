package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Redis struct {
	Addr            string `mapstructure:"addr"`               // Redis服务器地址，格式为"host:port"
	Username        string `mapstructure:"username"`           // Redis用户名，无用户名时为空字符串
	Password        string `mapstructure:"password"`           // Redis密码，无密码时为空字符串
	DB              int    `mapstructure:"db"`                 // Redis数据库索引
	DialTimeout     int    `mapstructure:"dial_timeout"`       // 连接超时时间（秒）
	ReadTimeout     int    `mapstructure:"read_timeout"`       // 读取超时时间（秒）
	WriteTimeout    int    `mapstructure:"write_timeout"`      // 写入超时时间（秒）
	PoolSize        int    `mapstructure:"pool_size"`          // 连接池大小
	MinIdleConns    int    `mapstructure:"min_idle_conns"`     // 最小空闲连接数
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`     // 最大空闲连接数
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间（分钟）
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`  // 连接最大生命周期（小时）
	UseTLS          bool   `mapstructure:"use_tls"`            // 是否使用TLS加密连接
}

type Database struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Name            string `mapstructure:"name"`
	SSLMode         string `mapstructure:"ssl_mode"`
	TablePrefix     string `mapstructure:"table_prefix"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"`
}

type Config struct {
	App struct {
		Name           string `mapstructure:"name"`
		Port           int    `mapstructure:"port"`
		Debug          bool   `mapstructure:"debug"`
		IdleTimeout    int    `mapstructure:"idle_timeout"`
		Prefork        bool   `mapstructure:"prefork"`
		Swagger        bool   `mapstructure:"swagger"`
		Secret         string `mapstructure:"secret"`
		LoginExpires   int    `mapstructure:"login_expires"`
		RefreshExpires int    `mapstructure:"refresh_expires"`
		BodyLimit      int    `mapstructure:"body_limit"`
	} `mapstructure:"app"`

	Log struct {
		Level         string `mapstructure:"level"`
		Format        string `mapstructure:"format"`
		FilePath      string `mapstructure:"file_path"`
		ConsoleOutput bool   `mapstructure:"console_output"`
		FileOutput    bool   `mapstructure:"file_output"`
		MaxSize       int    `mapstructure:"max_size"`
		MaxBackups    int    `mapstructure:"max_backups"`
		MaxAge        int    `mapstructure:"max_age"`
	} `mapstructure:"log"`

	Database struct {
		Default     string              `mapstructure:"default"`
		Connections map[string]Database `mapstructure:"connections"`
	} `mapstructure:"database"`

	// 缓存配置
	Cache struct {
		Default string `mapstructure:"default"` // 默认缓存存储
		Stores  map[string]struct {
			Driver     string `mapstructure:"driver"`      // 缓存驱动类型，如 bigcache, memory
			DefaultTTL int    `mapstructure:"default_ttl"` // 默认缓存过期时间（秒）
			Options    any    `mapstructure:"options"`     // 缓存驱动选项
		} `mapstructure:"stores"`
	} `mapstructure:"cache"`

	Redis struct {
		Default     string           `mapstructure:"default"`
		Connections map[string]Redis `mapstructure:"connections"`
	} `mapstructure:"redis"`

	Filesystem struct {
		Default    string `mapstructure:"default"`
		IsAndLocal bool   `mapstructure:"is_and_local"` // 是否同时存储在本地文件系统
		Disks      map[string]struct {
			Driver  string `mapstructure:"driver"`
			Options any    `mapstructure:"options"`
		} `mapstructure:"disks"`
	} `mapstructure:"filesystem"`

	Casbin struct {
		DomainsDefault string `mapstructure:"domains_default"` // 默认域名称
		Domains        []struct {
			Name      string `mapstructure:"name"`       // 域名称
			AutoLoad  bool   `mapstructure:"auto_load"`  // 是否自动加载
			ModelPath string `mapstructure:"model_path"` // 模型文件路径
			ModelText string `mapstructure:"model_text"` // 模型文本内容
		} `mapstructure:"domains"` // 多域配置列表
	} `mapstructure:"casbin"`
}

var v *viper.Viper
var Conf *Config

func init() {
	initEnv()
	v = NewViper("config")
	Conf = NewConfig()
}

// NewViper 创建一个带有默认参数的 Viper 实例
func NewViper(name string) *viper.Viper {
	return NewViperWithOptions(name, "CMF")
}

// NewViperWithOptions 创建一个可自定义参数的 Viper 实例
func NewViperWithOptions(name string, envPrefix string) *viper.Viper {
	Viper := viper.New()
	// 初始化配置
	Viper.SetEnvPrefix(envPrefix)
	Viper.SetConfigName(name)
	Viper.AddConfigPath("./config")
	return Viper
}

func ReadConfig(callback func(v *viper.Viper)) {
	if callback != nil {
		v.AutomaticEnv()
		callback(v)
		v.ReadInConfig()
	}
}

func InitConfig() {
	ReadConfig(func(v *viper.Viper) {
		v.SetDefault("app.name", "app")
		v.SetDefault("app.debug", false)
		v.SetDefault("app.idle_timeout", 60)
		v.SetDefault("app.port", 3000)
		v.SetDefault("app.prefork", false)
		v.SetDefault("app.swagger", false)
		v.SetDefault("app.secret", "secret")
		v.SetDefault("app.login_expires", 60*60*24)     // 24小时
		v.SetDefault("app.refresh_expires", 60*60*24*7) // 7天
		v.SetDefault("app.body_limit", 10*1024*1024)    // 10MB
		// 缓存默认配置
		v.SetDefault("cache.default", "memory")
		v.SetDefault("cache.stores.memory.driver", "memory")
		v.SetDefault("cache.stores.memory.default_ttl", 3600)
		v.SetDefault("cache.stores.redis.driver", "redis")
		v.SetDefault("cache.stores.redis.default_ttl", 3600)

		// Redis默认配置
		v.SetDefault("redis.default", "redis")
		v.SetDefault("redis.connections.redis.addr", "localhost:6379")
		v.SetDefault("redis.connections.redis.username", "")
		v.SetDefault("redis.connections.redis.password", "")
		v.SetDefault("redis.connections.redis.db", 0)
		v.SetDefault("redis.connections.redis.dial_timeout", 5)
		v.SetDefault("redis.connections.redis.read_timeout", 3)
		v.SetDefault("redis.connections.redis.write_timeout", 3)
		v.SetDefault("redis.connections.redis.pool_size", 10)
		v.SetDefault("redis.connections.redis.min_idle_conns", 5)
		v.SetDefault("redis.connections.redis.max_idle_conns", 10)
		v.SetDefault("redis.connections.redis.conn_max_idle_time", 30)
		v.SetDefault("redis.connections.redis.conn_max_lifetime", 24)
		v.SetDefault("redis.connections.redis.use_tls", false)
		// 日志默认配置
		v.SetDefault("log.level", "info")
		v.SetDefault("log.format", "json")
		v.SetDefault("log.console_output", true)
		v.SetDefault("log.file_output", true)
		v.SetDefault("log.max_size", "10")
		v.SetDefault("log.max_backups", 10)
		v.SetDefault("log.max_age", 180)
		v.SetDefault("log.file_path", "./data/logs/app.log")
		v.SetDefault("database.default", "default")
		v.SetDefault("database.connections.default.driver", "mysql")
		v.SetDefault("database.connections.default.host", "localhost")
		v.SetDefault("database.connections.default.port", 3306)
		v.SetDefault("database.connections.default.user", "root")
		v.SetDefault("database.connections.default.password", "123456")
		v.SetDefault("database.connections.default.name", "cmf")
		v.SetDefault("database.connections.default.ssl_mode", "false")
		v.SetDefault("database.connections.default.table_prefix", "cmf_")
		v.SetDefault("database.connections.default.max_open_conns", 25)
		v.SetDefault("database.connections.default.max_idle_conns", 10)
		v.SetDefault("database.connections.default.conn_max_lifetime", 3600)
		v.SetDefault("database.connections.default.conn_max_idle_time", 600)

		v.SetDefault("filesystem.default", "local")
		v.SetDefault("filesystem.is_and_local", false)
		v.SetDefault("filesystem.disks.local.driver", "local")
		v.SetDefault("filesystem.disks.local.options.root", "./data/storage")
		v.SetDefault("filesystem.disks.s3.driver", "s3")
		v.SetDefault("filesystem.disks.s3.options.access_key", "")
		v.SetDefault("filesystem.disks.s3.options.secret_key", "")
		v.SetDefault("filesystem.disks.s3.options.region", "")
		v.SetDefault("filesystem.disks.s3.options.bucket", "")
		v.SetDefault("filesystem.disks.s3.options.endpoint", "")

		// Casbin默认配置
		v.SetDefault("casbin.default", "default")
		v.SetDefault("casbin.domains_default", "default")
		// 添加默认域配置
		defaultDomain := make(map[string]any)
		defaultDomain["name"] = "default"
		defaultDomain["auto_load"] = true
		defaultDomain["model_path"] = "./config/rbac_model.conf"
		v.SetDefault("casbin.domains", []map[string]any{defaultDomain})
	})
}

func initEnv() {
	filenames := []string{".env"}
	if os.Getenv("CMF_APP_ENV") == "development" {
		filenames = append(filenames, ".env.development")
	} else if os.Getenv("CMF_APP_ENV") == "production" {
		filenames = append(filenames, ".env.production")
	}
	godotenv.Load(filenames...)
}

func NewConfig() *Config {
	InitConfig()
	c := &Config{}
	// 将配置绑定到结构体
	v.Unmarshal(c)
	return c
}

func GetString(key string) string {
	return v.GetString(key)
}

func GetInt(key string) int {
	return v.GetInt(key)
}

func GetBool(key string) bool {
	return v.GetBool(key)
}

func (c *Config) GetString(key string) string {
	return GetString(key)
}

func (c *Config) GetInt(key string) int {
	return GetInt(key)
}

func (c *Config) GetBool(key string) bool {
	return GetBool(key)
}

func (c *Config) SaveConfig(section string, key string, value any, defaultValue any) error {
	return SaveConfig(v, section, key, value, defaultValue)
}

func SaveConfig(viper *viper.Viper, section string, key string, value any, defaultValue any) error {
	// 读取现有配置
	var config map[string]any
	viper.SetEnvKeyReplacer(nil)
	err := viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	// 如果指定的 section 不存在，则创建它
	if _, ok := config[section]; !ok {
		config[section] = make(map[string]any)
	}

	// 检查 section 是否是映射类型
	sectionMap, ok := config[section].(map[string]any)
	if !ok {
		// 如果不是映射类型，将其替换为新的映射
		sectionMap = make(map[string]any)
		config[section] = sectionMap
	}

	// 使用默认值如果用户没有提供新值
	if value == nil {
		value = defaultValue
	}

	// 更新或添加键值对
	sectionMap[key] = value

	// 将更新后的配置写回文件
	viper.Set(section, config[section])
	if err := viper.WriteConfig(); err != nil {
		return err
	}

	return nil
}
