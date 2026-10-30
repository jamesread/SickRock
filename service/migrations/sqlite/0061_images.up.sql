-- Uploaded images. The file lives in the images directory (images_dir) as storage_name; image
-- columns (semantic "image") hold an images.id. table_configuration/column_name record the column
-- the image was uploaded for, and govern who may view it.
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_name TEXT NOT NULL UNIQUE,
    original_filename TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL,
    table_configuration TEXT NOT NULL DEFAULT '',
    column_name TEXT NOT NULL DEFAULT '',
    uploaded_by INTEGER,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP
);
