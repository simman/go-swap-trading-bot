package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/simman/go-swap-trading-bot/config"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/simman/go-swap-trading-bot/bot"
	"github.com/simman/go-swap-trading-bot/dao"
	"github.com/simman/go-swap-trading-bot/global"
	"github.com/simman/go-swap-trading-bot/utils"
	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
)

var actionkeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Start"),
		tgbotapi.NewKeyboardButton("Stop"),
		tgbotapi.NewKeyboardButton("Status"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Record"),
		tgbotapi.NewKeyboardButton("Balance"),
		tgbotapi.NewKeyboardButton("MSC"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("BSCScan"),
		tgbotapi.NewKeyboardButton("Config"),
		tgbotapi.NewKeyboardButton("Sponsor"),
	),
)

type Bot struct {
	ctx context.Context
	bot *tgbotapi.BotAPI
}

func NewBot(context context.Context) *Bot {
	return &Bot{
		ctx: context,
	}
}

func (b *Bot) Start() {
	token := viper.GetString("telegram.token")
	if funk.IsEmpty(token) {
		log.Println("telegram token is empty!")
		return
	}

	if bot, err := tgbotapi.NewBotAPI(token); err != nil {
		log.Fatal(err)
	} else {
		b.bot = bot
	}

	b.bot.Debug = viper.GetBool("telegram.debug")

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = viper.GetInt("telegram.timeout")
	updates := b.bot.GetUpdatesChan(updateConfig)

	b.poolMessage(updates)
}

// func (b *Bot) handleMessage(update *tgbotapi.Update, msg *tgbotapi.MessageConfig) {
// 	switch update.Message.Text {
// 	case "/start":
// 		msg.Text = "Please choose action"
// 		msg.ReplyMarkup = actionkeyboard
// 	case "Config":
// 		jsonStr, err := json.Marshal(viper.GetStringMap("strtegy"))
// 		if err == nil {
// 			msg.Text = utils.JsonPrettyPrint(string(jsonStr))
// 		} else {
// 			msg.Text = err.Error()
// 		}
// 	case "Sponsor":
// 		msg.Text = "Please allow me to express my gratitude to you, for the decision of your sponsorship and let you know the sponsorship has helped us make our cause speak out greater audience\n\nBinance Smart Chain: 0xC28C7bCaa85c6cf1B866B696E0598826c231A2a9"
// 	case "close":
// 		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
// 	}
// }

func (b *Bot) poolMessage(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		if update.Message == nil {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)

		managerUserId := viper.GetIntSlice("telegram.managerUserId")

		tradingBot := global.TradingBot.(*bot.Bot)

		if funk.Contains(managerUserId, int(update.Message.From.ID)) {
			// msg.ReplyToMessageID = update.Message.MessageID
			// b.handleMessage(&update, &msg)
			switch update.Message.Text {
			case "/start":
				msg.Text = "Please choose action"
				msg.ReplyMarkup = actionkeyboard
			case "Start":
				if !tradingBot.Running() {
					tradingBot.Start()
					msg.Text = "Start TradingBot Success"
				} else {
					msg.Text = "TradingBot already started"
				}
			case "Stop":
				if tradingBot.Running() {
					tradingBot.Stop()
					msg.Text = "Stop TradingBot Success"
				} else {
					msg.Text = "TradingBot already stoped"
				}
			case "Status":
				msg.Text = fmt.Sprintf("TradingBot Status: %v", tradingBot.Running())
			case "Record":
				msgContent := ""
				if records, err := dao.RecordDao.QueryLastDayListMap(); err == nil {
					for _, v := range records {
						ai, _ := strconv.ParseFloat(v["AmountIn"].(string), 64)
						msgContent += fmt.Sprintf("AD: %s, MscPrice: %f, AmountIn, %f, Status: %d, Time: %s \n", v["Address"], v["MscPrice"], ai, int64(v["Status"].(float64)), time.Unix(int64(v["CreateTime"].(float64)), 0).Format("2006-01-02 15:04:05"))
					}
				}
				msg.Text = msgContent
			case "Balance":
				balance := tradingBot.Balance()
				msg.Text = fmt.Sprintf("Balance, BNB: %f - MSC: %f - USDT: %f", utils.WeiToEther(balance.BNB), utils.WeiToEther(balance.MSC), utils.WeiToEther(balance.USDT))
			case "MSC":
				msg.Text = fmt.Sprintf("Current MSC Price: %f USDT", tradingBot.MscLastPrice())
			case "BSCScan":
				msg.Text = fmt.Sprintf("https://bscscan.com/address/%s", tradingBot.WalletAddress())
			case "Config":
				jsonStr, err := json.Marshal(config.StrtegyConfig)
				if err == nil {
					msg.Text = utils.JsonPrettyPrint(string(jsonStr))
				} else {
					msg.Text = err.Error()
				}
			case "Sponsor":
				msg.Text = "Please allow me to express my gratitude to you, for the decision of your sponsorship and let you know the sponsorship has helped us make our cause speak out greater audience\nBinance Smart Chain: 0xC28C7bCaa85c6cf1B866B696E0598826c231A2a9"
			case "close":
				msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			}
		} else {
			msg.Text = fmt.Sprintf("No permissions, if you are the administrator, please add %d to the telegram. ManagerUserId configuration in env.yaml", update.Message.From.ID)
		}

		if _, err := b.bot.Send(msg); err != nil {
			panic(err)
		}
	}
}
