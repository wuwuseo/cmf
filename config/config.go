package config

import (
	"os"
	"strings"

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

// StoreConfig 缓存存储配置
// 使用类型别名（而非新定义类型），保证既有的匿名结构体字面量赋值完全兼容
type StoreConfig = struct {
	Driver     string `mapstructure:"driver"`      // 缓存驱动类型，如 bigcache, memory
	DefaultTTL int    `mapstructure:"default_ttl"` // 默认缓存过期时间（秒）
	Options    any    `mapstructure:"options"`     // 缓存驱动选项
}

// DiskConfig 文件系统磁盘配置
// 使用类型别名（而非新定义类型），保证既有的匿名结构体字面量赋值完全兼容
type DiskConfig = struct {
	Driver  string `mapstructure:"driver"`  // 存储驱动类型，如 local, s3
	Options any    `mapstructure:"options"` // 驱动选项
}

type Config struct {
	App struct {
		Name                string `mapstructure:"name"`
		Port                int    `mapstructure:"port"`
		Debug               bool   `mapstructure:"debug"`
		IdleTimeout         int    `mapstructure:"idle_timeout"`
		Prefork             bool   `mapstructure:"prefork"`
		Swagger             bool   `mapstructure:"swagger"`
		Secret              string `mapstructure:"secret"`
		LoginExpires        int    `mapstructure:"login_expires"`
		RefreshExpires      int    `mapstructure:"refresh_expires"`
		BodyLimit           int    `mapstructure:"body_limit"`
		AdminSecret         string `mapstructure:"admin_secret"`
		AdminLoginExpires   int    `mapstructure:"admin_login_expires"`
		AdminRefreshExpires int    `mapstructure:"admin_refresh_expires"`
	} `mapstructure:"app"`

	Attachment struct {
		PublicBaseURL string `mapstructure:"public_base_url"`
		XAccelEnabled bool   `mapstructure:"x_accel_enabled"`
		XAccelPrefix  string `mapstructure:"x_accel_prefix"`
		AccessURLTTL  int    `mapstructure:"access_url_ttl"`
	} `mapstructure:"attachment"`

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
		Default string                 `mapstructure:"default"` // 默认缓存存储
		Stores  map[string]StoreConfig `mapstructure:"stores"`
	} `mapstructure:"cache"`

	Redis struct {
		Default     string           `mapstructure:"default"`
		Connections map[string]Redis `mapstructure:"connections"`
	} `mapstructure:"redis"`

	Captcha struct {
		Store           string `mapstructure:"store"`
		RedisConnection string `mapstructure:"redis_connection"`
		KeyPrefix       string `mapstructure:"key_prefix"`
	} `mapstructure:"captcha"`

	// 任务队列配置：Redis 连接参数复用 redis.connections 中的命名连接，
	// 运行参数（并发、队列优先级、重试等）见 queue 包的 Config。
	Queue struct {
		Driver          string         `mapstructure:"driver"`            // asynq (default), rabbitmq, nats, nsq, kafka, amqp10
		Namespace       string         `mapstructure:"namespace"`         // 隔离新后端的资源名
		RedisConnection string         `mapstructure:"redis_connection"`  // 引用 redis.connections 中的连接名
		Concurrency     int            `mapstructure:"concurrency"`       // 并发 worker 数，0 表示取 CPU 核数
		Queues          map[string]int `mapstructure:"queues"`            // 消费的队列及优先级权重，缺省仅 default
		StrictPriority  bool           `mapstructure:"strict_priority"`   // 严格优先级模式
		MaxRetry        int            `mapstructure:"max_retry"`         // 默认最大重试次数
		Timeout         int            `mapstructure:"timeout"`           // 单任务默认处理超时（秒），0 不限制
		Retention       int            `mapstructure:"retention"`         // completed 保留时长（秒），0 完成即删
		MaxQueueSize    int            `mapstructure:"max_queue_size"`    // 每队列存量任务准入阈值，包含保留的历史
		MaxPayloadBytes int            `mapstructure:"max_payload_bytes"` // 单任务载荷字节上限
		EnableMonitor   bool           `mapstructure:"enable_monitor"`    // 是否挂载 asynqmon 只读监控页
		MonitorPath     string         `mapstructure:"monitor_path"`      // 监控页挂载路径
		RabbitMQ        struct {
			URL           string `mapstructure:"url"`
			ManagementURL string `mapstructure:"management_url"`
		} `mapstructure:"rabbitmq"`
		NATS struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"nats"`
		NSQ struct {
			NSQD    string `mapstructure:"nsqd"`
			Lookupd string `mapstructure:"lookupd"`
			HTTP    string `mapstructure:"http"`
		} `mapstructure:"nsq"`
		Kafka struct {
			Brokers           []string `mapstructure:"brokers"`
			Group             string   `mapstructure:"group"`
			Partitions        int32    `mapstructure:"partitions"`
			ReplicationFactor int16    `mapstructure:"replication_factor"`
		} `mapstructure:"kafka"`
		AMQP10 struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"amqp10"`
	} `mapstructure:"queue"`

	Filesystem struct {
		Default    string                `mapstructure:"default"`
		IsAndLocal bool                  `mapstructure:"is_and_local"` // 是否同时存储在本地文件系统
		Disks      map[string]DiskConfig `mapstructure:"disks"`
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
	Viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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
		v.SetDefault("attachment.public_base_url", "/uploads")
		v.SetDefault("attachment.x_accel_enabled", true)
		v.SetDefault("attachment.x_accel_prefix", "/_protected_attachments")
		v.SetDefault("attachment.access_url_ttl", 600)
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

		v.SetDefault("captcha.store", "memory")
		v.SetDefault("captcha.redis_connection", "redis")
		v.SetDefault("captcha.key_prefix", "captcha")

		// 任务队列默认配置
		v.SetDefault("queue.redis_connection", "redis")
		v.SetDefault("queue.concurrency", 10)
		v.SetDefault("queue.queues.default", 10)
		v.SetDefault("queue.strict_priority", false)
		v.SetDefault("queue.max_retry", 3)
		v.SetDefault("queue.timeout", 600)
		v.SetDefault("queue.retention", 0)
		v.SetDefault("queue.max_queue_size", 10000)
		v.SetDefault("queue.max_payload_bytes", 65536)
		v.SetDefault("queue.enable_monitor", false)
		v.SetDefault("queue.monitor_path", "/api/v1/admin/auth/queue-monitor")

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

func SaveConfig(runtime *viper.Viper, section string, key string, value any, defaultValue any) error {
	// Read only the file: environment secrets and runtime overrides must never
	// become persisted configuration when an unrelated setting is changed.
	fileConfig := viper.New()
	fileConfig.SetConfigFile(runtime.ConfigFileUsed())
	if err := fileConfig.ReadInConfig(); err != nil {
		return err
	}
	if value == nil {
		value = defaultValue
	}
	fileConfig.Set(section+"."+key, value)
	if err := fileConfig.WriteConfig(); err != nil {
		return err
	}
	return runtime.ReadInConfig()
}
