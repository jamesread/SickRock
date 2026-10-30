-- Uploaded images. The file lives in the images directory (images_dir) as storage_name; image
-- columns (semantic "image") hold an images.id. table_configuration/column_name record the column
-- the image was uploaded for, and govern who may view it.
CREATE TABLE IF NOT EXISTS images (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    storage_name VARCHAR(191) NOT NULL,
    original_filename VARCHAR(255) NOT NULL DEFAULT '',
    mime_type VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    sha256 CHAR(64) NOT NULL,
    table_configuration VARCHAR(191) NOT NULL DEFAULT '',
    column_name VARCHAR(191) NOT NULL DEFAULT '',
    uploaded_by BIGINT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_images_storage_name (storage_name)
);
