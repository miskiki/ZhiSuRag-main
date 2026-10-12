-- Rollback: drop the session management columns added by 000013.
DROP INDEX IF EXISTS idx_auth_tokens_session_id;
ALTER TABLE auth_tokens DROP COLUMN device_ip;
ALTER TABLE auth_tokens DROP COLUMN device_label;
ALTER TABLE auth_tokens DROP COLUMN session_id;
