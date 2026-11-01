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
	FrontendDir string
	Title       TitleFunc
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	template, err := ReadTemplate(h.FrontendDir)
	if err != nil {
		log.WithError(err).Error("Cannot read the web app manifest template")
		http.Error(w, "manifest unavailable", http.StatusInternalServerError)
		return
	}

	body, err := Build(template, h.title(r.Context()))
	if err != nil {
		log.WithError(err).Error("Cannot build the web app manifest")
		http.Error(w, "manifest unavailable", http.StatusInternalServerError)
		return
	}

	header := w.Header()
	header.Set("Content-Type", ContentType)
	// Revalidate every time: a changed title or icon set should reach installed apps.
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
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
	template, err := os.ReadFile(filepath.Join(frontendDir, TemplateFile))
	if err == nil {
		return template, nil
	}
	if fromSources, srcErr := os.ReadFile(filepath.Join(frontendDir, "public", TemplateFile)); srcErr == nil {
		return fromSources, nil
	}
	return nil, err
}

// FrontendProblems returns what stops the built frontend in frontendDir working as an
// installable PWA: the manifest template, the files it references, the manifest link in
// index.html, the service worker and the offline page.
func FrontendProblems(frontendDir string) []string {
	var problems []string

	template, err := os.ReadFile(filepath.Join(frontendDir, TemplateFile))
	if err != nil {
		problems = append(problems, TemplateFile+" is missing from the frontend")
	} else if manifest, err := Build(template, DefaultTitle); err != nil {
		problems = append(problems, err.Error())
	} else {
		problems = append(problems, Validate(manifest)...)
		problems = append(problems, CheckFiles(manifest, frontendDir)...)
	}

	index, err := os.ReadFile(filepath.Join(frontendDir, "index.html"))
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

// HasManifestLink reports whether an HTML document links to the served manifest.
func HasManifestLink(html string) bool {
	html = htmlComment.ReplaceAllString(html, "")
	for _, tag := range strings.Split(html, "<link")[1:] {
		end := strings.Index(tag, ">")
		if end < 0 {
			continue
		}
		attrs := tag[:end]
		if strings.Contains(attrs, `rel="manifest"`) && strings.Contains(attrs, `href="`+Path+`"`) {
			return true
		}
	}
	return false
}
