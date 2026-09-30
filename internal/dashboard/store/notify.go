package store

import "context"

// NotifySent 某条一次性提醒是否已发送
func (s *Store) NotifySent(ctx context.Context, serverID, rule, key string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notify_state WHERE server_id = ? AND rule = ? AND key = ?`, serverID, rule, key).Scan(&n)
	return n > 0, err
}

// MarkNotified 记录一次性提醒已发送
func (s *Store) MarkNotified(ctx context.Context, serverID, rule, key string, ts int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO notify_state (server_id, rule, key, sent_at) VALUES (?, ?, ?, ?)`, serverID, rule, key, ts)
	return err
}
