DELETE FROM table_recently_viewed;

ALTER TABLE table_recently_viewed
    ADD COLUMN user_account_id BIGINT NOT NULL AFTER id;

ALTER TABLE table_recently_viewed
    ADD UNIQUE KEY uk_recently_viewed_user (user_account_id, name, table_id);
