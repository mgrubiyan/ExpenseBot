package bot

import (
	"fmt"

	"ExpenseBot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func groupsListKeyboard(groups []models.Group) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, g := range groups {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(g.Name, fmt.Sprintf("g:open:%d", g.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Создать группу", "g:new"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", callbackNavBackMain),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func groupCardKeyboard(groupID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить трату", fmt.Sprintf("g:addmenu:%d", groupID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Баланс", fmt.Sprintf("g:bal:%d", groupID)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Рассчитаться", fmt.Sprintf("g:settlemenu:%d", groupID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Пригласить", fmt.Sprintf("g:invite:%d", groupID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "g:list"),
		),
	)
}

func addModeKeyboard(groupID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Поровну на всех", fmt.Sprintf("g:addeq:%d", groupID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Выбрать участников", fmt.Sprintf("g:addcu:%d", groupID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("g:back:%d", groupID)),
		),
	)
}

func memberPickerKeyboard(groupID int64, members []models.User, selected map[int64]bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, m := range members {
		mark := "⬜"
		if selected[m.ID] {
			mark = "✅"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mark+" "+displayName(m), fmt.Sprintf("g:tog:%d:%d", groupID, m.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Готово", fmt.Sprintf("g:done:%d", groupID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("g:back:%d", groupID)),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func settleTargetKeyboard(groupID int64, members []models.User, excludeInternalUserID int64) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, m := range members {
		if m.ID == excludeInternalUserID {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(displayName(m), fmt.Sprintf("g:sttgt:%d:%d", groupID, m.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("g:back:%d", groupID)),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func cancelKeyboard(groupID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", fmt.Sprintf("g:back:%d", groupID)),
		),
	)
}
