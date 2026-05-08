package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

const (
	callbackMenuAdd        = "menu:add"
	callbackMenuDeleteLast = "menu:delete_last"
	callbackMenuHistory    = "menu:history"
	callbackMenuHelp       = "menu:help"

	callbackHistoryToday = "history:today"
	callbackHistoryWeek  = "history:week"
	callbackHistoryMonth = "history:month"
	callbackHistoryLast5 = "history:last5"

	callbackNavBackMain = "nav:back_main"
)

func mainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", callbackMenuAdd),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", callbackMenuDeleteLast),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 История", callbackMenuHistory),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Help", callbackMenuHelp),
		),
	)
}

func historyMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сегодня", callbackHistoryToday),
			tgbotapi.NewInlineKeyboardButtonData("7 дней", callbackHistoryWeek),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Месяц", callbackHistoryMonth),
			tgbotapi.NewInlineKeyboardButtonData("Последние 5", callbackHistoryLast5),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", callbackNavBackMain),
		),
	)
}
