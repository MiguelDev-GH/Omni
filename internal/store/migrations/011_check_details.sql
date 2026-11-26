ALTER TABLE check_results ADD COLUMN degraded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE check_results ADD COLUMN detail TEXT NOT NULL DEFAULT '';
-- -1 = não se aplica (fora do SMTP), 0 = sem STARTTLS, 1 = com STARTTLS
ALTER TABLE check_results ADD COLUMN starttls INTEGER NOT NULL DEFAULT -1;
