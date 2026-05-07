package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Expense struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Tag       string    `json:"tag"`
	Amount    int64     `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

func ParseExpenseInput(text string) (string, int64, error) {
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

func FormatStats(expenses []Expense, period string) string {
	totals := make(map[string]int64)
	counts := make(map[string]int)
	var total int64

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
		sb.WriteString(fmt.Sprintf("• %s — %.2f ₽ (%d)\n", tag, float64(totals[tag])/100, counts[tag]))
	}
	sb.WriteString(fmt.Sprintf("\nИтого: %.2f ₽", float64(total)/100))

	return sb.String()
}
