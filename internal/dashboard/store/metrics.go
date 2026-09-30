package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
)

// 历史数据表
const (
	Metrics1m  = "metrics_1m"
	Metrics10m = "metrics_10m"
)

// MetricRow 某台服务器的一个指标点
type MetricRow struct {
	ServerID string
	metric.Point
}

// sqlFor 把模板中的 {t} 替换为经过校验的表名
func sqlFor(table, tmpl string) (string, error) {
	if table != Metrics1m && table != Metrics10m {
		return "", fmt.Errorf("非法的历史数据表: %q", table)
	}
	return strings.ReplaceAll(tmpl, "{t}", table), nil
}

// InsertMetrics 批量写入（同一服务器同一时间点覆盖）
func (s *Store) InsertMetrics(ctx context.Context, table string, rows []MetricRow) error {
	q, err := sqlFor(table, `INSERT OR REPLACE INTO {t} (server_id, ts, cpu, mem, disk, net_in, net_out, load1, tcp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, q)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range rows {
			if _, err := stmt.ExecContext(ctx, r.ServerID, r.TS, r.CPU, r.Mem, r.Disk, r.NetIn, r.NetOut, r.Load1, r.TCP); err != nil {
				return err
			}
		}
		return nil
	})
}

// QueryMetrics 查询 from <= ts < to 的数据，按时间升序
func (s *Store) QueryMetrics(ctx context.Context, table, serverID string, from, to int64) ([]metric.Point, error) {
	q, err := sqlFor(table, `SELECT ts, cpu, mem, disk, net_in, net_out, load1, tcp FROM {t} WHERE server_id = ? AND ts >= ? AND ts < ? ORDER BY ts`)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, q, serverID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pts := []metric.Point{}
	for rows.Next() {
		var p metric.Point
		if err := rows.Scan(&p.TS, &p.CPU, &p.Mem, &p.Disk, &p.NetIn, &p.NetOut, &p.Load1, &p.TCP); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// Rollup10m 把 [from, to) 内的 1 分钟数据按服务器取平均，写入 10 分钟表（时间戳为 from）
func (s *Store) Rollup10m(ctx context.Context, from, to int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO metrics_10m (server_id, ts, cpu, mem, disk, net_in, net_out, load1, tcp)
 SELECT server_id, ?, AVG(cpu), AVG(mem), AVG(disk), AVG(net_in), AVG(net_out), AVG(load1), AVG(tcp)
 FROM metrics_1m WHERE ts >= ? AND ts < ? GROUP BY server_id`, from, from, to)
	return err
}

// CountMetrics 返回 from <= ts < to 的数据点数
func (s *Store) CountMetrics(ctx context.Context, table, serverID string, from, to int64) (int, error) {
	q, err := sqlFor(table, `SELECT COUNT(*) FROM {t} WHERE server_id = ? AND ts >= ? AND ts < ?`)
	if err != nil {
		return 0, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, q, serverID, from, to).Scan(&n)
	return n, err
}

// DeleteMetricsBefore 删除早于 ts 的数据，返回删除行数
func (s *Store) DeleteMetricsBefore(ctx context.Context, table string, ts int64) (int64, error) {
	q, err := sqlFor(table, `DELETE FROM {t} WHERE ts < ?`)
	if err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, q, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
