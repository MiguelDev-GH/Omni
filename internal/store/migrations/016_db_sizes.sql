CREATE TABLE db_sizes (
  time_ms INTEGER NOT NULL,
  name TEXT NOT NULL,
  bytes INTEGER NOT NULL,
  PRIMARY KEY (time_ms, name)
);
