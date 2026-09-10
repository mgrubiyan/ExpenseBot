package models

import "time"

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	CreatedAt  time.Time
}

type Group struct {
	ID         int64
	Name       string
	InviteCode string
	CreatedAt  time.Time
}

type GroupMember struct {
	UserID   int64
	GroupID  int64
	Role     string // "owner" или "member"
	JoinedAt time.Time
}

type Transaction struct {
	ID          int64
	GroupID     int64
	PayerID     int64
	Amount      int64
	Description string
	CreatedAt   time.Time
}

type TransactionSplit struct {
	TransactionID int64
	UserID        int64
	Amount        int64
}

type Settlement struct {
	ID         int64
	GroupID    int64
	FromUserID int64
	ToUserID   int64
	Amount     int64
	CreatedAt  time.Time
}
