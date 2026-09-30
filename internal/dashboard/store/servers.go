package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/randx"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// Server 服务器配置与最近一次的静态信息
type Server struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Secret          string       `json:"secret"`
	Group           string       `json:"group"`
	Country         string       `json:"country"`
	Sort            int          `json:"sort"`
	Hidden          bool         `json:"hidden"`
	Price           *float64     `json:"price"`
	Currency        string       `json:"currency"`
	BillingCycle    string       `json:"billing_cycle"`
	ExpireAt        *int64       `json:"expire_at"`
	TrafficLimit    *int64       `json:"traffic_limit"`
	TrafficMode     string       `json:"traffic_mode"`
	TrafficResetDay int          `json:"traffic_reset_day"`
	ReportInterval  int          `json:"report_interval"`
	NICInclude      []string     `json:"nic_include"`
	NICExclude      []string     `json:"nic_exclude"`
	MountExclude    []string     `json:"mount_exclude"`
	StaticInfo      *proto.Hello `json:"static_info"`
	LastIP          string       `json:"last_ip"`
	LastSeen        int64        `json:"last_seen"`
	CreatedAt       int64        `json:"created_at"`
	UpdatedAt       int64        `json:"updated_at"`
	Note            string       `json:"note"`
	NotifyMuted     bool         `json:"notify_muted"`
}

const selectCols = `id, name, secret, grp, country, sort, hidden, price, currency, billing_cycle,
 expire_at, traffic_limit, traffic_mode, traffic_reset_day, report_interval,
 nic_include, nic_exclude, mount_exclude, static_info, last_ip, last_seen, created_at, updated_at, note, notify_muted`

const insertSQL = `INSERT INTO servers (id, name, secret, grp, country, sort, hidden, price, currency, billing_cycle,
 expire_at, traffic_limit, traffic_mode, traffic_reset_day, report_interval,
 nic_include, nic_exclude, mount_exclude, created_at, updated_at, note, notify_muted)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const upsertSQL = insertSQL + ` ON CONFLICT(id) DO UPDATE SET
 name=excluded.name, secret=excluded.secret, grp=excluded.grp, country=excluded.country, sort=excluded.sort,
 hidden=excluded.hidden, price=excluded.price, currency=excluded.currency, billing_cycle=excluded.billing_cycle,
 expire_at=excluded.expire_at, traffic_limit=excluded.traffic_limit, traffic_mode=excluded.traffic_mode,
 traffic_reset_day=excluded.traffic_reset_day, report_interval=excluded.report_interval,
 nic_include=excluded.nic_include, nic_exclude=excluded.nic_exclude, mount_exclude=excluded.mount_exclude,
 note=excluded.note, notify_muted=excluded.notify_muted, updated_at=excluded.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (Server, error) {
	var (
		s                     Server
		hidden                int
		price                 sql.NullFloat64
		expire, limit         sql.NullInt64
		nicIn, nicEx, mountEx string
		static                sql.NullString
		muted                 int
	)
	err := row.Scan(&s.ID, &s.Name, &s.Secret, &s.Group, &s.Country, &s.Sort, &hidden, &price, &s.Currency,
		&s.BillingCycle, &expire, &limit, &s.TrafficMode, &s.TrafficResetDay, &s.ReportInterval,
		&nicIn, &nicEx, &mountEx, &static, &s.LastIP, &s.LastSeen, &s.CreatedAt, &s.UpdatedAt, &s.Note, &muted)
	if err != nil {
		return s, err
	}
	s.Hidden = hidden != 0
	s.NotifyMuted = muted != 0
	if price.Valid {
		v := price.Float64
		s.Price = &v
	}
	if expire.Valid {
		v := expire.Int64
		s.ExpireAt = &v
	}
	if limit.Valid {
		v := limit.Int64
		s.TrafficLimit = &v
	}
	s.NICInclude = decodeList(nicIn)
	s.NICExclude = decodeList(nicEx)
	s.MountExclude = decodeList(mountEx)
	if static.Valid && static.String != "" {
		var h proto.Hello
		if json.Unmarshal([]byte(static.String), &h) == nil {
			s.StaticInfo = &h
		}
	}
	return s, nil
}

func decodeList(s string) []string {
	var l []string
	_ = json.Unmarshal([]byte(s), &l)
	if l == nil {
		l = []string{}
	}
	return l
}

