package main

import (
	"fmt"
	"log"
	"os"
	"sort"
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
	amountKopecks := int64(amountFloat * 100)
	return tag, amountKopecks, nil
}

func formatStats(expenses []models.Expense, period string) string {
	totals := make(map[string]int)
	counts := make(map[string]int)
	var total int

	for _, e := range expenses {
		totals[e.Tag] += e.Amount
		total += e.Amount
		counts[e.Tag]++
	}

	tags := make([]string, 0, len(totals))
	for tag := range totals {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		return totals[tags[i]] > totals[tags[j]]
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Расходы за %s:\n\n", period))
	for _, tag := range tags {
		sb.WriteString(fmt.Sprintf(fmt.Sprintf("• %s — %.2f ₽ (%d)\n", tag, float64(totals[tag])/100, counts[tag])))
	}
	sb.WriteString(fmt.Sprintf(fmt.Sprintf("\nИтого: %.2f ₽", float64(total)/100)))

	return sb.String()
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("failed to load .env file")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	st, err := storage.NewSQLiteStorage("data/expenses.db")
	if err != nil {
		log.Fatal("failed to init storage:", err)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("authorized as @%s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	for update := range bot.GetUpdatesChan(updateConfig) {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID
		text := update.Message.Text

		send := func(reply string) {
			msg := tgbotapi.NewMessage(chatID, reply)
			msg.ReplyToMessageID = update.Message.MessageID
			if _, err := bot.Send(msg); err != nil {
				log.Println("send error:", err)
			}
		}

		switch text {
		case "/month":
			now := time.Now()
			from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

			expenses, err := st.GetExpensesByPeriod(userID, from, now)
			if err != nil || len(expenses) == 0 {
				send("Трат за этот месяц пока нет.")
				continue
			}
			send(formatStats(expenses, "текущий месяц"))

		case "/week":
			from := time.Now().AddDate(0, 0, -7)

			expenses, err := st.GetExpensesByPeriod(userID, from, time.Now())
			if err != nil || len(expenses) == 0 {
				send("Трат за последние 7 дней пока нет.")
				continue
			}
			send(formatStats(expenses, "7 дней"))

		case "/start":
			send("Привет! 👋\n\n" +
				"Я помогу тебе контролировать расходы и чувствовать себя увереннее в финансах.\n\n" +
				"Чтобы добавить трату, просто напиши:\n" +
				"еда 450\n" +
				"транспорт 120\n" +
				"кофе 4.5\n\n" +
				"Если что — пиши /help")

		case "/help":
			send("Как добавить трату:\n" +
				"<категория> <сумма>\n\n" +
				"Примеры:\n" +
				"еда 450\n" +
				"транспорт 120\n" +
				"кофе 4.5\n\n" +
				"Команды:\n" +
				"/month — расходы за текущий месяц\n" +
				"/week — расходы за 7 дней\n" +
				"/help — эта справка")

		default:
			tag, amount, err := parseExpenseInput(text)
			if err != nil {
				msg := tgbotapi.NewMessage(chatID, "Неверный формат. Используй: еда 450")
				msg.ReplyToMessageID = update.Message.MessageID
				if _, sendErr := bot.Send(msg); sendErr != nil {
					log.Println("send error:", sendErr)
				}
				continue
			}

			expense := models.Expense{
				UserID:    userID,
				Tag:       tag,
				Amount:    int(amount),
				CreatedAt: time.Now(),
			}

			if err := st.AddExpense(expense); err != nil {
				send(fmt.Sprintf("Ошибка сохранения: %v", err))
				log.Println("storage error:", err)
				continue
			}

			send(fmt.Sprintf("Сохранил: %s — %.2f ₽", tag, float64(amount)/100))
		}
	}
}
