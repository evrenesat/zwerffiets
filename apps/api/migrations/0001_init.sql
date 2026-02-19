CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS operators (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT,
  role TEXT NOT NULL CHECK (role IN ('admin', 'municipality_operator')),
  municipality TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  receives_reports BOOLEAN NOT NULL DEFAULT FALSE,
  unsubscribe_requested BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  display_name TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tags (
  id SERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bike_groups (
  id SERIAL PRIMARY KEY,
  anchor_lat DOUBLE PRECISION NOT NULL,
  anchor_lng DOUBLE PRECISION NOT NULL,
  last_report_at TIMESTAMPTZ NOT NULL,
  total_reports INTEGER NOT NULL DEFAULT 0,
  unique_reporters INTEGER NOT NULL DEFAULT 0,
  same_reporter_reconfirmations INTEGER NOT NULL DEFAULT 0,
  distinct_reporter_reconfirmations INTEGER NOT NULL DEFAULT 0,
  first_qualifying_reconfirmation_at TIMESTAMPTZ,
  last_qualifying_reconfirmation_at TIMESTAMPTZ,
  signal_strength TEXT NOT NULL CHECK (signal_strength IN ('none', 'weak_same_reporter', 'strong_distinct_reporters')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS dedupe_groups (
  id SERIAL PRIMARY KEY,
  canonical_report_id INTEGER,
  merged_report_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reports (
  id SERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN ('new', 'triaged', 'forwarded', 'resolved', 'invalid')),
  lat DOUBLE PRECISION NOT NULL,
  lng DOUBLE PRECISION NOT NULL,
  accuracy_m DOUBLE PRECISION NOT NULL,
  tags JSONB NOT NULL,
  note TEXT,
  dedupe_group_id INTEGER REFERENCES dedupe_groups(id),
  source TEXT NOT NULL,
  fingerprint_hash TEXT NOT NULL,
  reporter_hash TEXT NOT NULL,
  reporter_email TEXT,
  reporter_email_confirmed BOOLEAN NOT NULL DEFAULT FALSE,
  user_id INTEGER REFERENCES users(id),
  flagged_for_review BOOLEAN NOT NULL DEFAULT FALSE,
  bike_group_id INTEGER NOT NULL REFERENCES bike_groups(id),
  address TEXT,
  city TEXT,
  postcode TEXT,
  municipality TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE dedupe_groups
  ADD CONSTRAINT dedupe_groups_canonical_report_fk
  FOREIGN KEY (canonical_report_id) REFERENCES reports(id)
  DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE IF NOT EXISTS report_photos (
  id SERIAL PRIMARY KEY,
  report_id INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
  storage_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  filename TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS report_events (
  id SERIAL PRIMARY KEY,
  report_id INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  actor TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS exports (
  id SERIAL PRIMARY KEY,
  period_type TEXT NOT NULL CHECK (period_type IN ('weekly', 'monthly', 'all')),
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  generated_by TEXT NOT NULL,
  row_count INTEGER NOT NULL,
  csv_path TEXT,
  geojson_path TEXT,
  pdf_path TEXT,
  filter_status TEXT,
  filter_municipality TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS magic_link_tokens (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS operator_magic_link_tokens (
  id SERIAL PRIMARY KEY,
  operator_id INTEGER NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_reports_created_at ON reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_bike_group_id ON reports(bike_group_id);
CREATE INDEX IF NOT EXISTS idx_reports_municipality ON reports (municipality);
CREATE INDEX IF NOT EXISTS idx_reports_user_id ON reports(user_id);
CREATE INDEX IF NOT EXISTS idx_reports_reporter_email ON reports(reporter_email) WHERE reporter_email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_report_events_report_id ON report_events(report_id, created_at);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_user_id ON magic_link_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_expires_at ON magic_link_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_opmlt_operator_id ON operator_magic_link_tokens(operator_id);

-- Seed Tags
INSERT INTO tags (code, label, is_active) VALUES
  ('flat_tires', 'Flat tires', TRUE),
  ('rusted', 'Rusted', TRUE),
  ('missing_parts', 'Missing parts', TRUE),
  ('blocking_sidewalk', 'Blocking sidewalk', TRUE),
  ('damaged_frame', 'Damaged frame', TRUE),
  ('abandoned_long_time', 'Abandoned for long time', TRUE),
  ('no_chain', 'No chain', TRUE),
  ('wheel_missing', 'Missing wheel', TRUE),
  ('no_seat', 'No seat', TRUE),
  ('other_visibility_issue', 'Other visibility issue', TRUE)
ON CONFLICT (code) DO NOTHING;
