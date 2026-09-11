package bot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"ExpenseBot/internal/models"
)

func displayName(u models.User) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("id%d", u.TelegramID)
}

func findMemberByUsername(members []models.User, username string) *models.User {
	username = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	if username == "" {
		return nil
	}
	for i := range members {
		if strings.ToLower(members[i].Username) == username {
			return &members[i]
		}
	}
	return nil
}

func isMember(members []models.User, internalUserID int64) bool {
	for _, m := range members {
		if m.ID == internalUserID {
			return true
		}
	}
	return false
}

func (h *Handler) handleNewGroup(ctx context.Context, tgUserID int64, username, args string, send func(string)) {
	name := strings.TrimSpace(args)
	if name == "" {
		send("Использование: /newgroup <название>\nПример: /newgroup Соседи по квартире")
		return
	}

	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось создать группу.")
		log.Println("get or create user error:", err)
		return
	}

	group, err := h.storage.CreateGroup(ctx, name, user.ID)
	if err != nil {
		send("Не удалось создать группу.")
		log.Println("create group error:", err)
		return
	}

	send(fmt.Sprintf(
		"Группа «%s» создана!\n\nКод приглашения: %s\n\nОтправь его тем, кого хочешь добавить — им нужно написать боту:\n/join %s",
		group.Name, group.InviteCode, group.InviteCode,
	))
}

func (h *Handler) handleJoinGroup(ctx context.Context, tgUserID int64, username, args string, send func(string)) {
	code := strings.TrimSpace(args)
	if code == "" {
		send("Использование: /join <код приглашения>")
		return
	}

	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось присоединиться к группе.")
		log.Println("get or create user error:", err)
		return
	}

	group, err := h.storage.JoinGroupByInviteCode(ctx, code, user.ID)
	if err != nil {
		send("Группа с таким кодом не найдена.")
		log.Println("join group error:", err)
		return
	}

	send(fmt.Sprintf("Готово! Ты в группе «%s».", group.Name))
}

func (h *Handler) handleMyGroups(ctx context.Context, tgUserID int64, username string, send func(string)) {
	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось получить список групп.")
		log.Println("get or create user error:", err)
		return
	}

	groups, err := h.storage.GetUserGroups(ctx, user.ID)
	if err != nil {
		send("Не удалось получить список групп.")
		log.Println("get user groups error:", err)
		return
	}

	if len(groups) == 0 {
		send("Ты пока не состоишь ни в одной группе.\n\nСоздать: /newgroup <название>\nПрисоединиться: /join <код>")
		return
	}

	var sb strings.Builder
	sb.WriteString("Твои группы:\n\n")
	for _, g := range groups {
		sb.WriteString(fmt.Sprintf("• %s — код %s\n", g.Name, g.InviteCode))
	}
	send(sb.String())
}

