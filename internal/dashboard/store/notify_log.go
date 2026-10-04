package store

import (
	"context"
	"database/sql"
)

// 通知记录状态
const (
	LogPending = "pending"
	LogSent    = "sent"
	LogFailed  = "failed"
)

// NotifyLog 一条通知在一个渠道上的发送记录
type NotifyLog struct {
	ID         int64  `json:"id"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	EventID    *int64 `json:"event_id"` // 关联的事件；提醒与测试通知为 nil
	Kind       string `json:"kind"`
	Channel    string `json:"channel"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	CreatedAt  int64  `json:"created_at"`
	DoneAt     *int64 `json:"done_at"`
}

// AddNotifyLog 写入一条通知记录，返回其 ID
func (s *Store) AddNotifyLog(ctx context.Context, l NotifyLog) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO notify_log (server_id, server_name, event_id, kind, channel, title, message, status, error, created_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, l.ServerID, l.ServerName, nullInt(l.EventID), l.Kind, l.Channel, l.Title, l.Message, l.Status, l.Error, l.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishNotifyLog 更新通知记录的最终状态
func (s *Store) FinishNotifyLog(ctx context.Context, id int64, status, errMsg string, doneAt int64) error {
	return affected(s.db.ExecContext(ctx, `UPDATE notify_log SET status = ?, error = ?, done_at = ? WHERE id = ?`, status, errMsg, doneAt, id))
}

// ListNotifyLog 分页查询通知记录（serverID、status 为空表示不过滤），按时间倒序，同时返回总数
func (s *Store) ListNotifyLog(ctx context.Context, serverID, status string, limit, offset int) ([]NotifyLog, int, error) {
	const where = `WHERE (? = '' OR server_id = ?) AND (? = '' OR status = ?)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notify_log `+where, serverID, serverID, status, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, server_name, event_id, kind, channel, title, message, status, error, created_at, done_at
 FROM notify_log `+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, serverID, serverID, status, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []NotifyLog{}
	for rows.Next() {
		var l NotifyLog
		var done, eventID sql.NullInt64
		if err := rows.Scan(&l.ID, &l.ServerID, &l.ServerName, &eventID, &l.Kind, &l.Channel, &l.Title, &l.Message, &l.Status, &l.Error, &l.CreatedAt, &done); err != nil {
			return nil, 0, err
		}
		if done.Valid {
			v := done.Int64
			l.DoneAt = &v
		}
		if eventID.Valid {
			v := eventID.Int64
			l.EventID = &v
		}
		list = append(list, l)
	}
	return list, total, rows.Err()
}

// DeleteNotifyLogBefore 删除创建时间早于 ts 的通知记录
func (s *Store) DeleteNotifyLogBefore(ctx context.Context, ts int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notify_log WHERE created_at < ?`, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteOrphanNotifyLog 删除所属服务器已不存在的通知记录；测试消息（server_id 为空）保留
func (s *Store) DeleteOrphanNotifyLog(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notify_log WHERE server_id <> '' AND server_id NOT IN (SELECT id FROM servers)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FailPendingNotifyLog 把所有「发送中」的记录标记为失败（上次进程异常退出时遗留），返回更新数量
func (s *Store) FailPendingNotifyLog(ctx context.Context, errMsg string, doneAt int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE notify_log SET status = ?, error = ?, done_at = ? WHERE status = ?`, LogFailed, errMsg, doneAt, LogPending)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
