package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/simman/go-swap-trading-bot/bot"
	"github.com/simman/go-swap-trading-bot/config"
	"github.com/simman/go-swap-trading-bot/global"
	"github.com/simman/go-swap-trading-bot/telegram"
	"github.com/spf13/viper"

	c "github.com/ostafen/clover"
)

func main() {
	config.InitConfig()

	// Custom config
	app := fiber.New(fiber.Config{
		ServerHeader: "go-swap-trading-bot",
		AppName:      "go-swap-trading-bot v1.0.1",
	})

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// create database
	db, _ := c.Open("tmp")
	defer db.Close()
	db.CreateCollection("record")
	db.CreateCollection("strtegy")
	global.DB = db

	// create bot
	global.TelegramBot = telegram.NewBot(ctx)
	global.TradingBot = bot.NewBot(ctx)

	// start bot
	go global.TelegramBot.(*telegram.Bot).Start()
	global.TradingBot.(*bot.Bot).Start()

	port := fmt.Sprintf(":%d", viper.GetInt("server.port"))
	log.Fatal(app.Listen(port))
}
