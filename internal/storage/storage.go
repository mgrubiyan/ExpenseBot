package storage

import (
	"ExpenseBot/internal/models"
	"time"
)

type Storage interface {
	AddExpense(expense models.Expense) error
	GetExpensesByPeriod(userID int64, from, to time.Time) ([]models.Expense, error)
}
