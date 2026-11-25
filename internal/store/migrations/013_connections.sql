CREATE TABLE conn_samples (
  time_ms INTEGER NOT NULL,
  port INTEGER NOT NULL,
  kind TEXT NOT NULL,
  established INTEGER NOT NULL,
  states TEXT NOT NULL,
  clients TEXT NOT NULL,
  PRIMARY KEY (time_ms, port)
);
