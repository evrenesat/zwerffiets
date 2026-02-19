CREATE INDEX IF NOT EXISTS idx_reports_fingerprint_hash ON reports(fingerprint_hash);
CREATE INDEX IF NOT EXISTS idx_reports_reporter_hash ON reports(reporter_hash);
CREATE INDEX IF NOT EXISTS idx_opmlt_expires_at ON operator_magic_link_tokens(expires_at);

ALTER TABLE reports
  DROP CONSTRAINT IF EXISTS reports_dedupe_group_id_fkey;

ALTER TABLE reports
  ADD CONSTRAINT reports_dedupe_group_id_fkey
  FOREIGN KEY (dedupe_group_id) REFERENCES dedupe_groups(id) ON DELETE SET NULL;

ALTER TABLE dedupe_groups
  DROP CONSTRAINT IF EXISTS dedupe_groups_canonical_report_fk;

ALTER TABLE dedupe_groups
  ADD CONSTRAINT dedupe_groups_canonical_report_fk
  FOREIGN KEY (canonical_report_id) REFERENCES reports(id) ON DELETE SET NULL
  DEFERRABLE INITIALLY DEFERRED;

UPDATE operators
SET password_hash = crypt(gen_random_uuid()::text, gen_salt('bf'))
WHERE password_hash IS NULL;

ALTER TABLE operators
  ALTER COLUMN password_hash SET NOT NULL;
