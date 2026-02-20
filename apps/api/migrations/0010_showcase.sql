CREATE TABLE IF NOT EXISTS showcase_items (
  slot INTEGER PRIMARY KEY CHECK (slot IN (1, 2, 3, 4)),
  report_photo_id INTEGER NOT NULL REFERENCES report_photos(id) ON DELETE CASCADE,
  subtitle TEXT NOT NULL,
  focal_x INTEGER NOT NULL DEFAULT 50 CHECK (focal_x >= 0 AND focal_x <= 100),
  focal_y INTEGER NOT NULL DEFAULT 50 CHECK (focal_y >= 0 AND focal_y <= 100),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
