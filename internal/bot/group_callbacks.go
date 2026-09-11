package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"ExpenseBot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleGroupCallback dispatches inline-button taps for the group-expenses
// feature (every callback_data value prefixed "g:"). It edits the tapped
// message in place rather than sending new ones, so navigating groups
// doesn't flood the chat.
func (h *Handler) handleGroupCallback(ctx context.Context, api *tgbotapi.BotAPI, cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID
	messageID := cq.Message.MessageID
	tgUserID := cq.From.ID
	username := cq.From.UserName

	parts := strings.Split(cq.Data, ":")
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	edit := func(text string, kb tgbotapi.InlineKeyboardMarkup) {
		msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		msg.ReplyMarkup = &kb
		if _, err := api.Send(msg); err != nil {
			log.Println("edit message error:", err)
		}
	}
	prompt := func(text string, kb tgbotapi.InlineKeyboardMarkup) {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = kb
		if _, err := api.Send(msg); err != nil {
			log.Println("send prompt error:", err)
		}
	}

	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		log.Println("get or create user error:", err)
		return
	}
	sess := h.sessions.get(tgUserID)

	// groupFromArg resolves the group-id argument at parts[idx] and checks
	// that the current user is actually a member of it.
	groupFromArg := func(idx int) (*models.Group, []models.User, bool) {
		if idx >= len(parts) {
			return nil, nil, false
		}
		groupID, err := strconv.ParseInt(parts[idx], 10, 64)
		if err != nil {
			return nil, nil, false
		}
		group, err := h.storage.GetGroupByID(ctx, groupID)
		if err != nil || group == nil {
			return nil, nil, false
		}
		members, err := h.storage.GetGroupMembers(ctx, group.ID)
		if err != nil || !isMember(members, user.ID) {
			return nil, nil, false
		}
		return group, members, true
	}

	switch action {
	case "list":
		h.sessions.reset(tgUserID)
		groups, err := h.storage.GetUserGroups(ctx, user.ID)
		if err != nil {
			log.Println("get user groups error:", err)
			return
		}
		if len(groups) == 0 {
			edit("Ты пока не состоишь ни в одной группе.", groupsListKeyboard(nil))
			return
		}
		edit("Твои группы:", groupsListKeyboard(groups))

	case "new":
		// NB: assign through the *sess pointer (like the other flows below)
		// rather than calling h.sessions.reset() and then mutating sess -
		// reset() swaps in a brand-new session object, which would leave
		// this state change applied to the now-orphaned old one.
		*sess = session{state: stateAwaitingGroupName}
		edit("Введи название новой группы:", tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "g:list")),
		))

	case "open", "back":
		group, _, ok := groupFromArg(2)
		if !ok {
			edit("Группа недоступна.", groupsListKeyboard(nil))
			return
		}
		h.sessions.reset(tgUserID)
		edit(
			fmt.Sprintf("«%s»\nКод приглашения: %s", group.Name, group.InviteCode),
			groupCardKeyboard(group.ID),
		)

	case "addmenu":
		group, _, ok := groupFromArg(2)
		if !ok {
			return
		}
		edit(fmt.Sprintf("Добавляем трату в «%s». Как делим?", group.Name), addModeKeyboard(group.ID))

	case "addeq":
		group, _, ok := groupFromArg(2)
		if !ok {
			return
		}
		*sess = session{state: stateAwaitingAddAmount, groupID: group.ID}
		prompt("Введи сумму траты:", cancelKeyboard(group.ID))

	case "addcu":
		group, members, ok := groupFromArg(2)
		if !ok {
			return
		}
		*sess = session{groupID: group.ID, selected: map[int64]bool{}}
		edit("Кто участвует в трате? Отметь участников и жми «Готово»:", memberPickerKeyboard(group.ID, members, sess.selected))

	case "tog":
		group, members, ok := groupFromArg(2)
		if !ok || len(parts) < 4 {
			return
		}
		memberID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return
		}
		if sess.groupID != group.ID || sess.selected == nil {
			sess.groupID = group.ID
			sess.selected = map[int64]bool{}
		}
		if sess.selected[memberID] {
			delete(sess.selected, memberID)
		} else {
			sess.selected[memberID] = true
		}
		edit("Кто участвует в трате? Отметь участников и жми «Готово»:", memberPickerKeyboard(group.ID, members, sess.selected))

	case "done":
		group, _, ok := groupFromArg(2)
		if !ok {
			return
		}
		if len(sess.selected) == 0 {
			return
		}
		sess.state = stateAwaitingAddAmount
		prompt("Введи сумму траты:", cancelKeyboard(group.ID))

	case "bal":
		group, members, ok := groupFromArg(2)
		if !ok {
			edit("Группа недоступна.", groupsListKeyboard(nil))
			return
		}
		balances, err := h.storage.GetGroupBalances(ctx, group.ID)
		if err != nil {
			log.Println("get group balances error:", err)
			return
		}
		edit(formatGroupBalance(group, members, balances), groupCardKeyboard(group.ID))

	case "invite":
		group, _, ok := groupFromArg(2)
		if !ok {
			return
		}
		link := fmt.Sprintf("https://t.me/%s?start=join_%s", api.Self.UserName, group.InviteCode)
		edit(
			fmt.Sprintf("Пригласи в «%s» этой ссылкой — по клику человек сразу окажется в группе:\n%s\n\n(или код для /join: %s)", group.Name, link, group.InviteCode),
			groupCardKeyboard(group.ID),
		)

	case "settlemenu":
		group, members, ok := groupFromArg(2)
		if !ok {
			return
		}
		if len(members) < 2 {
			edit("В группе больше некому рассчитываться.", groupCardKeyboard(group.ID))
			return
		}
		edit("С кем рассчитались?", settleTargetKeyboard(group.ID, members, user.ID))

	case "sttgt":
		group, members, ok := groupFromArg(2)
		if !ok || len(parts) < 4 {
			return
		}
		targetID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || !isMember(members, targetID) {
			return
		}
		*sess = session{state: stateAwaitingSettleAmount, groupID: group.ID, settleTargetID: targetID}
		prompt("На какую сумму рассчитались?", cancelKeyboard(group.ID))

	default:
		log.Println("unknown group callback:", cq.Data)
	}
}

