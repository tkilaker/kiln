-- Audio files table for TTS-generated audio
-- Stores metadata about generated audio files for articles

CREATE TABLE IF NOT EXISTS audio_files (
  id SERIAL PRIMARY KEY,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  voice TEXT NOT NULL DEFAULT 'alloy',
  file_path TEXT NOT NULL,
  file_size BIGINT DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  error_message TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index on article_id for lookups
CREATE INDEX IF NOT EXISTS idx_audio_files_article_id ON audio_files(article_id);

-- Unique constraint: one audio file per article per voice
CREATE UNIQUE INDEX IF NOT EXISTS idx_audio_files_article_voice ON audio_files(article_id, voice);

-- Trigger to automatically update updated_at
CREATE TRIGGER update_audio_files_updated_at BEFORE UPDATE ON audio_files
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
