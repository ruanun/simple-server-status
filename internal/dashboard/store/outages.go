package store

import (
	"context"
	"database/sql"
)

// Outage 一次离线记录；EndAt 为 nil 表示仍离线
type Outage struct {
	ID       int64  `json:"id"`
	ServerID string `json:"server_id"`
	StartAt  int64  `json:"start_at"`
	EndAt    *int64 `json:"end_at"`
}

// CreateOutage 新建进行中的离线记录
func (s *Store) CreateOutage(ctx context.Context, serverID string, start int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO outages (server_id, start_at) VALUES (?, ?)`, serverID, start)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CloseOutage 结束离线记录
func (s *Store) CloseOutage(ctx context.Context, id, end int64) error {
	return affected(s.db.ExecContext(ctx, `UPDATE outages SET end_at = ? WHERE id = ?`, end, id))
}

func scanOutages(rows *sql.Rows) ([]Outage, error) {
	defer rows.Close()
	list := []Outage{}
	for rows.Next() {
		var o Outage
		var end sql.NullInt64
		if err := rows.Scan(&o.ID, &o.ServerID, &o.StartAt, &end); err != nil {
			return nil, err
		}
		if end.Valid {
			v := end.Int64
			o.EndAt = &v
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

// OpenOutages 返回全部进行中的离线记录
func (s *Store) OpenOutages(ctx context.Context) ([]Outage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, start_at, end_at FROM outages WHERE end_at IS NULL`)
	if err != nil {
		return nil, err
	}
	return scanOutages(rows)
}

// ListOutages 分页查询离线记录（serverID 为空表示全部），按开始时间倒序，同时返回总数
func (s *Store) ListOutages(ctx context.Context, serverID string, limit, offset int) ([]Outage, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outages WHERE (? = '' OR server_id = ?)`, serverID, serverID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, start_at, end_at FROM outages WHERE (? = '' OR server_id = ?)
 ORDER BY start_at DESC, id DESC LIMIT ? OFFSET ?`, serverID, serverID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	list, err := scanOutages(rows)
	return list, total, err
}

// DeleteOutagesBefore 删除结束时间早于 ts 的离线记录（进行中的保留）
func (s *Store) DeleteOutagesBefore(ctx context.Context, ts int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM outages WHERE end_at IS NOT NULL AND end_at < ?`, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteOrphanOutages 删除所属服务器已不存在的离线记录（删除服务器与检查并发时可能残留）
func (s *Store) DeleteOrphanOutages(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM outages WHERE server_id NOT IN (SELECT id FROM servers)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
