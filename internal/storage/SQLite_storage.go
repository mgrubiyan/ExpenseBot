package storage

import (
	"ExpenseBot/internal/models"
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS expenses (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id    INTEGER NOT NULL,
            tag        TEXT    NOT NULL,
            amount     INTEGER NOT NULL,
            created_at DATETIME NOT NULL
        )
    `)
	return err
}

func (s *SQLiteStorage) AddExpense(ctx context.Context, expense models.Expense) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO expenses (user_id, tag, amount, created_at) VALUES (?, ?, ?, ?)`,
		expense.UserID,
		expense.Tag,
		expense.Amount,
		expense.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStorage) GetExpensesByPeriod(ctx context.Context, userID int64, from, to time.Time) ([]models.Expense, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, tag, amount, created_at
        FROM expenses
        WHERE user_id = ? AND created_at >= ? AND created_at <= ?
        ORDER BY created_at DESC`,
		userID,
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		var createdAt string

		if err := rows.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &createdAt); err != nil {
			return nil, fmt.Errorf("row scan: %w", err)
		}
		e.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}

		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}
func (s *SQLiteStorage) GetLastExpenses(ctx context.Context, userID int64, limit int) ([]models.Expense, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, tag, amount, created_at
         FROM expenses
         WHERE user_id = ?
         ORDER BY created_at DESC
         LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		var createdAt string

		if err := rows.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &createdAt); err != nil {
			return nil, fmt.Errorf("row scan: %w", err)
		}

		e.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}

		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

func (s *SQLiteStorage) DeleteLastExpense(ctx context.Context, userID int64) (*models.Expense, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, tag, amount, created_at
		FROM expenses
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID)
	var e models.Expense
	var createdAt string

	err := row.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select last expense: %w", err)
	}

	e.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse time: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = ? AND user_id = ?`, e.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("delete last expense: %w", err)
	}

	return &e, nil
}
