package model

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/wuwuseo/cmf/global"
	"github.com/wuwuseo/cmf/pkg/setting"
)

type Model struct {
}

func NewDBEngine(databaseSetting *setting.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(databaseSetting.DBType, fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=Local",
		databaseSetting.UserName,
		databaseSetting.Password,
		databaseSetting.Host,
		databaseSetting.DBName,
		databaseSetting.Charset,
		databaseSetting.ParseTime,
	))
	if err != nil {
		return nil, err
	}

	if global.AppConfig.Debug {
		db.LogMode(true)
	}
	db.SingularTable(true) //设置全局表名禁用复数
	db.DB().SetMaxIdleConns(databaseSetting.MaxIdleConns)
	db.DB().SetMaxOpenConns(databaseSetting.MaxOpenConns)
	//指定表前缀，修改默认表名
	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		return databaseSetting.TablePrefix + defaultTableName
	}

	return db, nil
}
