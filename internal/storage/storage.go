package storage

import (
	"ExpenseBot/internal/models"
	"time"
)

type Storage interface {
	AddExpense(expense models.Expense) error
	GetExpensesByPeriod(userID int64, from, to time.Time) ([]models.Expense, error)
	GetLastExpenses(userID int64, limit int) ([]models.Expense, error)
	DeleteLastExpense(userID int64) (*models.Expense, error)
}
