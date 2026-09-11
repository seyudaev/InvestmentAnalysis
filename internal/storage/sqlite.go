package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER NOT NULL UNIQUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS broker_tokens (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    broker      TEXT NOT NULL DEFAULT 'tinkoff',
    token_enc   BLOB NOT NULL,
    account_id  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, broker)
);

CREATE TABLE IF NOT EXISTS target_allocations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    asset_type  TEXT NOT NULL,
    target_pct  REAL NOT NULL,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, asset_type)
);

CREATE TABLE IF NOT EXISTS user_states (
    user_id     INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    state       TEXT NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) GetOrCreateUser(telegramID int64) (*User, error) {
	user, err := s.GetUserByTelegramID(telegramID)
	if err == nil {
		return user, nil
	}

	res, err := s.db.Exec(`INSERT INTO users (telegram_id) VALUES (?)`, telegramID)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{ID: id, TelegramID: telegramID, CreatedAt: time.Now()}, nil
}

func (s *Store) GetUserByTelegramID(telegramID int64) (*User, error) {
	row := s.db.QueryRow(`SELECT id, telegram_id, created_at FROM users WHERE telegram_id = ?`, telegramID)

	var u User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) SaveBrokerToken(userID int64, broker string, tokenEnc []byte, accountID string) error {
	_, err := s.db.Exec(`
		INSERT INTO broker_tokens (user_id, broker, token_enc, account_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, broker) DO UPDATE SET
			token_enc = excluded.token_enc,
			account_id = excluded.account_id,
			created_at = CURRENT_TIMESTAMP
	`, userID, broker, tokenEnc, accountID)
	return err
}

func (s *Store) GetBrokerToken(userID int64, broker string) (*BrokerToken, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, broker, token_enc, account_id, created_at
		FROM broker_tokens WHERE user_id = ? AND broker = ?
	`, userID, broker)

	var t BrokerToken
	if err := row.Scan(&t.ID, &t.UserID, &t.Broker, &t.TokenEnc, &t.AccountID, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) HasBrokerToken(userID int64, broker string) bool {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM broker_tokens WHERE user_id = ? AND broker = ?`, userID, broker).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) SetUserState(userID int64, state string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_states (user_id, state, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP
	`, userID, state)
	return err
}

func (s *Store) GetUserState(userID int64) (string, error) {
	var state string
	err := s.db.QueryRow(`SELECT state FROM user_states WHERE user_id = ?`, userID).Scan(&state)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return state, err
}

func (s *Store) ClearUserState(userID int64) error {
	return s.SetUserState(userID, "")
}

func (s *Store) SaveTargetAllocation(userID int64, assetType string, targetPct float64) error {
	_, err := s.db.Exec(`
		INSERT INTO target_allocations (user_id, asset_type, target_pct, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, asset_type) DO UPDATE SET
			target_pct = excluded.target_pct,
			updated_at = CURRENT_TIMESTAMP
	`, userID, assetType, targetPct)
	return err
}

func (s *Store) GetTargetAllocations(userID int64) ([]TargetAllocation, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, asset_type, target_pct, updated_at
		FROM target_allocations WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []TargetAllocation
	for rows.Next() {
		var a TargetAllocation
		if err := rows.Scan(&a.ID, &a.UserID, &a.AssetType, &a.TargetPct, &a.UpdatedAt); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, rows.Err()
}

func (s *Store) DeleteTargetAllocations(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM target_allocations WHERE user_id = ?`, userID)
	return err
}
