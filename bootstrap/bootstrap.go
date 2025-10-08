package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/wuwuseo/cmf/config"
)

type Bootstrap struct {
	Config *config.Config
}

func NewBootstrap() *Bootstrap {
	config.InitConfig()
	return &Bootstrap{
		Config: config.NewConfig(),
	}
}

func (b *Bootstrap) Run() {
	app := fiber.New(fiber.Config{
		IdleTimeout: time.Duration(b.Config.App.IdleTimeout) * time.Second,
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello world!")
	})

	// Listen from a different goroutine
	go func() {
		if err := app.Listen(":" + fmt.Sprint(b.Config.App.Port)); err != nil {
			log.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Warnf("Gracefully shutting down...")
	_ = app.Shutdown()

	log.Warnf("Running cleanup tasks...")

	// Your cleanup tasks go here
	// db.Close()
	// redisConn.Close()
	log.Warnf("Fiber was successful shutdown.")
}
