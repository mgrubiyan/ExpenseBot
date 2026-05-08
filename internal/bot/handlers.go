package bot

import (
	"fmt"
	"log"
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

func (h *Handler) handleCallback(api *tgbotapi.BotAPI, cq *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cq.ID, "")
	if _, err := api.Request(callback); err != nil {
		log.Println("callback answer error:", err)
	}

	chatID := cq.Message.Chat.ID
	data := cq.Data
	userID := cq.From.ID

	send := func(reply string) {
		msg := tgbotapi.NewMessage(chatID, reply)
		if _, err := api.Send(msg); err != nil {
			log.Println("send callback message error:", err)
		}
	}

	switch data {
	case callbackMenuAdd:
		msg := tgbotapi.NewMessage(chatID, "Отправь трату в формате:\nкатегория сумма\n\nПримеры:\nеда 450\nкофе 4.5")
		msg.ReplyMarkup = mainMenuKeyboard()
		if _, err := api.Send(msg); err != nil {
			log.Println("send add hint error:", err)
		}

	case callbackMenuHelp:
		msg := tgbotapi.NewMessage(chatID, "Команды:\n/today — расходы за сегодня\n/week — расходы за 7 дней\n/month — расходы за текущий месяц\n/l5 — последние 5 трат\n/del — удалить последнюю трату")
		msg.ReplyMarkup = mainMenuKeyboard()
		if _, err := api.Send(msg); err != nil {
			log.Println("send help error:", err)
		}

	case callbackMenuHistory:
		msg := tgbotapi.NewMessage(chatID, "Выбери период истории:")
		msg.ReplyMarkup = historyMenuKeyboard()
		if _, err := api.Send(msg); err != nil {
			log.Println("send history menu error:", err)
		}

	case callbackHistoryToday:
		h.sendTodayStats(userID, send)

	case callbackHistoryWeek:
		h.sendWeekStats(userID, send)

	case callbackHistoryMonth:
		h.sendMonthStats(userID, send)

	case callbackHistoryLast5:
		h.sendLast5(userID, send)

	case callbackMenuDeleteLast:
		h.deleteLastExpense(userID, send)

	case callbackNavBackMain:
		msg := tgbotapi.NewMessage(chatID, "Главное меню:")
		msg.ReplyMarkup = mainMenuKeyboard()
		if _, err := api.Send(msg); err != nil {
			log.Println("send main menu error:", err)
		}

	default:
		send("Нажата кнопка: " + data)
	}
}

func (h *Handler) HandleUpdate(api *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(api, update.CallbackQuery)
		return
	}

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
		msg := tgbotapi.NewMessage(
			chatID, "Привет! 👋\n\n"+
				"Я помогу тебе контролировать расходы.\n\n"+
				"Выбери действие ниже или просто отправь трату в формате:\n"+
				"еда 450")
		msg.ReplyMarkup = mainMenuKeyboard()

		if _, err := api.Send(msg); err != nil {
			log.Println("send start message error:", err)
		}

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
		h.sendMonthStats(userID, send)

	case "/week":
		h.sendWeekStats(userID, send)

	case "/today":
		h.sendTodayStats(userID, send)

	case "/l5":
		h.sendLast5(userID, send)

	case "/del":
		h.deleteLastExpense(userID, send)

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
