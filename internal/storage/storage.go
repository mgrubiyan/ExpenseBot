package storage

import (
	"ExpenseBot/internal/models"
	"context"
	"time"
)

type Storage interface {
	AddExpense(ctx context.Context, expense models.Expense) error
	GetExpensesByPeriod(ctx context.Context, userID int64, from, to time.Time) ([]models.Expense, error)
	GetLastExpenses(ctx context.Context, userID int64, limit int) ([]models.Expense, error)
	DeleteLastExpense(ctx context.Context, userID int64) (*models.Expense, error)
}
