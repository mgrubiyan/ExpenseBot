package storage

import (
	"ExpenseBot/internal/models"
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := migratePostgres(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func migratePostgres(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
	CREATE TABLE IF NOT EXISTS expenses (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		tag TEXT NOT NULL,
		amount INTEGER NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		telegram_id BIGINT UNIQUE NOT NULL,
		username TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS groups (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		invite_code TEXT UNIQUE NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS group_members (
		user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		joined_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, group_id)
	);

	CREATE TABLE IF NOT EXISTS transactions (
		id BIGSERIAL PRIMARY KEY,
		group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,
		payer_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		amount BIGINT NOT NULL,
		description TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS transaction_splits (
		transaction_id BIGINT REFERENCES transactions(id) ON DELETE CASCADE,
		user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		amount BIGINT NOT NULL,
		PRIMARY KEY (transaction_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS settlements (
		id BIGSERIAL PRIMARY KEY,
		group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,
		from_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		to_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		amount BIGINT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	`

	_, err := db.ExecContext(ctx, query)
	return err
}
func (s *PostgresStorage) AddExpense(ctx context.Context, expense models.Expense) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO expenses (user_id, tag, amount, created_at) 
         VALUES ($1, $2, $3, $4)`,
		expense.UserID,
		expense.Tag,
		expense.Amount,
		expense.CreatedAt.UTC(),
	)
	return err
}

func (s *PostgresStorage) GetExpensesByPeriod(ctx context.Context, userID int64, from, to time.Time) ([]models.Expense, error) {
	rows, err := s.db.QueryContext(ctx,
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

func (s *PostgresStorage) GetLastExpenses(ctx context.Context, userID int64, limit int) ([]models.Expense, error) {
	rows, err := s.db.QueryContext(ctx,
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

func (s *PostgresStorage) DeleteLastExpense(ctx context.Context, userID int64) (*models.Expense, error) {
	var e models.Expense

	err := s.db.QueryRowContext(ctx,
		`DELETE FROM expenses
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

func (s *PostgresStorage) GetOrCreateUser(ctx context.Context, telegramID int64, username string) (models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (telegram_id, username)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET username = EXCLUDED.username
		RETURNING id, telegram_id, username, created_at
	`, telegramID, username).Scan(&u.ID, &u.TelegramID, &u.Username, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("get or create user: %w", err)
	}
	return u, nil
}

func (s *PostgresStorage) CreateGroup(ctx context.Context, name string, ownerUserID int64) (models.Group, error) {
	var g models.Group

	for attempt := 0; attempt < 5; attempt++ {
		code := generateInviteCode()

		err := s.db.QueryRowContext(ctx, `
			INSERT INTO groups (name, invite_code) VALUES ($1, $2)
			RETURNING id, name, invite_code, created_at
		`, name, code).Scan(&g.ID, &g.Name, &g.InviteCode, &g.CreatedAt)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				continue
			}
			return models.Group{}, fmt.Errorf("create group: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO group_members (user_id, group_id, role) VALUES ($1, $2, 'owner')
		`, ownerUserID, g.ID); err != nil {
			return models.Group{}, fmt.Errorf("add owner to group: %w", err)
		}

		return g, nil
	}

	return models.Group{}, fmt.Errorf("create group: could not generate a unique invite code")
}

func (s *PostgresStorage) GetGroupByInviteCode(ctx context.Context, inviteCode string) (*models.Group, error) {
	var g models.Group

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, invite_code, created_at FROM groups WHERE invite_code = $1
	`, strings.ToUpper(strings.TrimSpace(inviteCode))).Scan(&g.ID, &g.Name, &g.InviteCode, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by invite code: %w", err)
	}
	return &g, nil
}

func (s *PostgresStorage) JoinGroupByInviteCode(ctx context.Context, inviteCode string, userID int64) (models.Group, error) {
	group, err := s.GetGroupByInviteCode(ctx, inviteCode)
	if err != nil {
		return models.Group{}, err
	}
	if group == nil {
		return models.Group{}, fmt.Errorf("группа с кодом %q не найдена", inviteCode)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO group_members (user_id, group_id, role) VALUES ($1, $2, 'member')
		ON CONFLICT (user_id, group_id) DO NOTHING
	`, userID, group.ID); err != nil {
		return models.Group{}, fmt.Errorf("join group: %w", err)
	}

	return *group, nil
}

func (s *PostgresStorage) GetUserGroups(ctx context.Context, userID int64) ([]models.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.invite_code, g.created_at
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY g.created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	defer rows.Close()

	var groups []models.Group
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.InviteCode, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *PostgresStorage) GetGroupMembers(ctx context.Context, groupID int64) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.telegram_id, u.username, u.created_at
		FROM users u
		JOIN group_members gm ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get group members: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *PostgresStorage) CreateTransaction(ctx context.Context, t models.Transaction, splits []models.TransactionSplit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var txID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO transactions (group_id, payer_id, amount, description, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, t.GroupID, t.PayerID, t.Amount, t.Description, t.CreatedAt.UTC()).Scan(&txID)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	for _, sp := range splits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES ($1, $2, $3)
		`, txID, sp.UserID, sp.Amount); err != nil {
			return fmt.Errorf("insert split: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStorage) GetGroupBalances(ctx context.Context, groupID int64) (map[int64]int64, error) {
	balances := make(map[int64]int64)

	rows, err := s.db.QueryContext(ctx, `SELECT payer_id, amount FROM transactions WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get transactions: %w", err)
	}
	for rows.Next() {
		var payerID, amount int64
		if err := rows.Scan(&payerID, &amount); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		balances[payerID] += amount
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `
		SELECT ts.user_id, ts.amount
		FROM transaction_splits ts
		JOIN transactions t ON t.id = ts.transaction_id
		WHERE t.group_id = $1
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get splits: %w", err)
	}
	for rows.Next() {
		var userID, amount int64
		if err := rows.Scan(&userID, &amount); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan split: %w", err)
		}
		balances[userID] -= amount
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT from_user_id, to_user_id, amount FROM settlements WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get settlements: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var from, to, amount int64
		if err := rows.Scan(&from, &to, &amount); err != nil {
			return nil, fmt.Errorf("scan settlement: %w", err)
		}
		balances[from] += amount
		balances[to] -= amount
	}
	return balances, rows.Err()
}

func (s *PostgresStorage) CreateSettlement(ctx context.Context, st models.Settlement) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settlements (group_id, from_user_id, to_user_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, st.GroupID, st.FromUserID, st.ToUserID, st.Amount, st.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("create settlement: %w", err)
	}
	return nil
}
