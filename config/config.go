package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name        string `mapstructure:"name"`
		Port        int    `mapstructure:"port"`
		Debug       bool   `mapstructure:"debug"`
		IdleTimeout int    `mapstructure:"idle_timeout"`
		Prefork     bool   `mapstructure:"prefork"`
		Swagger     bool   `mapstructure:"swagger"`
	} `mapstructure:"app"`

	Log struct {
		FilePath      string `mapstructure:"file_path"`
		ConsoleOutput bool   `mapstructure:"console_output"`
		FileOutput    bool   `mapstructure:"file_output"`
		MaxSize       int    `mapstructure:"max_size"`
		MaxBackups    int    `mapstructure:"max_backups"`
		MaxAge        int    `mapstructure:"max_age"`
	} `mapstructure:"log"`

	Database struct {
		Driver      string `mapstructure:"driver"`
		Host        string `mapstructure:"host"`
		Port        int    `mapstructure:"port"`
		User        string `mapstructure:"user"`
		Password    string `mapstructure:"password"`
		Name        string `mapstructure:"name"`
		SSLMode     string `mapstructure:"ssl_mode"`
		TablePrefix string `mapstructure:"table_prefix"`
	} `mapstructure:"database"`

	// 缓存配置
	Cache struct {
		Driver        string `mapstructure:"driver"`        // 缓存驱动类型，如 bigcache, memory
		DefaultTTL    int    `mapstructure:"default_ttl"`   // 默认缓存过期时间（秒）
		Size          int    `mapstructure:"size"`          // 缓存大小
		CleanWindow   int    `mapstructure:"clean_window"`  // 清理窗口（秒）
		HardMaxCacheSize int `mapstructure:"hard_max_cache_size"` // 最大缓存大小（MB）
	} `mapstructure:"cache"`

	// Redis配置
	Redis struct {
		Addr               string `mapstructure:"addr"`                // Redis服务器地址，格式为"host:port"
		Password           string `mapstructure:"password"`            // Redis密码，无密码时为空字符串
		DB                 int    `mapstructure:"db"`                  // Redis数据库索引
		DialTimeout        int    `mapstructure:"dial_timeout"`        // 连接超时时间（秒）
		ReadTimeout        int    `mapstructure:"read_timeout"`        // 读取超时时间（秒）
		WriteTimeout       int    `mapstructure:"write_timeout"`       // 写入超时时间（秒）
		PoolSize           int    `mapstructure:"pool_size"`           // 连接池大小
		MinIdleConns       int    `mapstructure:"min_idle_conns"`      // 最小空闲连接数
		MaxIdleConns       int    `mapstructure:"max_idle_conns"`      // 最大空闲连接数
		ConnMaxIdleTime    int    `mapstructure:"conn_max_idle_time"`   // 连接最大空闲时间（分钟）
		ConnMaxLifetime    int    `mapstructure:"conn_max_lifetime"`   // 连接最大生命周期（小时）
		UseTLS             bool   `mapstructure:"use_tls"`             // 是否使用TLS加密连接
	} `mapstructure:"redis"`
	
}

var v = NewViper("config")

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
	Viper.SetConfigType("yaml")
	Viper.AddConfigPath("./config")
	return Viper
}

func InitConfig() {
	filenames := []string{".env"}
	if os.Getenv("CMF_APP_ENV") == "development" {
		filenames = append(filenames, ".env.development")
	} else if os.Getenv("CMF_APP_ENV") == "production" {
		filenames = append(filenames, ".env.production")
	}
	godotenv.Load(filenames...)
	v.AutomaticEnv()
	v.SetDefault("app.name", "app")
	v.SetDefault("app.debug", false)
	v.SetDefault("app.idle_timeout", 60)
	v.SetDefault("app.port", 3000)
	v.SetDefault("app.prefork", false)
	v.SetDefault("app.swagger", false)
	// 缓存默认配置
	v.SetDefault("cache.driver", "bigcache")
	v.SetDefault("cache.default_ttl", 3600)  // 默认1小时
	v.SetDefault("cache.size", 10000)       // 默认10000个元素
	v.SetDefault("cache.clean_window", 5)   // 默认5秒清理一次
	v.SetDefault("cache.hard_max_cache_size", 1024) // 默认1GB

	// Redis默认配置
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", 5)
	v.SetDefault("redis.read_timeout", 3)
	v.SetDefault("redis.write_timeout", 3)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.max_idle_conns", 10)
	v.SetDefault("redis.conn_max_idle_time", 30)
	v.SetDefault("redis.conn_max_lifetime", 24)
	v.SetDefault("redis.use_tls", false)
	// 日志默认配置
	v.SetDefault("log.console_output", true)
	v.SetDefault("log.file_output", true)
	v.SetDefault("log.max_size", "10")
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.max_age", 180)
	v.SetDefault("log.file_path", "./data/logs/app.log")
	v.SetDefault("database.driver", "mysql")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 3306)
	v.SetDefault("database.user", "root")
	v.SetDefault("database.password", "123456")
	v.SetDefault("database.name", "cmf")
	v.SetDefault("database.ssl_mode", "false")
	v.SetDefault("database.table_prefix", "cmf_")
	v.ReadInConfig()
}

func NewConfig() *Config {
	c := &Config{}
	// 将配置绑定到结构体
	v.Unmarshal(c)
	return c
}

func (c *Config) GetString(key string) string {
	return v.GetString(key)
}

func (c *Config) GetInt(key string) int {
	return v.GetInt(key)
}

func (c *Config) GetBool(key string) bool {
	return v.GetBool(key)
}

func (c *Config) SaveConfig(section string, key string, value any, defaultValue any) error {
	// 读取现有配置
	var config map[string]any
	v.SetEnvKeyReplacer(nil)
	err := v.Unmarshal(&config)
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
	v.Set(section, config[section])
	if err := v.WriteConfig(); err != nil {
		return err
	}

	return nil
}
