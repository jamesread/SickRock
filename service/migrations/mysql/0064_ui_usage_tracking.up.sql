-- Opt-in UI usage tracking ("Track and log UI usage" in the user control panel). The log itself is
-- a JSONL file per user in ui_usage_dir, not a table.
ALTER TABLE user_accounts ADD COLUMN ui_usage_tracking TINYINT(1) NOT NULL DEFAULT 0;
