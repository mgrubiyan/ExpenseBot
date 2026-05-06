package storage

import (
	"ExpenseBot/internal/models"
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

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS expenses (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			tag        TEXT    NOT NULL,
			amount     INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	return err
}

func (s *SQLiteStorage) AddExpense(expense models.Expense) error {
	_, err := s.db.Exec(
		`INSERT INTO expenses (user_id, tag, amount, created_at) VALUES (?, ?, ?, ?)`,
		expense.UserID,
		expense.Tag,
		expense.Amount,
		expense.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert expense: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetExpensesByPeriod(userID int64, from, to time.Time) ([]models.Expense, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, tag, amount, created_at
		 FROM expenses
		 WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		 ORDER BY created_at DESC`,
		userID,
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		var createdAt string

		if err := rows.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &createdAt); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		parsed, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse time %q: %w", createdAt, err)
		}
		e.CreatedAt = parsed

		expenses = append(expenses, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return expenses, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
