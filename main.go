package main

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/wuwuseo/cmf/global"
	"github.com/wuwuseo/cmf/pkg/config"
	"github.com/wuwuseo/cmf/pkg/logger"
	"github.com/wuwuseo/cmf/pkg/model"
	"github.com/wuwuseo/cmf/router"
	"log"
)

func init() {
	// 初始化配置
	err := config.NewConfig()
	if err != nil {
		log.Fatalf("init.NewConfig err: %v", err)
	}
	//初始化 日志
	global.Log = logger.NewLog()
	err = setupDBEngine()
	if err != nil {
		log.Fatalf("init.setupDBEngine err: %v", err)
	}
}

func setupDBEngine() error {
	var err error
	global.DBEngine, err = model.NewDBEngine(global.DatabaseConfig)
	if err != nil {
		return err
	}
	return nil
}

//go:generate go mod tidy
//go:generate go build .
func main() {
	port := global.ServerConfig.Http.Port
	addr := fmt.Sprintf(":%d", port)
	preFork := global.ServerConfig.Prefork
	defer global.Log.Sync()
	// Fiber instance
	app := fiber.New(fiber.Config{
		Prefork:      preFork,
		ServerHeader: "Fiber",
	})
	global.Log.Info("app init")
	// Routes
	router.NewRouter(app)
	global.Log.Info("app router init")
	// Start server
	err := app.Listen(addr)
	global.Log.Fatal(err.Error())
	// Run the following command to see all processes sharing port 3000:
	// sudo lsof -i -P -n | grep LISTEN
}
