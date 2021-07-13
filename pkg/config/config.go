package config

import (
	"fmt"
	"github.com/wuwuseo/cmf/global"
	"log"

	"github.com/spf13/viper"
)

func NewConfig() error {
	viper.AutomaticEnv()
	viper.SetConfigFile("config.yml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	err = setupSetting()
	if err != nil {
		log.Fatalf("init.setupSetting err: %v", err)
	}
	return nil
}

func setupSetting() error {
	err := ReadConfig("Server", &global.ServerConfig)
	if err != nil {
		return err
	}
	err = ReadConfig("App", &global.AppConfig)
	if err != nil {
		return err
	}
	err = ReadConfig("Database", &global.DatabaseConfig)
	if err != nil {
		return err
	}
	return nil

}

func ReadConfig(k string, v interface{}) error {
	err := viper.UnmarshalKey(k, v)
	if err != nil {
		return err
	}
	return nil
}
