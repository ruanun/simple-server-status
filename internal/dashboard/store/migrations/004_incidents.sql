CREATE TABLE events (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id    TEXT    NOT NULL,
  kind         TEXT    NOT NULL,
  start_at     INTEGER NOT NULL,
  end_at       INTEGER,
  detail       TEXT    NOT NULL DEFAULT '{}',
  notify_state INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_events_server ON events (server_id, start_at);
CREATE INDEX idx_events_kind ON events (kind, start_at);
CREATE INDEX idx_events_start ON events (start_at);
CREATE INDEX idx_events_open ON events (server_id, kind) WHERE end_at IS NULL;

DROP TABLE outages;
DELETE FROM settings WHERE key IN ('notify_load_cpu', 'notify_load_mem', 'notify_load_disk', 'notify_load_minutes');

ALTER TABLE notify_log ADD COLUMN event_id INTEGER;
ALTER TABLE servers ADD COLUMN boot_at INTEGER NOT NULL DEFAULT 0;
