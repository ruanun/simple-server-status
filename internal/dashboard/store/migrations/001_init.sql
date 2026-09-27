CREATE TABLE servers (
  id                TEXT PRIMARY KEY,
  name              TEXT NOT NULL,
  secret            TEXT NOT NULL,
  grp               TEXT NOT NULL DEFAULT '',
  country           TEXT NOT NULL DEFAULT '',
  sort              INTEGER NOT NULL DEFAULT 0,
  hidden            INTEGER NOT NULL DEFAULT 0,
  price             REAL,
  currency          TEXT NOT NULL DEFAULT '',
  billing_cycle     TEXT NOT NULL DEFAULT '',
  expire_at         INTEGER,
  traffic_limit     INTEGER,
  traffic_mode      TEXT NOT NULL DEFAULT 'sum',
  traffic_reset_day INTEGER NOT NULL DEFAULT 1,
  report_interval   INTEGER NOT NULL DEFAULT 2,
  nic_include       TEXT NOT NULL DEFAULT '[]',
  nic_exclude       TEXT NOT NULL DEFAULT '[]',
  mount_exclude     TEXT NOT NULL DEFAULT '[]',
  static_info       TEXT,
  last_ip           TEXT NOT NULL DEFAULT '',
  last_seen         INTEGER NOT NULL DEFAULT 0,
  created_at        INTEGER NOT NULL,
  updated_at        INTEGER NOT NULL
);

CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  token_version INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL
);

CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE metrics_1m (
  server_id TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  cpu       REAL NOT NULL,
  mem       REAL NOT NULL,
  disk      REAL NOT NULL,
  net_in    REAL NOT NULL,
  net_out   REAL NOT NULL,
  load1     REAL NOT NULL,
  tcp       REAL NOT NULL,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE TABLE metrics_10m (
  server_id TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  cpu       REAL NOT NULL,
  mem       REAL NOT NULL,
  disk      REAL NOT NULL,
  net_in    REAL NOT NULL,
  net_out   REAL NOT NULL,
  load1     REAL NOT NULL,
  tcp       REAL NOT NULL,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE INDEX idx_metrics_1m_ts ON metrics_1m (ts);
CREATE INDEX idx_metrics_10m_ts ON metrics_10m (ts);

CREATE TABLE traffic_monthly (
  server_id      TEXT NOT NULL,
  period         TEXT NOT NULL,
  in_bytes       INTEGER NOT NULL DEFAULT 0,
  out_bytes      INTEGER NOT NULL DEFAULT 0,
  last_in_total  INTEGER NOT NULL DEFAULT 0,
  last_out_total INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (server_id, period)
) WITHOUT ROWID;
