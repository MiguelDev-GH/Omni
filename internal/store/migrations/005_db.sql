CREATE TABLE db_windows (
  start_ms INTEGER PRIMARY KEY,
  queries INTEGER NOT NULL,
  errors INTEGER NOT NULL,
  by_kind TEXT NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL
);

CREATE TABLE db_queries (
  start_ms INTEGER NOT NULL,
  query TEXT NOT NULL,
  kind TEXT NOT NULL,
  count INTEGER NOT NULL,
  errors INTEGER NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL,
  hist TEXT NOT NULL,
  PRIMARY KEY (start_ms, query)
);