func (h *Handler) handleGroupAdd(ctx context.Context, tgUserID int64, username, args string, send func(string)) {
	fields := strings.Fields(args)
	if len(fields) < 3 {
		send("Использование:\n" +
			"/gadd <код> <сумма> <описание>\n" +
			"/gadd <код> <сумма> <описание> @user1:сумма1 @user2:сумма2 ...\n\n" +
			"Если доли не указаны — трата делится поровну между всеми в группе.")
		return
	}

	code := fields[0]

	amount, err := models.ParseAmount(fields[1])
	if err != nil || amount <= 0 {
		send("Не понял сумму. Пример: /gadd ABC123 1500 аренда")
		return
	}

	i := 2
	var descWords []string
	for i < len(fields) && !strings.Contains(fields[i], ":") {
		descWords = append(descWords, fields[i])
		i++
	}
	description := strings.Join(descWords, " ")
	if description == "" {
		send("Не хватает описания траты. Пример: /gadd ABC123 1500 аренда")
		return
	}
	splitTokens := fields[i:]

	group, err := h.storage.GetGroupByInviteCode(ctx, code)
	if err != nil {
		send("Не удалось добавить трату.")
		log.Println("get group by invite code error:", err)
		return
	}
	if group == nil {
		send("Группа с таким кодом не найдена.")
		return
	}

	payer, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось добавить трату.")
		log.Println("get or create user error:", err)
		return
	}

	members, err := h.storage.GetGroupMembers(ctx, group.ID)
	if err != nil {
		send("Не удалось добавить трату.")
		log.Println("get group members error:", err)
		return
	}
	if !isMember(members, payer.ID) {
		send(fmt.Sprintf("Ты не состоишь в группе «%s». Сначала: /join %s", group.Name, group.InviteCode))
		return
	}

	var splits []models.TransactionSplit

	if len(splitTokens) == 0 {
		if len(members) == 0 {
			send("В группе пока нет участников.")
			return
		}
		base := amount / int64(len(members))
		remainder := amount % int64(len(members))
		for idx, m := range members {
			share := base
			if int64(idx) < remainder {
				share++
			}
			splits = append(splits, models.TransactionSplit{UserID: m.ID, Amount: share})
		}
	} else {
		var sum int64
		for _, tok := range splitTokens {
			parts := strings.SplitN(tok, ":", 2)
			if len(parts) != 2 {
				send(fmt.Sprintf("Не понял долю «%s». Формат: @username:сумма", tok))
				return
			}
			member := findMemberByUsername(members, parts[0])
			if member == nil {
				send(fmt.Sprintf("%s не найден среди участников группы (сначала должен(на) сделать /join %s).", parts[0], group.InviteCode))
				return
			}
			share, err := models.ParseAmount(parts[1])
			if err != nil {
				send(fmt.Sprintf("Не понял сумму «%s» для %s.", parts[1], parts[0]))
				return
			}
			splits = append(splits, models.TransactionSplit{UserID: member.ID, Amount: share})
			sum += share
		}
		if sum != amount {
			send(fmt.Sprintf("Сумма долей (%.2f ₽) не совпадает с суммой траты (%.2f ₽).", float64(sum)/100, float64(amount)/100))
			return
		}
	}

	tx := models.Transaction{
		GroupID:     group.ID,
		PayerID:     payer.ID,
		Amount:      amount,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if err := h.storage.CreateTransaction(ctx, tx, splits); err != nil {
		send("Не удалось сохранить трату.")
		log.Println("create transaction error:", err)
		return
	}

	send(fmt.Sprintf("Добавил в «%s»: %s — %.2f ₽", group.Name, description, float64(amount)/100))
}

func (h *Handler) handleGroupBalance(ctx context.Context, tgUserID int64, username, args string, send func(string)) {
	code := strings.TrimSpace(args)
	if code == "" {
		send("Использование: /gbalance <код группы>")
		return
	}

	group, err := h.storage.GetGroupByInviteCode(ctx, code)
	if err != nil {
		send("Не удалось получить баланс.")
		log.Println("get group by invite code error:", err)
		return
	}
	if group == nil {
		send("Группа с таким кодом не найдена.")
		return
	}

	user, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось получить баланс.")
		log.Println("get or create user error:", err)
		return
	}

	members, err := h.storage.GetGroupMembers(ctx, group.ID)
	if err != nil {
		send("Не удалось получить баланс.")
		log.Println("get group members error:", err)
		return
	}
	if !isMember(members, user.ID) {
		send(fmt.Sprintf("Ты не состоишь в группе «%s».", group.Name))
		return
	}

	balances, err := h.storage.GetGroupBalances(ctx, group.ID)
	if err != nil {
		send("Не удалось получить баланс.")
		log.Println("get group balances error:", err)
		return
	}

	text := formatGroupBalance(group, members, balances)
	if len(simplifyDebts(balances)) > 0 {
		text += "\nКогда рассчитаетесь: /gsettle " + group.InviteCode + " @user сумма"
	}
	send(text)
}

