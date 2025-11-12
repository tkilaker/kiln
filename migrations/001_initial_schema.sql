-- Initial schema for Kiln
-- Creates the articles table for storing scraped content

CREATE TABLE IF NOT EXISTS articles (
  id SERIAL PRIMARY KEY,
  source TEXT NOT NULL DEFAULT 'gasetten',
  url TEXT UNIQUE NOT NULL,
  title TEXT,
  author TEXT,
  published_at TIMESTAMP,
  content_html TEXT,
  content_text TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index on created_at for sorting by newest first
CREATE INDEX IF NOT EXISTS idx_articles_created_at ON articles(created_at DESC);

-- Index on published_at for RSS feed ordering
CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);

-- Index on source for potential multi-source filtering
CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
CREATE TRIGGER update_articles_updated_at BEFORE UPDATE ON articles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
