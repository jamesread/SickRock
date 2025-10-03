-- Add table_user_bookmarks table (MySQL)

CREATE TABLE IF NOT EXISTS table_user_bookmarks (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user INT NOT NULL,
    navigation_item INT NOT NULL
);
