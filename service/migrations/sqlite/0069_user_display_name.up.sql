-- Each user's own display name (User preferences), shown in the header instead of the username;
-- NULL shows the username.
ALTER TABLE user_accounts ADD COLUMN display_name TEXT;
