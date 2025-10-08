package config

import "github.com/spf13/viper"

type Config struct {
	App struct {
		Name        string `mapstructure:"name"`
		Port        int    `mapstructure:"port"`
		Debug       bool   `mapstructure:"debug"`
		IdleTimeout int    `mapstructure:"idle_timeout"`
	} `mapstructure:"app"`
}

func InitConfig() {
	// 初始化配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.SetDefault("app.name", "app")
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.idle_timeout", 60)
	viper.SetDefault("app.port", 3000)
	viper.ReadInConfig()
}

func NewConfig() *Config {
	c := &Config{}
	// 将配置绑定到结构体
	viper.Unmarshal(c)
	return c
}

func (c *Config) GetString(key string) string {
	return viper.GetString(key)
}

func (c *Config) GetInt(key string) int {
	return viper.GetInt(key)
}

func (c *Config) GetBool(key string) bool {
	return viper.GetBool(key)
}
