package storage

import (
	"ExpenseBot/internal/models"
	"context"
	"time"
)

type Storage interface {

	// --- Individual methods ---

	AddExpense(ctx context.Context, expense models.Expense) error

	GetExpensesByPeriod(ctx context.Context, userID int64, from, to time.Time) ([]models.Expense, error)

	GetLastExpenses(ctx context.Context, userID int64, limit int) ([]models.Expense, error)

	DeleteLastExpense(ctx context.Context, userID int64) (*models.Expense, error)

	// --- Group methods ---

	// GetOrCreateUser upserts a user by Telegram ID, keeping the stored
	// username in sync, and returns the internal user record.
	GetOrCreateUser(ctx context.Context, telegramID int64, username string) (models.User, error)

	// CreateGroup creates a new group with a fresh invite code and adds
	// ownerUserID (internal user ID) as its owner.
	CreateGroup(ctx context.Context, name string, ownerUserID int64) (models.Group, error)

	// JoinGroupByInviteCode adds userID (internal user ID) to the group
	// identified by inviteCode as a member. Idempotent.
	JoinGroupByInviteCode(ctx context.Context, inviteCode string, userID int64) (models.Group, error)

	// GetGroupByInviteCode looks up a group by its invite code.
	// Returns (nil, nil) if no such group exists.
	GetGroupByInviteCode(ctx context.Context, inviteCode string) (*models.Group, error)

	// GetUserGroups returns every group userID (internal user ID) belongs to.
	GetUserGroups(ctx context.Context, userID int64) ([]models.Group, error)

	// GetGroupByID looks up a group by its internal ID.
	// Returns (nil, nil) if no such group exists.
	GetGroupByID(ctx context.Context, groupID int64) (*models.Group, error)

	// GetGroupMembers returns every user in the group.
	GetGroupMembers(ctx context.Context, groupID int64) ([]models.User, error)

	// CreateTransaction records a group expense together with how it's
	// split across members. Persisted atomically.
	CreateTransaction(ctx context.Context, t models.Transaction, splits []models.TransactionSplit) error

	// GetGroupBalances returns, per internal user ID, their net balance in
	// the group: positive means the group owes them, negative means they
	// owe the group (accounts for transactions and settlements).
	GetGroupBalances(ctx context.Context, groupID int64) (map[int64]int64, error)

	// CreateSettlement records that FromUserID paid ToUserID to reduce debt.
	CreateSettlement(ctx context.Context, s models.Settlement) error
}
