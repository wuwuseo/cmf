package config

import "github.com/spf13/viper"

type Config struct {
	App struct {
		Name  string `mapstructure:"name"`
		Port  int    `mapstructure:"port"`
		Debug bool   `mapstructure:"debug"`
	} `mapstructure:"app"`
}

func InitConfig() {
	// 初始化配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.SetDefault("app.name", "app")
	viper.SetDefault("app.debug", false)

}

func NewConfig() *Config {
	return &Config{}
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
