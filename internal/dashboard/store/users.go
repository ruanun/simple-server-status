package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// User 管理员账户
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	TokenVersion int
	CreatedAt    int64
}

const userCols = `id, username, password_hash, token_version, created_at`

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TokenVersion, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// CountUsers 用户数量
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser 新建用户并回填 ID
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	u.CreatedAt = time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, token_version, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.TokenVersion, u.CreatedAt)
	if err != nil {
		return err
	}
	u.ID, err = res.LastInsertId()
	return err
}

// GetUserByName 按用户名读取
func (s *Store) GetUserByName(ctx context.Context, name string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, name))
}

// GetUser 按 ID 读取
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// SetPassword 修改密码并使旧 token 失效
func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	return affected(s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?`, hash, id))
}
