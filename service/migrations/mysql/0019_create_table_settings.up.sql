-- Create table_settings table (MySQL)
CREATE TABLE IF NOT EXISTS table_settings (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    setting_key VARCHAR(191) NOT NULL UNIQUE,
    string_val TEXT,
    bool_val BOOLEAN,
    title VARCHAR(255),
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Insert initial appTitle setting
INSERT INTO table_settings (setting_key, string_val, title)
VALUES ('appTitle', 'SickRock', 'Application Title')
ON DUPLICATE KEY UPDATE string_val = 'SickRock', title = 'Application Title';
