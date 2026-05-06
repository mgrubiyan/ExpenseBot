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

	amountFloat, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return "", 0, fmt.Errorf("amount must be a number")
	}
	if amountFloat <= 0 {
		return "", 0, fmt.Errorf("amount must be greater than zero")
	}

	// Store as kopecks to avoid floating point issues
	amountKopecks := int64(amountFloat * 100)

	return tag, amountKopecks, nil
}

func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, st storage.Storage) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := strings.TrimSpace(update.Message.Text)

	send := func(reply string) {
		msg := tgbotapi.NewMessage(chatID, reply)
		msg.ReplyToMessageID = update.Message.MessageID
		msg.ParseMode = "Markdown"
		if _, err := bot.Send(msg); err != nil {
			log.Println("send error:", err)
		}
	}

	switch {
	case text == "/start":
		send("Привет! 👋 Я помогу отслеживать расходы.\n\n" +
			"*Добавить трату:* `еда 450`\n" +
			"*Статистика за месяц:* /month\n" +
			"*Справка:* /help")

	case text == "/help":
		send("*Как добавить трату:*\n" +
			"`<категория> <сумма>`\n\n" +
			"Примеры:\n" +
			"`еда 450`\n" +
			"`транспорт 120`\n" +
			"`кофе 4.5`\n\n" +
			"*Команды:*\n" +
			"/month — статистика за текущий месяц\n" +
			"/week — статистика за 7 дней")

	case text == "/month":
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to := now

		expenses, err := st.GetExpensesByPeriod(userID, from, to)
		if err != nil {
			send("Трат за этот месяц пока нет.")
			return
		}
		send(formatStats(expenses, "месяц"))

	case text == "/week":
		from := time.Now().AddDate(0, 0, -7)
		to := time.Now()

		expenses, err := st.GetExpensesByPeriod(userID, from, to)
		if err != nil {
			send("Трат за последние 7 дней пока нет.")
			return
		}
		send(formatStats(expenses, "7 дней"))

	default:
		tag, amount, err := parseExpenseInput(text)
		if err != nil {
			send("Неверный формат. Используй: `еда 450` или /help")
			return
		}

		expense := models.Expense{
			UserID:    userID,
			Tag:       tag,
			Amount:    int(amount),
			CreatedAt: time.Now(),
		}

		if err := st.AddExpense(expense); err != nil {
			log.Println("storage error:", err)
			send(fmt.Sprintf("Ошибка сохранения: %v", err))
			return
		}

		send(fmt.Sprintf("✅ Сохранил: *%s* — %.2f ₽", tag, float64(amount)/100))
	}
}

func formatStats(expenses []models.Expense, period string) string {
	tagTotals := make(map[string]int)
	var total int
	for _, e := range expenses {
		tagTotals[e.Tag] += e.Amount
		total += e.Amount
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Расходы за %s:*\n\n", period))
	for tag, amount := range tagTotals {
		sb.WriteString(fmt.Sprintf("• %s — %.2f ₽\n", tag, float64(amount)/100))
	}
	sb.WriteString(fmt.Sprintf("\n*Итого: %.2f ₽*", float64(total)/100))
	return sb.String()
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("no .env file found, reading env vars directly")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/expenses.db"
	}

	st, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		log.Fatal("failed to init storage:", err)
	}
	defer st.Close()

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = os.Getenv("BOT_DEBUG") == "true"

	log.Printf("authorized as @%s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	for update := range bot.GetUpdatesChan(updateConfig) {
		handleUpdate(bot, update, st)
	}
}
