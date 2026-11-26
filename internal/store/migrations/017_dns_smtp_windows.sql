CREATE TABLE dns_windows (
  start_ms INTEGER PRIMARY KEY,
  queries INTEGER NOT NULL,
  failures INTEGER NOT NULL,
  by_rcode TEXT NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL
);

CREATE TABLE smtp_windows (
  start_ms INTEGER PRIMARY KEY,
  attempts INTEGER NOT NULL,
  delivered INTEGER NOT NULL,
  rejected INTEGER NOT NULL,
  aborted INTEGER NOT NULL,
  by_code TEXT NOT NULL,
  p50_ms REAL NOT NULL,
  p95_ms REAL NOT NULL
);

CREATE TABLE smtp_queue (
  time_ms INTEGER PRIMARY KEY,
  messages INTEGER NOT NULL
);
