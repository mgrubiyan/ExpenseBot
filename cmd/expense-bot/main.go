package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"ExpenseBot/internal/models"
	"ExpenseBot/internal/storage"
)

func parseExpenseInput(text string) (string, int64, error) {
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("format must be: <tag> <amount>")
	}

	tag := parts[0]

	amountRub, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("amount must be a number")
	}

	if amountRub <= 0 {
		return "", 0, fmt.Errorf("amount must be greater than zero")
	}

	amountKopecks := int64(amountRub * 100)

	return tag, amountKopecks, nil
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("failed to load .env file")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	st := storage.NewJSONStorage("data/expenses.json")

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("authorized as @%s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		text := update.Message.Text

		tag, amount, err := parseExpenseInput(text)
		if err != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный формат. Используй: еда 450")
			msg.ReplyToMessageID = update.Message.MessageID
			if _, sendErr := bot.Send(msg); sendErr != nil {
				log.Println("send error:", sendErr)
			}
			continue
		}

		expense := models.Expense{
			UserID:    update.Message.From.ID,
			Tag:       tag,
			Amount:    int(amount),
			CreatedAt: time.Now(),
		}

		if err := st.AddExpense(expense); err != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Ошибка сохранения: %v", err))
			log.Println("storage error:", err)
			msg.ReplyToMessageID = update.Message.MessageID
			if _, sendErr := bot.Send(msg); sendErr != nil {
				log.Println("send error:", sendErr)
			}
			continue
		}

		replyText := fmt.Sprintf("Сохранил: %s — %d ₽", tag, amount/100)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText)
		msg.ReplyToMessageID = update.Message.MessageID

		if _, err := bot.Send(msg); err != nil {
			log.Println("send error:", err)
		}
	}
}
