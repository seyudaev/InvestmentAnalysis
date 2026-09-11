package storage

import "time"

type User struct {
	ID         int64
	TelegramID int64
	CreatedAt  time.Time
}

type BrokerToken struct {
	ID           int64
	UserID       int64
	Broker       string // "tinkoff"
	TokenEnc     []byte
	AccountID    string
	CreatedAt    time.Time
}

type TargetAllocation struct {
	ID        int64
	UserID    int64
	AssetType string  // "share", "bond", "etf", "currency", "other"
	TargetPct float64 // 0-100
	UpdatedAt time.Time
}

type UserState struct {
	UserID    int64
	State     string // "", "awaiting_token"
	UpdatedAt time.Time
}
