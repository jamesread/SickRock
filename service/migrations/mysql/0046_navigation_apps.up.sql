CREATE TABLE IF NOT EXISTS apps (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    ordinal INT NOT NULL DEFAULT 99
);

CREATE TABLE IF NOT EXISTS app_navigation_links (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    app_id INT NOT NULL,
    navigation_item_id INT NOT NULL,
    ordinal INT NOT NULL DEFAULT 99,
    UNIQUE KEY uk_app_nav_item (navigation_item_id),
    KEY idx_app_nav_links_app (app_id),
    CONSTRAINT fk_anl_app FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
    CONSTRAINT fk_anl_navigation FOREIGN KEY (navigation_item_id) REFERENCES table_navigation(id) ON DELETE CASCADE
);
