CREATE TABLE scans (
  id INTEGER PRIMARY KEY,
  target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  tools TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  created_ms INTEGER NOT NULL,
  started_ms INTEGER NOT NULL DEFAULT 0,
  finished_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX scans_target ON scans (target_id, id);

CREATE TABLE findings (
  scan_id INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
  tool TEXT NOT NULL,
  title TEXT NOT NULL,
  evidence TEXT NOT NULL,
  severity TEXT NOT NULL,
  port INTEGER NOT NULL,
  fingerprint TEXT NOT NULL,
  PRIMARY KEY (scan_id, fingerprint)
);
