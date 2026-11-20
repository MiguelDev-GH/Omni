CREATE TABLE check_results (
  id INTEGER PRIMARY KEY,
  target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  port INTEGER NOT NULL,
  kind TEXT NOT NULL,
  up INTEGER NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms REAL NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  checked_at_ms INTEGER NOT NULL
);
CREATE INDEX check_results_target_time ON check_results (target_id, checked_at_ms);
