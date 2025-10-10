package orm

import (
	"fmt"

	"github.com/wuwuseo/cmf/config"
)

func GetDatabaseSource(config *config.Config) string {
	switch config.Database.Driver {
	case "postgres":
		return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s", config.Database.User, config.Database.Password, config.Database.Host, config.Database.Port, config.Database.Name)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", config.Database.User, config.Database.Password, config.Database.Host, config.Database.Port, config.Database.Name)
	}
	return ""
}
