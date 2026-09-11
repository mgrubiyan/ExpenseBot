package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ExpenseBot/internal/models"
	"ExpenseBot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	storage  storage.Storage
	sessions *sessionStore
}

func NewHandler(st storage.Storage) *Handler {
	return &Handler{storage: st, sessions: newSessionStore()}
}

func (h *Handler) handleCallback(ctx context.Context, api *tgbotapi.BotAPI, cq *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cq.ID, "")
	if _, err := api.Request(callback); err != nil {
		log.Println("callback answer error:", err)
	}

	// Group-flow buttons (group list, group card, add/balance/settle wizards)
	// live in their own callback data namespace and their own handler.
	if strings.HasPrefix(cq.Data, "g:") {
		h.handleGroupCallback(ctx, api, cq)
		return
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
		msg := tgbotapi.NewMessage(chatID, helpText())
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
		h.sendTodayStats(ctx, userID, send)

	case callbackHistoryWeek:
		h.sendWeekStats(ctx, userID, send)

	case callbackHistoryMonth:
		h.sendMonthStats(ctx, userID, send)

	case callbackHistoryLast5:
		h.sendLast5(ctx, userID, send)

	case callbackMenuDeleteLast:
		h.deleteLastExpense(ctx, userID, send)

	case callbackMenuGroups:
		// Reuse the group-list view by handing off with rewritten
		// callback data, so there's one place that renders it.
		cq.Data = "g:list"
		h.handleGroupCallback(ctx, api, cq)

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

func helpText() string {
	return "Как добавить личную трату:\n" +
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
		"/del — удалить последнюю трату\n\n" +
		"Групповые траты (с соседями/друзьями) — жми «👥 Группы» в меню, коды вводить не нужно.\n" +
		"Для тех, кто любит команды, есть и текстовый вариант:\n" +
		"/newgroup <название> — создать группу\n" +
		"/join <код> — вступить в группу по коду\n" +
		"/mygroups — мои группы\n" +
		"/gadd <код> <сумма> <описание> — общая трата, делится поровну\n" +
		"/gadd <код> <сумма> <описание> @user:сумма ... — с кастомными долями\n" +
		"/gbalance <код> — баланс группы и кто кому должен\n" +
		"/gsettle <код> @user <сумма> — отметить, что рассчитались"
}

func (h *Handler) HandleUpdate(api *tgbotapi.BotAPI, update tgbotapi.Update) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if update.CallbackQuery != nil {
		h.handleCallback(ctx, api, update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	username := update.Message.From.UserName
	text := update.Message.Text

	// A plain-text reply (not a command) while a button-driven flow is in
	// progress (e.g. we just asked "введи сумму") continues that flow
	// instead of being parsed as a personal expense.
	if !strings.HasPrefix(text, "/") {
		if h.handleGroupSessionText(ctx, userID, username, text, api, chatID) {
			return
		}
	}

	send := func(reply string) {
		msg := tgbotapi.NewMessage(chatID, reply)
		msg.ReplyToMessageID = update.Message.MessageID
		if _, err := api.Send(msg); err != nil {
			log.Println("send error:", err)
		}
	}

	// Split off the command word so commands that take arguments
	// (e.g. "/newgroup Соседи") route correctly; a bare "еда 450" still
	// falls straight through to the expense parser below.
	fields := strings.Fields(text)
	command := ""
	args := ""
	if len(fields) > 0 {
		command = fields[0]
		args = strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
	}

	if command != "" {
		// Any explicit command abandons whatever button flow was pending -
		// otherwise a stray "введи сумму" reply could later be reinterpreted.
		h.sessions.reset(userID)
	}

	switch command {
	case "/start":
		if strings.HasPrefix(args, "join_") {
			code := strings.TrimPrefix(args, "join_")
			h.handleJoinGroup(ctx, userID, username, code, send)
		}

		msg := tgbotapi.NewMessage(
			chatID, "Привет! 👋\n\n"+
				"Я помогу тебе контролировать расходы.\n\n"+
				"Выбери действие ниже или просто отправь трату в формате:\n"+
				"еда 450\n\n"+
				"Есть общие траты с соседями или друзьями? Жми «👥 Группы» в меню.")
		msg.ReplyMarkup = mainMenuKeyboard()

		if _, err := api.Send(msg); err != nil {
			log.Println("send start message error:", err)
		}

	case "/help":
		send(helpText())

	case "/month":
		h.sendMonthStats(ctx, userID, send)

	case "/week":
		h.sendWeekStats(ctx, userID, send)

	case "/today":
		h.sendTodayStats(ctx, userID, send)

	case "/l5":
		h.sendLast5(ctx, userID, send)

	case "/del":
		h.deleteLastExpense(ctx, userID, send)

	case "/newgroup":
		h.handleNewGroup(ctx, userID, username, args, send)

	case "/join":
		h.handleJoinGroup(ctx, userID, username, args, send)

	case "/mygroups":
		h.handleMyGroups(ctx, userID, username, send)

	case "/gadd":
		h.handleGroupAdd(ctx, userID, username, args, send)

	case "/gbalance":
		h.handleGroupBalance(ctx, userID, username, args, send)

	case "/gsettle":
		h.handleGroupSettle(ctx, userID, username, args, send)

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

		if err := h.storage.AddExpense(ctx, expense); err != nil {
			send(fmt.Sprintf("Ошибка сохранения: %v", err))
			log.Println("storage error:", err)
			return
		}

		send(fmt.Sprintf("Сохранил: %s — %.2f ₽", tag, float64(amount)/100))
	}
}
