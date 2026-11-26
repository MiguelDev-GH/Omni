CREATE TABLE captures (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  file TEXT NOT NULL DEFAULT '',
  started_ms INTEGER NOT NULL,
  finished_ms INTEGER,
  duration_s INTEGER NOT NULL,
  port INTEGER NOT NULL,
  packets INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  dropped INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX captures_started ON captures (started_ms);
