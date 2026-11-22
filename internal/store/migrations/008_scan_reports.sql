ALTER TABLE scans ADD COLUMN options TEXT NOT NULL DEFAULT '{}';

CREATE TABLE scan_reports (
  scan_id INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
  tool TEXT NOT NULL,
  report TEXT NOT NULL,
  PRIMARY KEY (scan_id, tool)
);
