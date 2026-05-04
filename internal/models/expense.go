package models

import "time"

type Expense struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Tag       string    `json:"tag"`
	Amount    int       `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}
