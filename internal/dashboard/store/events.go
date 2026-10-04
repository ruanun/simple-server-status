package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// 事件的推送状态（events.notify_state）
const (
	NotifyUndecided = 0 // 尚未决定是否推送
	NotifyDone      = 1 // 已推送（至少一个渠道成功）
	NotifySkipped   = 2 // 不推送（未开启、静音、无渠道或发生在渠道启用前）
	NotifyFailed    = 3 // 推送失败（全部渠道在重试后仍失败），不再重试
	NotifySending   = 4 // 发送中
)

// Event 一条事件；时段事件进行中时 EndAt 为 nil，瞬时事件 EndAt 等于 StartAt
type Event struct {
	ID          int64           `json:"id"`
	ServerID    string          `json:"server_id"`
	Kind        string          `json:"kind"`
	StartAt     int64           `json:"start_at"`
	EndAt       *int64          `json:"end_at"`
	Detail      json.RawMessage `json:"detail"`
	NotifyState int             `json:"-"`
}

// EventFilter 事件查询条件；字段为空表示不过滤
type EventFilter struct {
	ServerID string
	Kinds    []string
}

// CreateEvent 新建事件并回填 ID；Detail 为空时写入 {}
func (s *Store) CreateEvent(ctx context.Context, e *Event) error {
	if len(e.Detail) == 0 {
		e.Detail = json.RawMessage("{}")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO events (server_id, kind, start_at, end_at, detail, notify_state) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ServerID, e.Kind, e.StartAt, nullInt(e.EndAt), string(e.Detail), e.NotifyState)
	if err != nil {
		return err
	}
	e.ID, err = res.LastInsertId()
	return err
}

// CloseEvent 结束进行中的事件
func (s *Store) CloseEvent(ctx context.Context, id, end int64) error {
	return affected(s.db.ExecContext(ctx, `UPDATE events SET end_at = ? WHERE id = ?`, end, id))
}

// SetEventDetail 更新事件详情
func (s *Store) SetEventDetail(ctx context.Context, id int64, detail json.RawMessage) error {
	return affected(s.db.ExecContext(ctx, `UPDATE events SET detail = ? WHERE id = ?`, string(detail), id))
}

// SetEventNotifyState 更新事件的推送状态，返回事件当前的结束时间（进行中为 nil）
func (s *Store) SetEventNotifyState(ctx context.Context, id int64, state int) (*int64, error) {
	var end sql.NullInt64
	err := s.db.QueryRowContext(ctx, `UPDATE events SET notify_state = ? WHERE id = ? RETURNING end_at`, state, id).Scan(&end)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil || !end.Valid {
		return nil, err
	}
	return &end.Int64, nil
}

const eventCols = `id, server_id, kind, start_at, end_at, detail, notify_state`

func scanEvents(rows *sql.Rows) ([]Event, error) {
	defer rows.Close()
	list := []Event{}
	for rows.Next() {
		var e Event
		var end sql.NullInt64
		var detail string
		if err := rows.Scan(&e.ID, &e.ServerID, &e.Kind, &e.StartAt, &end, &detail, &e.NotifyState); err != nil {
			return nil, err
		}
		if end.Valid {
			v := end.Int64
			e.EndAt = &v
		}
		e.Detail = json.RawMessage(detail)
		list = append(list, e)
	}
	return list, rows.Err()
}

// OpenEvents 返回全部进行中的事件
func (s *Store) OpenEvents(ctx context.Context) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventCols+` FROM events WHERE end_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// ListEvents 分页查询事件，按开始时间倒序，同时返回总数
func (s *Store) ListEvents(ctx context.Context, f EventFilter, limit, offset int) ([]Event, int, error) {
	// 按条件拼接 WHERE，让 SQLite 能用上 server_id 与 kind 的索引
	where := `WHERE 1 = 1`
	var args []any
	if f.ServerID != "" {
		where += ` AND server_id = ?`
		args = append(args, f.ServerID)
	}
	if len(f.Kinds) > 0 {
		where += ` AND kind IN (?` + strings.Repeat(`, ?`, len(f.Kinds)-1) + `)`
		for _, k := range f.Kinds {
			args = append(args, k)
		}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventCols+` FROM events `+where+` ORDER BY start_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := scanEvents(rows)
	return list, total, err
}

// DeleteEventsBefore 删除结束时间早于 ts 的事件（进行中的保留）
func (s *Store) DeleteEventsBefore(ctx context.Context, ts int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE end_at IS NOT NULL AND end_at < ?`, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteOrphanEvents 删除所属服务器已不存在的事件（删除服务器与检测并发时可能残留）
func (s *Store) DeleteOrphanEvents(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE server_id NOT IN (SELECT id FROM servers)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
