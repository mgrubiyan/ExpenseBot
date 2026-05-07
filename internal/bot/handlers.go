package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"ExpenseBot/internal/models"
	"ExpenseBot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	storage storage.Storage
}

func NewHandler(st storage.Storage) *Handler {
	return &Handler{storage: st}
}

func (h *Handler) HandleUpdate(api *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := update.Message.Text

	send := func(reply string) {
		msg := tgbotapi.NewMessage(chatID, reply)
		msg.ReplyToMessageID = update.Message.MessageID
		if _, err := api.Send(msg); err != nil {
			log.Println("send error:", err)
		}
	}

	switch text {
	case "/start":
		send("Привет! 👋\n\n" +
			"Я помогу тебе контролировать расходы и чувствовать себя увереннее в финансах.\n\n" +
			"Чтобы добавить трату, просто напиши:\n" +
			"еда 450\n" +
			"транспорт 120\n" +
			"кофе 4.5\n\n" +
			"Команды:\n" +
			"/today — расходы за сегодня\n" +
			"/week — расходы за 7 дней\n" +
			"/month — расходы за текущий месяц\n" +
			"/l5 — последние 5 трат\n" +
			"/help — справка\n" +
			"/del — удалить последнюю трату")

	case "/help":
		send("Как добавить трату:\n" +
			"<категория> <сумма>\n\n" +
			"Примеры:\n" +
			"еда 450\n" +
			"транспорт 120\n" +
			"кофе 4.5\n\n" +
			"Команды:\n" +
			"/today — расходы за сегодня\n" +
			"/week — расходы за 7 дней\n" +
			"/month — расходы за текущий месяц\n" +
			"/l5 — последние 5 трат\n" +
			"/help — эта справка\n" +
			"/del — удалить последнюю трату")

	case "/month":
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		expenses, err := h.storage.GetExpensesByPeriod(userID, from, now)
		if err != nil {
			send("Не удалось получить расходы за месяц.")
			log.Println("get month expenses error:", err)
			return
		}
		if len(expenses) == 0 {
			send("Трат за этот месяц пока нет.")
			return
		}

		send(models.FormatStats(expenses, "текущий месяц"))

	case "/week":
		now := time.Now()
		from := now.AddDate(0, 0, -7)

		expenses, err := h.storage.GetExpensesByPeriod(userID, from, now)
		if err != nil {
			send("Не удалось получить расходы за последние 7 дней.")
			log.Println("get week expenses error:", err)
			return
		}
		if len(expenses) == 0 {
			send("Трат за последние 7 дней пока нет.")
			return
		}

		send(models.FormatStats(expenses, "7 дней"))

	case "/today":
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		expenses, err := h.storage.GetExpensesByPeriod(userID, from, now)
		if err != nil {
			send("Не удалось получить расходы за сегодня.")
			log.Println("get today expenses error:", err)
			return
		}
		if len(expenses) == 0 {
			send("Трат за сегодня пока нет.")
			return
		}

		send(models.FormatStats(expenses, "сегодня"))

	case "/l5":
		expenses, err := h.storage.GetLastExpenses(userID, 5)
		if err != nil {
			send("Не удалось получить последние траты.")
			log.Println("get last expenses error:", err)
			return
		}
		if len(expenses) == 0 {
			send("У тебя пока нет сохранённых трат.")
			return
		}

		var sb strings.Builder
		sb.WriteString("Последние 5 трат:\n\n")

		for i, e := range expenses {
			sb.WriteString(fmt.Sprintf(
				"%d. %s — %.2f ₽ (%s)\n",
				i+1,
				e.Tag,
				float64(e.Amount)/100,
				e.CreatedAt.Format("02.01 15:04"),
			))
		}

		send(sb.String())

	case "/del":
		expense, err := h.storage.DeleteLastExpense(userID)
		if err != nil {
			send("Не удалось удалить последнюю трату.")
			log.Println("delete last expense error:", err)
			return
		}
		if expense == nil {
			send("У тебя пока нет трат для удаления.")
			return
		}

		send(fmt.Sprintf("Удалил: %s — %.2f ₽", expense.Tag, float64(expense.Amount)/100))

	default:
		tag, amount, err := models.ParseExpenseInput(text)
		if err != nil {
			send("Неверный формат. Используй: еда 450")
			return
		}

		expense := models.Expense{
			UserID:    userID,
			Tag:       tag,
			Amount:    amount,
			CreatedAt: time.Now(),
		}

		if err := h.storage.AddExpense(expense); err != nil {
			send(fmt.Sprintf("Ошибка сохранения: %v", err))
			log.Println("storage error:", err)
			return
		}

		send(fmt.Sprintf("Сохранил: %s — %.2f ₽", tag, float64(amount)/100))
	}
}
