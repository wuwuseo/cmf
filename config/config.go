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
	} `mapstructure:"app"`
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
