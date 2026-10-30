package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSaveAndOpen(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "images"))
	if err != nil {
		t.Fatal(err)
	}
	data := pngBytes(t, 3, 2)
	meta, err := store.Save(data)
	if err != nil {
		t.Fatal(err)
	}
	if meta.MimeType != "image/png" || meta.Width != 3 || meta.Height != 2 || meta.SizeBytes != int64(len(data)) || len(meta.SHA256) != 64 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if !storageNamePattern.MatchString(meta.StorageName) {
		t.Fatalf("storage name %q", meta.StorageName)
	}
	f, err := store.Open(meta.StorageName)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if !bytes.Equal(got, data) {
		t.Fatal("stored bytes differ")
	}
	if err := store.Remove(meta.StorageName); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(meta.StorageName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not exist after remove, got %v", err)
	}
}

func TestSaveRejectsNonImages(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	for _, data := range [][]byte{
		[]byte("hello"),
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
	} {
		if _, err := store.Save(data); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("expected ErrUnsupported for %q, got %v", data, err)
		}
	}
	if _, err := Inspect(make([]byte, MaxBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestOpenRejectsPathTraversal(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	for _, name := range []string{"../etc/passwd", "x.png", "/etc/passwd", ""} {
		if _, err := store.Open(name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Open(%q) = %v, want ErrNotExist", name, err)
		}
	}
}