// formatGroupBalance renders per-member balances plus a simplified
// "who pays whom" settlement plan. Shared by the text-command and
// button-driven balance views.
func formatGroupBalance(group *models.Group, members []models.User, balances map[int64]int64) string {
	byID := make(map[int64]models.User, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Баланс в «%s»:\n\n", group.Name))
	for _, m := range members {
		bal := balances[m.ID]
		switch {
		case bal > 0:
			sb.WriteString(fmt.Sprintf("%s: ему/ей должны %.2f ₽\n", displayName(m), float64(bal)/100))
		case bal < 0:
			sb.WriteString(fmt.Sprintf("%s: должен(на) %.2f ₽\n", displayName(m), float64(-bal)/100))
		default:
			sb.WriteString(fmt.Sprintf("%s: в расчёте\n", displayName(m)))
		}
	}

	debts := simplifyDebts(balances)
	if len(debts) > 0 {
		sb.WriteString("\nЧтобы рассчитаться:\n")
		for _, d := range debts {
			sb.WriteString(fmt.Sprintf("%s → %s: %.2f ₽\n", displayName(byID[d.From]), displayName(byID[d.To]), float64(d.Amount)/100))
		}
	}

	return sb.String()
}

func (h *Handler) handleGroupSettle(ctx context.Context, tgUserID int64, username, args string, send func(string)) {
	fields := strings.Fields(args)
	if len(fields) != 3 {
		send("Использование: /gsettle <код> @user <сумма>\n\nЭто значит: ты рассчитался(лась) с этим человеком.")
		return
	}

	code, targetRaw, amountRaw := fields[0], fields[1], fields[2]

	amount, err := models.ParseAmount(amountRaw)
	if err != nil || amount <= 0 {
		send("Не понял сумму.")
		return
	}

	group, err := h.storage.GetGroupByInviteCode(ctx, code)
	if err != nil {
		send("Не удалось зафиксировать расчёт.")
		log.Println("get group by invite code error:", err)
		return
	}
	if group == nil {
		send("Группа с таким кодом не найдена.")
		return
	}

	sender, err := h.storage.GetOrCreateUser(ctx, tgUserID, username)
	if err != nil {
		send("Не удалось зафиксировать расчёт.")
		log.Println("get or create user error:", err)
		return
	}

	members, err := h.storage.GetGroupMembers(ctx, group.ID)
	if err != nil {
		send("Не удалось зафиксировать расчёт.")
		log.Println("get group members error:", err)
		return
	}
	if !isMember(members, sender.ID) {
		send(fmt.Sprintf("Ты не состоишь в группе «%s».", group.Name))
		return
	}

	target := findMemberByUsername(members, targetRaw)
	if target == nil {
		send(fmt.Sprintf("%s не найден среди участников группы.", targetRaw))
		return
	}
	if target.ID == sender.ID {
		send("Нельзя рассчитаться самим с собой.")
		return
	}

	st := models.Settlement{
		GroupID:    group.ID,
		FromUserID: sender.ID,
		ToUserID:   target.ID,
		Amount:     amount,
		CreatedAt:  time.Now(),
	}

	if err := h.storage.CreateSettlement(ctx, st); err != nil {
		send("Не удалось зафиксировать расчёт.")
		log.Println("create settlement error:", err)
		return
	}

	send(fmt.Sprintf("Записал: ты рассчитался(лась) с %s на %.2f ₽ в «%s».", displayName(*target), float64(amount)/100, group.Name))
}

type debtPair struct {
	From   int64
	To     int64
	Amount int64
}

// simplifyDebts reduces a set of net balances to a minimal list of
// "who pays whom" transfers using a greedy largest-creditor/largest-debtor
// match. It doesn't reconstruct the original pairwise history, only a
// settlement plan that zeroes everyone out.
func simplifyDebts(balances map[int64]int64) []debtPair {
	type entry struct {
		id     int64
		amount int64
	}

	var creditors, debtors []entry
	for id, bal := range balances {
		switch {
		case bal > 0:
			creditors = append(creditors, entry{id, bal})
		case bal < 0:
			debtors = append(debtors, entry{id, -bal})
		}
	}

	sort.Slice(creditors, func(i, j int) bool { return creditors[i].amount > creditors[j].amount })
	sort.Slice(debtors, func(i, j int) bool { return debtors[i].amount > debtors[j].amount })

	var result []debtPair
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		d := &debtors[i]
		c := &creditors[j]

		amt := d.amount
		if c.amount < amt {
			amt = c.amount
		}
		if amt > 0 {
			result = append(result, debtPair{From: d.id, To: c.id, Amount: amt})
		}

		d.amount -= amt
		c.amount -= amt

		if d.amount == 0 {
			i++
		}
		if c.amount == 0 {
			j++
		}
	}

	return result
}
