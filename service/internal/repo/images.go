package repo

import (
	"context"
	"database/sql"
	"errors"
)

// SemanticTypeImage marks a column holding an images.id; the frontend shows the image and edits it
// with an upload / camera capture field.
const SemanticTypeImage = "image"

// ErrImageNotFound is returned when no images row has the requested id.
var ErrImageNotFound = errors.New("image not found")

// Image is a row of the images system table.
type Image struct {
	StorageName        string        `db:"storage_name"`
	OriginalFilename   string        `db:"original_filename"`
	MimeType           string        `db:"mime_type"`
	SHA256             string        `db:"sha256"`
	TableConfiguration string        `db:"table_configuration"`
	ColumnName         string        `db:"column_name"`
	UploadedBy         sql.NullInt64 `db:"uploaded_by"`
	ID                 int64         `db:"id"`
	SizeBytes          int64         `db:"size_bytes"`
	Width              int           `db:"width"`
	Height             int           `db:"height"`
}

const imageSelectColumns = `id, storage_name, original_filename, mime_type, size_bytes, width, height, sha256, table_configuration, column_name, uploaded_by`

// InsertImage records an uploaded image file and returns it with its new id.
func (r *Repository) InsertImage(ctx context.Context, img Image) (Image, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO images (storage_name, original_filename, mime_type, size_bytes, width, height, sha256, table_configuration, column_name, uploaded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.StorageName, img.OriginalFilename, img.MimeType, img.SizeBytes, img.Width, img.Height, img.SHA256,
		img.TableConfiguration, img.ColumnName, img.UploadedBy)
	if err != nil {
		return Image{}, err
	}
	img.ID, err = res.LastInsertId()
	return img, err
}

// GetImage returns the images row with id, or ErrImageNotFound.
func (r *Repository) GetImage(ctx context.Context, id int64) (Image, error) {
	var img Image
	err := r.db.GetContext(ctx, &img, `SELECT `+imageSelectColumns+` FROM images WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrImageNotFound
	}
	return img, err
}
