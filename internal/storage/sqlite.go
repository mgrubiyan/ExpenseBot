package storage

import (
	"ExpenseBot/internal/models"
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS expenses (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			tag        TEXT    NOT NULL,
			amount     INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS users (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER NOT NULL UNIQUE,
			username    TEXT    NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS groups (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			invite_code TEXT NOT NULL UNIQUE,
			created_at  DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS group_members (
			user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			role      TEXT NOT NULL,
			joined_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, group_id)
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id    INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			payer_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount      INTEGER NOT NULL,
			description TEXT NOT NULL,
			created_at  DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS transaction_splits (
			transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount         INTEGER NOT NULL,
			PRIMARY KEY (transaction_id, user_id)
		);

		CREATE TABLE IF NOT EXISTS settlements (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id     INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			from_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			to_user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount       INTEGER NOT NULL,
			created_at   DATETIME NOT NULL
		);
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

func (s *SQLiteStorage) GetOrCreateUser(ctx context.Context, telegramID int64, username string) (models.User, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (telegram_id, username, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(telegram_id) DO UPDATE SET username = excluded.username
	`, telegramID, username, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return models.User{}, fmt.Errorf("upsert user: %w", err)
	}

	var u models.User
	var createdAt string
	err = s.db.QueryRowContext(ctx, `
		SELECT id, telegram_id, username, created_at FROM users WHERE telegram_id = ?
	`, telegramID).Scan(&u.ID, &u.TelegramID, &u.Username, &createdAt)
	if err != nil {
		return models.User{}, fmt.Errorf("select user: %w", err)
	}
	u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return models.User{}, fmt.Errorf("parse time: %w", err)
	}
	return u, nil
}

func (s *SQLiteStorage) CreateGroup(ctx context.Context, name string, ownerUserID int64) (models.Group, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	for attempt := 0; attempt < 5; attempt++ {
		code := generateInviteCode()

		res, err := s.db.ExecContext(ctx, `
			INSERT INTO groups (name, invite_code, created_at) VALUES (?, ?, ?)
		`, name, code, now)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			return models.Group{}, fmt.Errorf("create group: %w", err)
		}

		groupID, err := res.LastInsertId()
		if err != nil {
			return models.Group{}, fmt.Errorf("get group id: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO group_members (user_id, group_id, role, joined_at) VALUES (?, ?, 'owner', ?)
		`, ownerUserID, groupID, now); err != nil {
			return models.Group{}, fmt.Errorf("add owner to group: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339, now)
		if err != nil {
			return models.Group{}, fmt.Errorf("parse time: %w", err)
		}

		return models.Group{ID: groupID, Name: name, InviteCode: code, CreatedAt: createdAt}, nil
	}

	return models.Group{}, fmt.Errorf("create group: could not generate a unique invite code")
}

func (s *SQLiteStorage) GetGroupByInviteCode(ctx context.Context, inviteCode string) (*models.Group, error) {
	var g models.Group
	var createdAt string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, invite_code, created_at FROM groups WHERE invite_code = ?
	`, strings.ToUpper(strings.TrimSpace(inviteCode))).Scan(&g.ID, &g.Name, &g.InviteCode, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by invite code: %w", err)
	}

	g.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse time: %w", err)
	}
	return &g, nil
}

func (s *SQLiteStorage) JoinGroupByInviteCode(ctx context.Context, inviteCode string, userID int64) (models.Group, error) {
	group, err := s.GetGroupByInviteCode(ctx, inviteCode)
	if err != nil {
		return models.Group{}, err
	}
	if group == nil {
		return models.Group{}, fmt.Errorf("группа с кодом %q не найдена", inviteCode)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO group_members (user_id, group_id, role, joined_at) VALUES (?, ?, 'member', ?)
	`, userID, group.ID, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return models.Group{}, fmt.Errorf("join group: %w", err)
	}

	return *group, nil
}

func (s *SQLiteStorage) GetUserGroups(ctx context.Context, userID int64) ([]models.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.invite_code, g.created_at
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = ?
		ORDER BY g.created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	defer rows.Close()

	var groups []models.Group
	for rows.Next() {
		var g models.Group
		var createdAt string
		if err := rows.Scan(&g.ID, &g.Name, &g.InviteCode, &createdAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		g.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *SQLiteStorage) GetGroupMembers(ctx context.Context, groupID int64) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.telegram_id, u.username, u.created_at
		FROM users u
		JOIN group_members gm ON gm.user_id = u.id
		WHERE gm.group_id = ?
		ORDER BY gm.joined_at
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get group members: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &createdAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLiteStorage) CreateTransaction(ctx context.Context, t models.Transaction, splits []models.TransactionSplit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (group_id, payer_id, amount, description, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, t.GroupID, t.PayerID, t.Amount, t.Description, t.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	txID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get transaction id: %w", err)
	}

	for _, sp := range splits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, ?)
		`, txID, sp.UserID, sp.Amount); err != nil {
			return fmt.Errorf("insert split: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStorage) GetGroupBalances(ctx context.Context, groupID int64) (map[int64]int64, error) {
	balances := make(map[int64]int64)

	rows, err := s.db.QueryContext(ctx, `SELECT payer_id, amount FROM transactions WHERE group_id = ?`, groupID)
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
		WHERE t.group_id = ?
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

	rows, err = s.db.QueryContext(ctx, `SELECT from_user_id, to_user_id, amount FROM settlements WHERE group_id = ?`, groupID)
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

func (s *SQLiteStorage) CreateSettlement(ctx context.Context, st models.Settlement) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settlements (group_id, from_user_id, to_user_id, amount, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, st.GroupID, st.FromUserID, st.ToUserID, st.Amount, st.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create settlement: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetGroupByID(ctx context.Context, groupID int64) (*models.Group, error) {
	var g models.Group
	var createdAt string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, invite_code, created_at FROM groups WHERE id = ?
	`, groupID).Scan(&g.ID, &g.Name, &g.InviteCode, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by id: %w", err)
	}

	g.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse time: %w", err)
	}
	return &g, nil
}
