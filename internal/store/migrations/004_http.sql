CREATE TABLE http_windows (
  start_ms INTEGER PRIMARY KEY,
  requests INTEGER NOT NULL,
  by_status TEXT NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL
);

CREATE TABLE http_routes (
  start_ms INTEGER NOT NULL,
  method TEXT NOT NULL,
  route TEXT NOT NULL,
  requests INTEGER NOT NULL,
  errors INTEGER NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL,
  hist TEXT NOT NULL,
  PRIMARY KEY (start_ms, method, route)
);
