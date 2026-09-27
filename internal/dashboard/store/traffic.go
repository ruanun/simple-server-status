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
