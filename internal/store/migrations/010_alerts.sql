CREATE TABLE alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  time_ms INTEGER NOT NULL,
  rule TEXT NOT NULL,
  service TEXT NOT NULL,
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  level TEXT NOT NULL,
  previous TEXT NOT NULL,
  value REAL NOT NULL,
  threshold REAL NOT NULL,
  unit TEXT NOT NULL,
  detail TEXT NOT NULL
);
CREATE INDEX alerts_time ON alerts (time_ms);
