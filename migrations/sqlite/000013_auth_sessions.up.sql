-- Mirrors versioned migration 000088_auth_sessions:
-- multi-device session management columns on auth_tokens.

ALTER TABLE auth_tokens ADD COLUMN session_id TEXT;
ALTER TABLE auth_tokens ADD COLUMN device_label TEXT DEFAULT '';
ALTER TABLE auth_tokens ADD COLUMN device_ip TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_auth_tokens_session_id ON auth_tokens(session_id);
