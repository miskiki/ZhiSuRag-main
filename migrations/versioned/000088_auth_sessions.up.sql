-- Description: Multi-device session management (方案 B：多端并存 + 会话管理面板).
-- auth_tokens rows double as login sessions: session_id groups the access +
-- refresh tokens minted by one login (refresh rotation keeps the session id),
-- device_label / device_ip record where the session was established. Columns
-- are nullable so tokens created before this migration degrade gracefully to
-- "early session" in the management UI.
DO $$ BEGIN RAISE NOTICE '[Migration 000088] Adding session management columns to auth_tokens'; END $$;

ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS session_id VARCHAR(36);
ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS device_label VARCHAR(255) DEFAULT '';
ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS device_ip VARCHAR(64) DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_auth_tokens_session_id ON auth_tokens(session_id);

COMMENT ON COLUMN auth_tokens.session_id IS 'Login session UUID shared by the access and refresh tokens of one login; refresh rotation inherits it';
COMMENT ON COLUMN auth_tokens.device_label IS 'Human-readable device description parsed from the login User-Agent';
COMMENT ON COLUMN auth_tokens.device_ip IS 'Client IP at session establishment (display/troubleshooting only)';
