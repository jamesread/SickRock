package pwa

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// titleTimeout bounds the title lookup, so a slow database cannot stall the manifest.
const titleTimeout = 2 * time.Second

// TitleFunc returns the instance's app title ("" for the default).
type TitleFunc func(ctx context.Context) (string, error)

// Handler serves the manifest, built from TemplateFile in FrontendDir with the title from Title.
type Handler struct {
	Title       TitleFunc
	FrontendDir string
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, ok := h.manifestBody(w, r)
	if !ok {
		return
	}

	header := w.Header()
	header.Set("Content-Type", ContentType)
	// Revalidate every time: a changed title or icon set should reach installed apps.
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		// Body is application/manifest+json produced by Build (encoding/json); not HTML.
		_, _ = w.Write(body) //nolint:gosec // G705: JSON we built; Content-Type is application/manifest+json with nosniff
	}
}

// manifestBody builds the manifest, replying with a 500 and returning false when it cannot.
func (h Handler) manifestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	template, err := ReadTemplate(h.FrontendDir)
	if err != nil {
		log.WithError(err).Error("Cannot read the web app manifest template")
		http.Error(w, "manifest unavailable", http.StatusInternalServerError)
		return nil, false
	}

	body, err := Build(template, h.title(r.Context()))
	if err != nil {
		log.WithError(err).Error("Cannot build the web app manifest")
		http.Error(w, "manifest unavailable", http.StatusInternalServerError)
		return nil, false
	}
	return body, true
}

func (h Handler) title(ctx context.Context) string {
	if h.Title == nil {
		return DefaultTitle
	}
	ctx, cancel := context.WithTimeout(ctx, titleTimeout)
	defer cancel()
	title, err := h.Title(ctx)
	if err != nil {
		log.WithError(err).Warn("Cannot read the app title for the web app manifest; using the default")
		return DefaultTitle
	}
	return title
}

// ReadTemplate reads the manifest template from a built frontend (frontendDir) or, for a
// development server pointed at the frontend sources, from frontendDir/public.
func ReadTemplate(frontendDir string) ([]byte, error) {
	template, err := readFrontendFile(frontendDir, TemplateFile)
	if err == nil {
		return template, nil
	}
	if fromSources, srcErr := readFrontendFile(frontendDir, filepath.Join("public", TemplateFile)); srcErr == nil {
		return fromSources, nil
	}
	return nil, err
}

// FrontendProblems returns what stops the built frontend in frontendDir working as an
// installable PWA: the manifest template, the files it references, the manifest link in
// index.html, the service worker and the offline page.
func FrontendProblems(frontendDir string) []string {
	problems := templateProblems(frontendDir)

	index, err := readFrontendFile(frontendDir, "index.html")
	switch {
	case err != nil:
		problems = append(problems, "index.html is missing from the frontend")
	case !HasManifestLink(string(index)):
		problems = append(problems, `index.html has no <link rel="manifest" href="`+Path+`">`)
	}

	for _, file := range []string{"sw.js", "offline.html"} {
		if _, err := os.Stat(filepath.Join(frontendDir, file)); err != nil {
			problems = append(problems, file+" is missing from the frontend")
		}
	}
	return problems
}

// templateProblems builds the manifest from the template in frontendDir and returns
// what is wrong with it or the files it references.
func templateProblems(frontendDir string) []string {
	template, err := readFrontendFile(frontendDir, TemplateFile)
	if err != nil {
		return []string{TemplateFile + " is missing from the frontend"}
	}
	manifest, err := Build(template, DefaultTitle)
	if err != nil {
		return []string{err.Error()}
	}
	return append(Validate(manifest), CheckFiles(manifest, frontendDir)...)
}

// readFrontendFile reads name (a path relative to frontendDir) without leaving frontendDir.
func readFrontendFile(frontendDir, name string) ([]byte, error) {
	root, err := os.OpenRoot(frontendDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(name)
}

// HasManifestLink reports whether an HTML document links to the served manifest.
func HasManifestLink(html string) bool {
	html = htmlComment.ReplaceAllString(html, "")
	for _, tag := range strings.Split(html, "<link")[1:] {
		attrs, _, closed := strings.Cut(tag, ">")
		if !closed {
			continue
		}
		if strings.Contains(attrs, `rel="manifest"`) && strings.Contains(attrs, `href="`+Path+`"`) {
			return true
		}
	}
	return false
}
