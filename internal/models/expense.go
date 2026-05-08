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
		return "", 0, fmt.Errorf("ожидается: <категория> <сумма>")
	}

	tag := parts[0]
	raw := parts[1]

	amount, err := parseAmountToCents(raw)
	if err != nil {
		return "", 0, err
	}

	return tag, amount, nil
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

func parseAmountToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)

	if s == "" {
		return 0, fmt.Errorf("пустая сумма")
	}

	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}

	if s == "" {
		return 0, fmt.Errorf("пустая сумма")
	}

	parts := strings.SplitN(s, ".", 2)

	intPartStr := parts[0]
	fracPartStr := ""
	if len(parts) == 2 {
		fracPartStr = parts[1]
	}

	if intPartStr == "" {
		intPartStr = "0"
	}

	for _, r := range intPartStr {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("неверный формат суммы")
		}
	}

	intPart, err := strconv.ParseInt(intPartStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("неверный формат суммы")
	}

	var fracPart int64
	switch len(fracPartStr) {
	case 0:
		fracPart = 0
	case 1:
		if fracPartStr[0] < '0' || fracPartStr[0] > '9' {
			return 0, fmt.Errorf("неверный формат суммы")
		}
		fracPart = int64(fracPartStr[0]-'0') * 10
	default:
		r0, r1 := fracPartStr[0], fracPartStr[1]
		if r0 < '0' || r0 > '9' || r1 < '0' || r1 > '9' {
			return 0, fmt.Errorf("неверный формат суммы")
		}
		fracPart = int64(r0-'0')*10 + int64(r1-'0')
	}

	cents := intPart*100 + fracPart
	if negative {
		cents = -cents
	}

	return cents, nil
}
