package store

import (
	"context"
	"database/sql"
	"errors"
)

// TrafficRow 某台服务器在某个计费周期内的流量
type TrafficRow struct {
	ServerID string
	Period   string // 周期起始日 YYYY-MM-DD
	In       int64
	Out      int64
	LastIn   int64 // 最近一次读到的累计计数，用于计算增量
	LastOut  int64
}

// GetTraffic 读取某周期的流量记录
func (s *Store) GetTraffic(ctx context.Context, serverID, period string) (TrafficRow, error) {
	r := TrafficRow{ServerID: serverID, Period: period}
	err := s.db.QueryRowContext(ctx, `SELECT in_bytes, out_bytes, last_in_total, last_out_total FROM traffic_monthly WHERE server_id = ? AND period = ?`,
		serverID, period).Scan(&r.In, &r.Out, &r.LastIn, &r.LastOut)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// SaveTraffic 批量保存流量记录
func (s *Store) SaveTraffic(ctx context.Context, rows []TrafficRow) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO traffic_monthly (server_id, period, in_bytes, out_bytes, last_in_total, last_out_total)
 VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(server_id, period) DO UPDATE SET in_bytes = excluded.in_bytes,
 out_bytes = excluded.out_bytes, last_in_total = excluded.last_in_total, last_out_total = excluded.last_out_total`,
				r.ServerID, r.Period, r.In, r.Out, r.LastIn, r.LastOut); err != nil {
				return err
			}
		}
		return nil
	})
}

// DailyTraffic 某一天的流量（日期为 Dashboard 本地时区）
type DailyTraffic struct {
	Day string `json:"day"`
	In  int64  `json:"in"`
	Out int64  `json:"out"`
}

// DailyDelta 某台服务器某一天待累加的流量增量
type DailyDelta struct {
	ServerID string
	Day      string
	In       int64
	Out      int64
}

// AddTrafficDaily 把增量累加到每日流量
func (s *Store) AddTrafficDaily(ctx context.Context, rows []DailyDelta) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO traffic_daily (server_id, day, in_bytes, out_bytes) VALUES (?, ?, ?, ?)
 ON CONFLICT(server_id, day) DO UPDATE SET in_bytes = in_bytes + excluded.in_bytes, out_bytes = out_bytes + excluded.out_bytes`,
				r.ServerID, r.Day, r.In, r.Out); err != nil {
				return err
			}
		}
		return nil
	})
}

// QueryTrafficDaily 返回 from <= day <= to 且有记录的每日流量，按日期升序
func (s *Store) QueryTrafficDaily(ctx context.Context, serverID, from, to string) ([]DailyTraffic, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT day, in_bytes, out_bytes FROM traffic_daily WHERE server_id = ? AND day >= ? AND day <= ? ORDER BY day`,
		serverID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []DailyTraffic{}
	for rows.Next() {
		var d DailyTraffic
		if err := rows.Scan(&d.Day, &d.In, &d.Out); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// DeleteTrafficDailyBefore 删除 day 之前的每日流量，返回删除行数
func (s *Store) DeleteTrafficDailyBefore(ctx context.Context, day string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM traffic_daily WHERE day < ?`, day)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
