package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/images"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// ImagesPathPrefix is where images are served, below the /api prefix: GET /api/images/{id}.
const ImagesPathPrefix = "/images/"

// SetImageStore sets where uploaded image files are kept. Without one, uploads are refused.
func (s *SickRockServer) SetImageStore(store *images.Store) {
	s.images = store
}

// imageURL is the path an image is served from by ImageHandler.
func imageURL(id int64) string {
	return "/api" + ImagesPathPrefix + strconv.FormatInt(id, 10)
}

func imageToProto(img repo.Image) *sickrockpb.ImageInfo {
	return &sickrockpb.ImageInfo{
		Id:               img.ID,
		MimeType:         img.MimeType,
		SizeBytes:        img.SizeBytes,
		Width:            safeInt64ToInt32(int64(img.Width)),
		Height:           safeInt64ToInt32(int64(img.Height)),
		OriginalFilename: img.OriginalFilename,
		Sha256:           img.SHA256,
		Url:              imageURL(img.ID),
	}
}

// UploadImage stores an image for a table's image column. The caller needs insert or edit access to
// the table; the column must have the image semantic.
func (s *SickRockServer) UploadImage(ctx context.Context, req *connect.Request[sickrockpb.UploadImageRequest]) (*connect.Response[sickrockpb.UploadImageResponse], error) {
	table := strings.TrimSpace(req.Msg.GetPageId())
	column := strings.TrimSpace(req.Msg.GetColumnName())
	if err := s.checkImageUpload(ctx, table, column); err != nil {
		return nil, err
	}

	meta, err := s.images.Save(req.Msg.GetData())
	if errors.Is(err, images.ErrTooLarge) || errors.Is(err, images.ErrUnsupported) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	img, err := s.recordImage(ctx, meta, table, column, req.Msg.GetFilename())
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{"table": table, "column": column, "image": img.ID, "bytes": img.SizeBytes}).Info("Image uploaded")
	return connect.NewResponse(&sickrockpb.UploadImageResponse{Image: imageToProto(img)}), nil
}

// checkImageUpload rejects an upload when storage is off, the target is missing, the caller may
// not write to table, or column is not an image column.
func (s *SickRockServer) checkImageUpload(ctx context.Context, table, column string) error {
	if s.images == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("image storage is not configured"))
	}
	if table == "" || column == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("page_id and column_name are required"))
	}
	if err := s.requireImageUploadAccess(ctx, table); err != nil {
		return err
	}
	return s.requireImageColumn(ctx, table, column)
}

// recordImage adds a saved file to the images table, removing the file if that fails.
func (s *SickRockServer) recordImage(ctx context.Context, meta images.Metadata, table, column, filename string) (repo.Image, error) {
	userID, _ := s.getUserIDFromContext(ctx)
	img, err := s.repo.InsertImage(ctx, repo.Image{
		StorageName:        meta.StorageName,
		OriginalFilename:   uploadFilename(filename),
		MimeType:           meta.MimeType,
		SizeBytes:          meta.SizeBytes,
		Width:              meta.Width,
		Height:             meta.Height,
		SHA256:             meta.SHA256,
		TableConfiguration: table,
		ColumnName:         column,
		UploadedBy:         sql.NullInt64{Int64: int64(userID), Valid: userID > 0},
	})
	if err == nil {
		return img, nil
	}
	if rmErr := s.images.Remove(meta.StorageName); rmErr != nil {
		log.WithError(rmErr).Warn("Failed to remove image file after insert failure")
	}
	return repo.Image{}, connect.NewError(connect.CodeInternal, fmt.Errorf("record image: %w", err))
}

// requireImageUploadAccess allows callers that may insert or edit rows of table.
func (s *SickRockServer) requireImageUploadAccess(ctx context.Context, table string) error {
	err := s.requireItemWriteAccess(ctx, table, repo.ResourceActionInsert)
	if err == nil {
		return nil
	}
	if s.requireItemWriteAccess(ctx, table, repo.ResourceActionEdit) == nil {
		return nil
	}
	return err
}

func (s *SickRockServer) requireImageColumn(ctx context.Context, table, column string) error {
	semantics, err := s.repo.GetColumnSemantics(ctx, table)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if semantics[column] != repo.SemanticTypeImage {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("column %q of %q is not an image column", column, table))
	}
	return nil
}

// uploadFilename keeps the base name of a client-supplied filename, for display only.
func uploadFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/"))
	if name == "." || name == "/" {
		return ""
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}

// ImageHandler serves GET /images/{id} (mounted below /api, behind the auth layer). The caller needs
// view access to the table the image was uploaded for; images they cannot see are reported as missing.
func (s *SickRockServer) ImageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		img, ok := s.viewableImage(r)
		if !ok {
			http.NotFound(w, r)
			return
		}
		s.serveImageFile(w, r, img)
	})
}

// viewableImage returns the image named by the request path when it exists and the caller may see it.
func (s *SickRockServer) viewableImage(r *http.Request) (repo.Image, bool) {
	id, ok := imageIDFromPath(r.URL.Path)
	if !ok || s.images == nil {
		return repo.Image{}, false
	}
	img, err := s.repo.GetImage(r.Context(), id)
	if err != nil {
		if !errors.Is(err, repo.ErrImageNotFound) {
			log.WithError(err).WithField("image", id).Error("Failed to load image")
		}
		return repo.Image{}, false
	}
	if img.TableConfiguration != "" && s.requireDataTableAccess(r.Context(), img.TableConfiguration, repo.ResourceActionView) != nil {
		return repo.Image{}, false
	}
	return img, true
}

// imageIDFromPath parses the id from /images/{id}.
func imageIDFromPath(path string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(path, ImagesPathPrefix), 10, 64)
	return id, err == nil && id > 0
}

func (s *SickRockServer) serveImageFile(w http.ResponseWriter, r *http.Request, img repo.Image) {
	f, err := s.images.Open(img.StorageName)
	if err != nil {
		log.WithError(err).WithField("image", img.ID).Warn("Image file is missing")
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "image unavailable", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", img.MimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// An image id always refers to the same bytes, so browsers may keep it.
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("ETag", `"`+img.SHA256+`"`)
	http.ServeContent(w, r, "", stat.ModTime(), f)
}
