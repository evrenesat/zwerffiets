-- Add name to operators
ALTER TABLE operators ADD COLUMN name TEXT;

-- Update existing operators to have a name based on email prefix
UPDATE operators SET name = INITCAP(SPLIT_PART(email, '@', 1)) WHERE name IS NULL;

-- Create blog_posts table
CREATE TABLE IF NOT EXISTS blog_posts (
  id SERIAL PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  content_html TEXT NOT NULL,
  author_id INTEGER REFERENCES operators(id) ON DELETE SET NULL,
  is_published BOOLEAN NOT NULL DEFAULT FALSE,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_blog_posts_slug ON blog_posts(slug);
CREATE INDEX IF NOT EXISTS idx_blog_posts_published ON blog_posts(is_published, published_at DESC) WHERE is_published = TRUE;

-- Create blog_media table
CREATE TABLE IF NOT EXISTS blog_media (
  id SERIAL PRIMARY KEY,
  filename TEXT NOT NULL UNIQUE,
  storage_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_blog_media_filename ON blog_media(filename);