// handleGroupSessionText continues a button-driven flow when the user
// replies with plain text (a name, an amount, a description). Returns
// true if the message was consumed as part of such a flow.
func (h *Handler) handleGroupSessionText(ctx context.Context, tgUserID int64, username, text string, api *tgbotapi.BotAPI, chatID int64) bool {
	sess := h.sessions.get(tgUserID)
	if sess.state == stateNone {
		return false
	}

	send := func(reply string) {
		if _, err := api.Send(tgbotapi.NewMessage(chatID, reply)); err != nil {
			log.Println("send error:", err)
		}
	}

	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Что-то пошло не так, попробуй ещё раз.")
		log.Println("get or create user error:", err)
		h.sessions.reset(tgUserID)
		return true
	}

	// The group-name step is the only one without a group to check yet.
	if sess.state == stateAwaitingGroupName {
		name := strings.TrimSpace(text)
		if name == "" {
			send("Название не может быть пустым. Введи название группы:")
			return true
		}
		h.sessions.reset(tgUserID)

		group, err := h.storage.CreateGroup(ctx, name, user.ID)
		if err != nil {
			send("Не удалось создать группу.")
			log.Println("create group error:", err)
			return true
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Группа «%s» создана!", group.Name))
		msg.ReplyMarkup = groupCardKeyboard(group.ID)
		if _, err := api.Send(msg); err != nil {
			log.Println("send group card error:", err)
		}
		return true
	}

	group, err := h.storage.GetGroupByID(ctx, sess.groupID)
	if err != nil || group == nil {
		send("Группа больше не доступна.")
		h.sessions.reset(tgUserID)
		return true
	}

	members, err := h.storage.GetGroupMembers(ctx, group.ID)
	if err != nil || !isMember(members, user.ID) {
		send("Ты больше не состоишь в этой группе.")
		h.sessions.reset(tgUserID)
		return true
	}

	switch sess.state {
	case stateAwaitingAddAmount:
		amount, err := models.ParseAmount(text)
		if err != nil || amount <= 0 {
			send("Не понял сумму. Введи число, например 1500 или 450.50:")
			return true
		}
		sess.amount = amount
		sess.state = stateAwaitingAddDescription

		msg := tgbotapi.NewMessage(chatID, "На что потратили? Введи короткое описание:")
		msg.ReplyMarkup = cancelKeyboard(group.ID)
		if _, err := api.Send(msg); err != nil {
			log.Println("send prompt error:", err)
		}

	case stateAwaitingAddDescription:
		description := strings.TrimSpace(text)
		if description == "" {
			send("Описание не может быть пустым. Введи, на что потратили:")
			return true
		}

		var participants []models.User
		for _, m := range members {
			if len(sess.selected) == 0 || sess.selected[m.ID] {
				participants = append(participants, m)
			}
		}
		if len(participants) == 0 {
			participants = members
		}

		amount := sess.amount
		base := amount / int64(len(participants))
		remainder := amount % int64(len(participants))
		splits := make([]models.TransactionSplit, 0, len(participants))
		for idx, m := range participants {
			share := base
			if int64(idx) < remainder {
				share++
			}
			splits = append(splits, models.TransactionSplit{UserID: m.ID, Amount: share})
		}

		tx := models.Transaction{
			GroupID:     group.ID,
			PayerID:     user.ID,
			Amount:      amount,
			Description: description,
			CreatedAt:   time.Now(),
		}

		h.sessions.reset(tgUserID)

		if err := h.storage.CreateTransaction(ctx, tx, splits); err != nil {
			send("Не удалось сохранить трату.")
			log.Println("create transaction error:", err)
			return true
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"Добавил в «%s»: %s — %.2f ₽ (участников: %d)",
			group.Name, description, float64(amount)/100, len(participants),
		))
		msg.ReplyMarkup = groupCardKeyboard(group.ID)
		if _, err := api.Send(msg); err != nil {
			log.Println("send confirmation error:", err)
		}

	case stateAwaitingSettleAmount:
		amount, err := models.ParseAmount(text)
		if err != nil || amount <= 0 {
			send("Не понял сумму. Введи число:")
			return true
		}

		var target *models.User
		for i := range members {
			if members[i].ID == sess.settleTargetID {
				target = &members[i]
				break
			}
		}
		h.sessions.reset(tgUserID)
		if target == nil {
			send("Этот участник больше не в группе.")
			return true
		}

		st := models.Settlement{
			GroupID:    group.ID,
			FromUserID: user.ID,
			ToUserID:   target.ID,
			Amount:     amount,
			CreatedAt:  time.Now(),
		}
		if err := h.storage.CreateSettlement(ctx, st); err != nil {
			send("Не удалось зафиксировать расчёт.")
			log.Println("create settlement error:", err)
			return true
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"Записал: рассчитались с %s на %.2f ₽ в «%s».",
			displayName(*target), float64(amount)/100, group.Name,
		))
		msg.ReplyMarkup = groupCardKeyboard(group.ID)
		if _, err := api.Send(msg); err != nil {
			log.Println("send confirmation error:", err)
		}

	default:
		h.sessions.reset(tgUserID)
		return false
	}

	return true
}
