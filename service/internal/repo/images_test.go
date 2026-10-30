package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestInsertAndGetImage(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()

	in := Image{
		StorageName:        "0123456789abcdef0123456789abcdef.jpg",
		OriginalFilename:   "cake.jpg",
		MimeType:           "image/jpeg",
		SizeBytes:          1234,
		Width:              640,
		Height:             480,
		SHA256:             "ab",
		TableConfiguration: "recipes",
		ColumnName:         "main_picture",
		UploadedBy:         sql.NullInt64{Int64: 1, Valid: true},
	}
	saved, err := r.InsertImage(ctx, in)
	if err != nil {
		t.Fatalf("InsertImage: %v", err)
	}
	if saved.ID <= 0 {
		t.Fatalf("expected an id, got %d", saved.ID)
	}
	got, err := r.GetImage(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if got != saved {
		t.Fatalf("GetImage = %+v, want %+v", got, saved)
	}
	if _, err := r.GetImage(ctx, saved.ID+1); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("expected ErrImageNotFound, got %v", err)
	}
}

func TestAddColumnImageIsBigint(t *testing.T) {
	typ, def := addColumnSQLType(FieldSpec{Type: SemanticTypeImage}, false)
	if typ != "BIGINT" || def != "" {
		t.Fatalf("image column type = %q %q", typ, def)
	}
}
