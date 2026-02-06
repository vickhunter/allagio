ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS user_program_id UUID REFERENCES user_programs(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_recommendations_user_program ON recommendations(user_program_id);
