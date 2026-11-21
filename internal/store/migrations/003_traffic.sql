CREATE TABLE traffic_windows (
  start_ms INTEGER PRIMARY KEY,
  bytes INTEGER NOT NULL,
  packets INTEGER NOT NULL,
  dropped INTEGER NOT NULL DEFAULT 0,
  protocols TEXT NOT NULL,
  top_sources TEXT NOT NULL
);
