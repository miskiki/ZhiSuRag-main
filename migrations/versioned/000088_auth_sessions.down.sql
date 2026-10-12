-- Rollback: drop the session management columns added by 000088.
DROP INDEX IF EXISTS idx_auth_tokens_session_id;
ALTER TABLE auth_tokens DROP COLUMN IF EXISTS device_ip;
ALTER TABLE auth_tokens DROP COLUMN IF EXISTS device_label;
ALTER TABLE auth_tokens DROP COLUMN IF EXISTS session_id;
