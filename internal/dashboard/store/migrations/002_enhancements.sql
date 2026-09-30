ALTER TABLE servers ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN notify_muted INTEGER NOT NULL DEFAULT 0;

CREATE TABLE traffic_daily (
  server_id TEXT NOT NULL,
  day       TEXT NOT NULL,
  in_bytes  INTEGER NOT NULL DEFAULT 0,
  out_bytes INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (server_id, day)
) WITHOUT ROWID;

CREATE INDEX idx_traffic_daily_day ON traffic_daily (day);

CREATE TABLE notify_state (
  server_id TEXT NOT NULL,
  rule      TEXT NOT NULL,
  key       TEXT NOT NULL,
  sent_at   INTEGER NOT NULL,
  PRIMARY KEY (server_id, rule, key)
) WITHOUT ROWID;
