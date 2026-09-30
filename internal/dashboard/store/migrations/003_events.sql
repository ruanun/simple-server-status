CREATE TABLE outages (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id TEXT NOT NULL,
  start_at  INTEGER NOT NULL,
  end_at    INTEGER
);
CREATE INDEX idx_outages_server ON outages (server_id, start_at);
CREATE INDEX idx_outages_start ON outages (start_at);

CREATE TABLE notify_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id   TEXT NOT NULL DEFAULT '',
  server_name TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL,
  channel     TEXT NOT NULL,
  title       TEXT NOT NULL DEFAULT '',
  message     TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL,
  error       TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  done_at     INTEGER
);
CREATE INDEX idx_notify_log_created ON notify_log (created_at);
CREATE INDEX idx_notify_log_server ON notify_log (server_id, created_at);
