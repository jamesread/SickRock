// Package images stores uploaded image files on the filesystem. Metadata about each file is kept in
// the images system table (see repo.InsertImage); this package only reads and writes the files.
package images

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register decoders for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"

	_ "golang.org/x/image/webp"
)

// MaxBytes is the largest image file accepted.
const MaxBytes = 15 << 20

// ErrTooLarge is returned for a file over MaxBytes.
var ErrTooLarge = fmt.Errorf("image is larger than %d MiB", MaxBytes>>20)

// ErrUnsupported is returned for data that is not a JPEG, PNG, GIF or WebP image.
var ErrUnsupported = errors.New("not a JPEG, PNG, GIF or WebP image")

// formatMimeTypes maps image.DecodeConfig format names to the MIME type the file is served as.
// SVG and other formats that can carry script are deliberately not accepted.
var formatMimeTypes = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

var formatExtensions = map[string]string{
	"jpeg": ".jpg",
	"png":  ".png",
	"gif":  ".gif",
	"webp": ".webp",
}

// storageNamePattern matches the names Save generates, so Open never leaves the directory.
var storageNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|gif|webp)$`)

// Metadata describes a stored image file.
type Metadata struct {
	StorageName string
	MimeType    string
	SHA256      string
	SizeBytes   int64
	Width       int
	Height      int
}

// Store keeps image files in Dir.
type Store struct {
	Dir string
}

// NewStore returns a Store for dir, creating the directory when it is missing.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create images directory %s: %w", dir, err)
	}
	return &Store{Dir: dir}, nil
}

// Inspect checks data is a supported image and returns its metadata, without a storage name.
func Inspect(data []byte) (Metadata, error) {
	meta, _, err := inspect(data)
	return meta, err
}

// inspect is Inspect, also returning the image.DecodeConfig format name.
func inspect(data []byte) (Metadata, string, error) {
	if len(data) > MaxBytes {
		return Metadata{}, "", ErrTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Metadata{}, "", ErrUnsupported
	}
	mimeType, ok := formatMimeTypes[format]
	if !ok {
		return Metadata{}, "", ErrUnsupported
	}
	sum := sha256.Sum256(data)
	return Metadata{
		MimeType:  mimeType,
		SizeBytes: int64(len(data)),
		Width:     cfg.Width,
		Height:    cfg.Height,
		SHA256:    hex.EncodeToString(sum[:]),
	}, format, nil
}

// Save validates data and writes it to a new, randomly named file in the store.
func (s *Store) Save(data []byte) (Metadata, error) {
	meta, format, err := inspect(data)
	if err != nil {
		return Metadata{}, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return Metadata{}, err
	}
	meta.StorageName = hex.EncodeToString(random) + formatExtensions[format]

	path := filepath.Join(s.Dir, meta.StorageName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return Metadata{}, fmt.Errorf("write image: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Metadata{}, fmt.Errorf("write image: %w", err)
	}
	return meta, nil
}

// Open opens a stored image file by its storage name.
func (s *Store) Open(storageName string) (*os.File, error) {
	if !storageNamePattern.MatchString(storageName) {
		return nil, os.ErrNotExist
	}
	return os.Open(filepath.Join(s.Dir, storageName)) //nolint:gosec // G304: name is validated above
}

// Remove deletes a stored image file; a missing file is not an error.
func (s *Store) Remove(storageName string) error {
	if !storageNamePattern.MatchString(storageName) {
		return nil
	}
	err := os.Remove(filepath.Join(s.Dir, storageName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
