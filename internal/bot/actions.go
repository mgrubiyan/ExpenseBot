package bot

import (
	"ExpenseBot/internal/models"
	"fmt"
	"log"
	"strings"
	"time"
)

func (h *Handler) sendTodayStats(userID int64, send func(string)) {
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
}

func (h *Handler) sendWeekStats(userID int64, send func(string)) {
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
}

func (h *Handler) sendMonthStats(userID int64, send func(string)) {
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
}

func (h *Handler) sendLast5(userID int64, send func(string)) {
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
}

func (h *Handler) deleteLastExpense(userID int64, send func(string)) {
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

}