func encodeList(l []string) string {
	if l == nil {
		return "[]"
	}
	b, _ := json.Marshal(l)
	return string(b)
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// applyDefaults 补全缺省字段
func applyDefaults(s *Server) {
	if s.TrafficMode == "" {
		s.TrafficMode = "sum"
	}
	if s.TrafficResetDay < 1 || s.TrafficResetDay > 28 {
		s.TrafficResetDay = 1
	}
	if s.ReportInterval < 1 {
		s.ReportInterval = 2
	}
	if s.NICInclude == nil {
		s.NICInclude = []string{}
	}
	if s.NICExclude == nil {
		s.NICExclude = []string{}
	}
	if s.MountExclude == nil {
		s.MountExclude = []string{}
	}
}

func insertArgs(s Server) []any {
	return []any{s.ID, s.Name, s.Secret, s.Group, s.Country, s.Sort, boolInt(s.Hidden), nullFloat(s.Price),
		s.Currency, s.BillingCycle, nullInt(s.ExpireAt), nullInt(s.TrafficLimit), s.TrafficMode,
		s.TrafficResetDay, s.ReportInterval, encodeList(s.NICInclude), encodeList(s.NICExclude),
		encodeList(s.MountExclude), s.CreatedAt, s.UpdatedAt, s.Note, boolInt(s.NotifyMuted)}
}

// ListServers 按 sort、name 返回全部服务器
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+selectCols+` FROM servers ORDER BY sort, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Server{}
	for rows.Next() {
		sv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, sv)
	}
	return list, rows.Err()
}

// GetServer 按 ID 读取服务器
func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	sv, err := scanServer(s.db.QueryRowContext(ctx, `SELECT `+selectCols+` FROM servers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return sv, ErrNotFound
	}
	return sv, err
}

// CreateServer 新建服务器，自动生成 ID、密钥与时间戳
func (s *Store) CreateServer(ctx context.Context, sv *Server) error {
	now := time.Now().Unix()
	if sv.ID == "" {
		sv.ID = randx.String(12)
	}
	if sv.Secret == "" {
		sv.Secret = randx.String(32)
	}
	sv.CreatedAt, sv.UpdatedAt = now, now
	applyDefaults(sv)
	_, err := s.db.ExecContext(ctx, insertSQL, insertArgs(*sv)...)
	return err
}

// UpdateServer 更新可编辑字段（不含密钥、排序与静态信息）
func (s *Store) UpdateServer(ctx context.Context, sv Server) error {
	applyDefaults(&sv)
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET name=?, grp=?, country=?, hidden=?, price=?,
 currency=?, billing_cycle=?, expire_at=?, traffic_limit=?, traffic_mode=?, traffic_reset_day=?, report_interval=?,
 nic_include=?, nic_exclude=?, mount_exclude=?, note=?, notify_muted=?, updated_at=? WHERE id=?`,
		sv.Name, sv.Group, sv.Country, boolInt(sv.Hidden), nullFloat(sv.Price), sv.Currency, sv.BillingCycle,
		nullInt(sv.ExpireAt), nullInt(sv.TrafficLimit), sv.TrafficMode, sv.TrafficResetDay, sv.ReportInterval,
		encodeList(sv.NICInclude), encodeList(sv.NICExclude), encodeList(sv.MountExclude), sv.Note, boolInt(sv.NotifyMuted),
		time.Now().Unix(), sv.ID))
}

// DeleteServer 删除服务器及其历史与流量数据
func (s *Store) DeleteServer(ctx context.Context, id string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{
			`DELETE FROM metrics_1m WHERE server_id = ?`,
			`DELETE FROM metrics_10m WHERE server_id = ?`,
			`DELETE FROM traffic_monthly WHERE server_id = ?`,
			`DELETE FROM traffic_daily WHERE server_id = ?`,
			`DELETE FROM notify_state WHERE server_id = ?`,
			`DELETE FROM outages WHERE server_id = ?`,
			`DELETE FROM notify_log WHERE server_id = ?`,
		} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
		return affected(tx.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id))
	})
}

// SetServerOrder 按 ids 顺序重写排序值
func (s *Store) SetServerOrder(ctx context.Context, ids []string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE servers SET sort = ? WHERE id = ?`, i, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// ResetSecret 生成新密钥
func (s *Store) ResetSecret(ctx context.Context, id string) (string, error) {
	sec := randx.String(32)
	err := affected(s.db.ExecContext(ctx, `UPDATE servers SET secret = ?, updated_at = ? WHERE id = ?`, sec, time.Now().Unix(), id))
	return sec, err
}

// SetStaticInfo 保存 Agent 最近一次上报的静态信息与来源 IP
func (s *Store) SetStaticInfo(ctx context.Context, id, ip string, h proto.Hello) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET static_info = ?, last_ip = ? WHERE id = ?`, string(b), ip, id))
}

// SetLastSeen 记录最后在线时间
func (s *Store) SetLastSeen(ctx context.Context, id string, ts int64) error {
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET last_seen = ? WHERE id = ?`, ts, id))
}

// UpsertServers 导入服务器：同 ID 覆盖配置（保留静态信息），新 ID 新增
func (s *Store) UpsertServers(ctx context.Context, list []Server) error {
	now := time.Now().Unix()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, sv := range list {
			applyDefaults(&sv)
			if sv.CreatedAt == 0 {
				sv.CreatedAt = now
			}
			sv.UpdatedAt = now
			if _, err := tx.ExecContext(ctx, upsertSQL, insertArgs(sv)...); err != nil {
				return err
			}
		}
		return nil
	})
}
