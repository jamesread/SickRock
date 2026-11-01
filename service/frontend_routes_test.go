package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jamesread/SickRock/internal/pwa"
)

// newFrontendTestRouter serves a frontend directory laid out like the build: public/ plus
// index.html and some hashed assets.
func newFrontendTestRouter(t *testing.T, title string) *gin.Engine {
	t.Helper()
	dir := t.TempDir()
	public := filepath.Join("..", "frontend", "public")
	err := filepath.Walk(public, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(public, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		return copyFileForTest(p, filepath.Join(dir, rel))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFileForTest(filepath.Join("..", "frontend", "index.html"), filepath.Join(dir, "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets", "hugeicons", "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"assets/index-AbCd1234.js":             "console.log('app')",
		"assets/index-Zz_9-yX0.css":            "body{}",
		"assets/hugeicons/1.0.0/catalog.json":  "{}",
		"assets/hugeicons/1.0.0/HomeIcon.json": "[]",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerFrontendRoutes(router, dir, func(context.Context) (string, error) { return title, nil })
	return router
}

func copyFileForTest(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func get(router http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestManifestRoute(t *testing.T) {
	router := newFrontendTestRouter(t, "Acme Stock")

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := get(router, method, pwa.Path)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != pwa.ContentType {
			t.Fatalf("%s %s: status %d, type %q", method, pwa.Path, rec.Code, rec.Header().Get("Content-Type"))
		}
		if method == http.MethodHead {
			continue
		}
		if problems := pwa.Validate(rec.Body.Bytes()); len(problems) > 0 {
			t.Errorf("served manifest is invalid: %v", problems)
		}
		if m, _ := pwa.Parse(rec.Body.Bytes()); m.Name != "Acme Stock" {
			t.Errorf("name = %q, want the instance title", m.Name)
		}
	}
}

func TestEveryManifestImageIsServedAsAnImage(t *testing.T) {
	router := newFrontendTestRouter(t, "")
	m, err := pwa.Parse(get(router, http.MethodGet, pwa.Path).Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	images := append(append([]pwa.Image{}, m.Icons...), m.Screenshots...)
	if len(images) == 0 {
		t.Fatal("manifest has no images")
	}
	for _, image := range images {
		rec := get(router, http.MethodGet, image.Src)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != image.Type {
			t.Errorf("%s: status %d, type %q; want 200 and %s", image.Src, rec.Code, rec.Header().Get("Content-Type"), image.Type)
		}
	}
}

func TestServiceWorkerRoute(t *testing.T) {
	rec := get(newFrontendTestRouter(t, ""), http.MethodGet, "/sw.js")
	checks := map[string]string{
		"Content-Type":           "application/javascript",
		"Service-Worker-Allowed": "/",
		"Cache-Control":          "no-cache",
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "addEventListener('fetch'") {
		t.Fatalf("/sw.js: status %d, not the service worker", rec.Code)
	}
	for header, want := range checks {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("/sw.js %s = %q, want %q", header, got, want)
		}
	}
}

func TestPagesAreTheRevalidatedAppShell(t *testing.T) {
	router := newFrontendTestRouter(t, "")
	for _, target := range []string{"/", "/login", "/table/jobs", "/table/jobs/12", "/admin/health", "/exports/free_weekends"} {
		rec := get(router, http.MethodGet, target)
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("%s: status %d, type %q; want the HTML app shell", target, rec.Code, rec.Header().Get("Content-Type"))
			continue
		}
		if !pwa.HasManifestLink(rec.Body.String()) {
			t.Errorf("%s: page has no manifest link", target)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", target, got)
		}
	}

	offline := get(router, http.MethodGet, "/offline.html")
	if offline.Code != http.StatusOK || !strings.HasPrefix(offline.Header().Get("Content-Type"), "text/html") {
		t.Errorf("/offline.html: status %d, type %q", offline.Code, offline.Header().Get("Content-Type"))
	}
}

func TestMissingFilesAre404NotTheAppShell(t *testing.T) {
	router := newFrontendTestRouter(t, "")
	for _, target := range []string{
		"/manifest.template.json", // no longer fetched by the app
		"/missing.js", "/missing.css", "/missing.png", "/icons/missing.png", "/missing.webmanifest",
		"/favicon.ico", "/assets/index-Missing1.js", "/assets/hugeicons/1.0.0/Missing.json",
		"/api/nothing", "/table/jobs/source.map", "/UPPER.JS",
	} {
		rec := get(router, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<html") || strings.Contains(rec.Body.String(), "<!doctype") {
			t.Errorf("%s: answered with the HTML app shell", target)
		}
	}
}

func TestHashedAssetsAreCachedForever(t *testing.T) {
	router := newFrontendTestRouter(t, "")
	for target, immutable := range map[string]bool{
		"/assets/index-AbCd1234.js":             true,
		"/assets/index-Zz_9-yX0.css":            true,
		"/assets/hugeicons/1.0.0/HomeIcon.json": false,
		"/assets/hugeicons/1.0.0/catalog.json":  false,
	} {
		rec := get(router, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", target, rec.Code)
			continue
		}
		got := strings.Contains(rec.Header().Get("Cache-Control"), "immutable")
		if got != immutable {
			t.Errorf("%s: immutable = %v (Cache-Control %q), want %v", target, got, rec.Header().Get("Cache-Control"), immutable)
		}
	}
	if rec := get(router, http.MethodGet, "/assets/index-AbCd1234.js"); !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Errorf("hashed script type = %q", rec.Header().Get("Content-Type"))
	}
}
