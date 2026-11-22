-- Each user's home page layout (section order and visibility, quick access bookmarks) as JSON;
-- NULL is the default layout.
ALTER TABLE user_accounts ADD COLUMN home_layout_json TEXT;
