CREATE TABLE IF NOT EXISTS site_contents (
  key TEXT PRIMARY KEY,
  nl_text TEXT NOT NULL,
  en_text TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by TEXT NOT NULL
);

-- Preload keys that make sense to be editable
INSERT INTO site_contents (key, nl_text, en_text, updated_by) VALUES
  ('landing_title', '', '', 'system'),
  ('landing_description', '', '', 'system'),
  ('landing_problem_title', '', '', 'system'),
  ('landing_problem_text', '', '', 'system'),
  ('landing_solution_title', '', '', 'system'),
  ('landing_solution_text', '', '', 'system'),
  ('landing_features_title', '', '', 'system'),
  ('landing_features_desc', '', '', 'system'),
  ('about_mission_title', '', '', 'system'),
  ('about_mission_p1', '', '', 'system'),
  ('about_mission_p2', '', '', 'system'),
  ('privacy_intro_title', '', '', 'system'),
  ('privacy_intro_text', '', '', 'system')
ON CONFLICT (key) DO NOTHING;
