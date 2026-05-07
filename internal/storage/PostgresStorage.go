package storage

import (
	"ExpenseBot/internal/models"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(connStr string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migratePostgres(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func migratePostgres(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS expenses (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    tag        TEXT        NOT NULL,
    amount     INTEGER     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
`)
	return err
}

func (s *PostgresStorage) AddExpense(expense models.Expense) error {
	_, err := s.db.Exec(
		`INSERT INTO expenses (user_id, tag, amount, created_at) 
         VALUES ($1, $2, $3, $4)`,
		expense.UserID,
		expense.Tag,
		expense.Amount,
		expense.CreatedAt.UTC(),
	)
	return err
}

func (s *PostgresStorage) GetExpensesByPeriod(userID int64, from, to time.Time) ([]models.Expense, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, tag, amount, created_at
         FROM expenses
         WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
         ORDER BY created_at DESC`,
		userID,
		from.UTC(),
		to.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense

	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("row scan: %w", err)
		}
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

func (s *PostgresStorage) GetLastExpenses(userID int64, limit int) ([]models.Expense, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, tag, amount, created_at
		 FROM expenses
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query last expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan last expenses: %w", err)
		}
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

func (s *PostgresStorage) DeleteLastExpense(userID int64) (*models.Expense, error) {
	var e models.Expense

	err := s.db.QueryRow(`
		DELETE FROM expenses
		WHERE id = (
			SELECT id
			FROM expenses
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		)
		RETURNING id, user_id, tag, amount, created_at
	`, userID).Scan(&e.ID, &e.UserID, &e.Tag, &e.Amount, &e.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("delete last expense: %w", err)
	}

	return &e, nil
}
