package server

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/images"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

// newImageTestServer returns a server on a migrated SQLite database with a recipes table whose
// main_picture column is an image column.
func newImageTestServer(t *testing.T) *SickRockServer {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		CREATE TABLE recipes (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, main_picture BIGINT);
		INSERT INTO table_configurations (name, title, db, "table") VALUES ('recipes', 'Recipes', 'main', 'recipes');
		INSERT INTO table_column_semantics (table_configuration, column_name, semantic_type) VALUES ('recipes', 'main_picture', 'image');`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	store, err := images.NewStore(t.TempDir())
	require.NoError(t, err)
	srv.SetImageStore(store)
	return srv
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 3))))
	return buf.Bytes()
}

func uploadImage(ctx context.Context, srv *SickRockServer, column string, data []byte) (*sickrockpb.ImageInfo, error) {
	res, err := srv.UploadImage(ctx, connect.NewRequest(&sickrockpb.UploadImageRequest{
		PageId: "recipes", ColumnName: column, Data: data, Filename: `C:\photos\cake.png`,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetImage(), nil
}

func getImage(ctx context.Context, srv *SickRockServer, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	srv.ImageHandler().ServeHTTP(rec, req)
	return rec
}

func TestUploadAndServeImage(t *testing.T) {
	srv := newImageTestServer(t)
	admin := authn.SetInfo(context.Background(), authUserWith(true))
	data := testPNG(t)

	info, err := uploadImage(admin, srv, "main_picture", data)
	require.NoError(t, err)
	assert.Equal(t, "image/png", info.GetMimeType())
	assert.Equal(t, int32(4), info.GetWidth())
	assert.Equal(t, int32(3), info.GetHeight())
	assert.Equal(t, "cake.png", info.GetOriginalFilename())
	assert.Equal(t, "/api/images/"+strconv.FormatInt(info.GetId(), 10), info.GetUrl())

	path := ImagesPathPrefix + strconv.FormatInt(info.GetId(), 10)
	rec := getImage(admin, srv, path)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, data, rec.Body.Bytes())

	assert.Equal(t, http.StatusNotFound, getImage(admin, srv, ImagesPathPrefix+"999").Code)
	assert.Equal(t, http.StatusNotFound, getImage(admin, srv, ImagesPathPrefix+"../x").Code)
	// Users without view access to recipes, or not signed in, are told the image does not exist.
	noGrants := authn.SetInfo(context.Background(), authUserWith(false))
	assert.Equal(t, http.StatusNotFound, getImage(noGrants, srv, path).Code)
	assert.Equal(t, http.StatusNotFound, getImage(context.Background(), srv, path).Code)
}

func TestUploadImageRejects(t *testing.T) {
	srv := newImageTestServer(t)
	admin := authn.SetInfo(context.Background(), authUserWith(true))

	_, err := uploadImage(admin, srv, "name", testPNG(t))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "not an image column")

	_, err = uploadImage(admin, srv, "main_picture", []byte("<svg/>"))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "not an image")

	_, err = uploadImage(context.Background(), srv, "main_picture", testPNG(t))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = uploadImage(authn.SetInfo(context.Background(), authUserWith(false)), srv, "main_picture", testPNG(t))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUploadFilename(t *testing.T) {
	assert.Equal(t, "a.jpg", uploadFilename("../../a.jpg"))
	assert.Equal(t, "b.png", uploadFilename(`C:\x\b.png`))
	assert.Empty(t, uploadFilename(""))
}
