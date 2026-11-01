package pwa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The real frontend files: the tests guard what ships, not just the code.
var (
	publicDir    = filepath.Join("..", "..", "..", "frontend", "public")
	frontendRoot = filepath.Join("..", "..", "..", "frontend")
)

func readTemplate(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(publicDir, TemplateFile))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	return data
}

func buildManifest(t *testing.T, title string) map[string]any {
	t.Helper()
	body, err := Build(readTemplate(t), title)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("built manifest is not JSON: %v\n%s", err, body)
	}
	return m
}

func TestCleanTitle(t *testing.T) {
	long := strings.Repeat("é", MaxTitleLength+20)
	cases := map[string]string{
		"":                      DefaultTitle,
		"   ":                   DefaultTitle,
		"\t\n\r":                DefaultTitle,
		"Acme":                  "Acme",
		"  Acme   Stock  ":      "Acme Stock",
		"Acme\nStock\x00Room":   "Acme Stock Room",
		"R&D <Lab> \"Q\"":       "R&D <Lab> \"Q\"",
		"Ünïcødé 📦":             "Ünïcødé 📦",
		long:                    strings.Repeat("é", MaxTitleLength),
		"\u200bZero\u00a0width": "\u200bZero width",
	}
	for in, want := range cases {
		if got := CleanTitle(in); got != want {
			t.Errorf("CleanTitle(%q) = %q, want %q", in, got, want)
		}
		if n := len([]rune(CleanTitle(in))); n > MaxTitleLength {
			t.Errorf("CleanTitle(%q) is %d characters, over %d", in, n, MaxTitleLength)
		}
	}
}

func TestBuildSetsNamesAndKeepsTemplateFields(t *testing.T) {
	var template map[string]any
	if err := json.Unmarshal(readTemplate(t), &template); err != nil {
		t.Fatalf("template is not JSON: %v", err)
	}

	m := buildManifest(t, "  Acme   Stock ")
	if m["name"] != "Acme Stock" || m["short_name"] != "Acme Stock" {
		t.Fatalf("name/short_name = %q/%q, want Acme Stock", m["name"], m["short_name"])
	}
	for key, want := range template {
		if key == "name" || key == "short_name" {
			continue
		}
		got, _ := json.Marshal(m[key])
		expected, _ := json.Marshal(want)
		if !bytes.Equal(got, expected) {
			t.Errorf("Build changed %q: got %s, want %s", key, got, expected)
		}
	}
	// The id must not follow the title, or renaming the instance would make installed apps
	// look like a different app.
	if m["id"] != template["id"] {
		t.Errorf("id = %v, want the template's %v", m["id"], template["id"])
	}
}

func TestBuildDoesNotEscapeHTMLCharacters(t *testing.T) {
	body, err := Build(readTemplate(t), "R&D <Lab>")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"R&D <Lab>"`)) {
		t.Errorf("title was escaped:\n%s", body)
	}
}

func TestBuildRejectsBadTemplates(t *testing.T) {
	for _, template := range []string{"", "null", "[]", `"x"`, "{", "42"} {
		if _, err := Build([]byte(template), "x"); err == nil {
			t.Errorf("Build(%q) succeeded, want an error", template)
		}
	}
}

func TestShippedManifestIsValidForAnyTitle(t *testing.T) {
	for _, title := range []string{"", "SickRock", "Acme Stock", "R&D \"Lab\"", strings.Repeat("x", 500), "\x00\x01"} {
		body, err := Build(readTemplate(t), title)
		if err != nil {
			t.Fatalf("Build(%q): %v", title, err)
		}
		if problems := Validate(body); len(problems) > 0 {
			t.Errorf("manifest for title %q is invalid:\n%s", title, strings.Join(problems, "\n"))
		}
	}
}

func TestShippedManifestFilesExistAndMatchTheirSizes(t *testing.T) {
	body, err := Build(readTemplate(t), DefaultTitle)
	if err != nil {
		t.Fatal(err)
	}
	if problems := CheckFiles(body, publicDir); len(problems) > 0 {
		t.Errorf("manifest files:\n%s", strings.Join(problems, "\n"))
	}
}

func TestShippedFrontendHasNoPWAProblems(t *testing.T) {
	// The built frontend is public/ plus index.html; rebuild that layout from the sources.
	dir := t.TempDir()
	copyDir(t, publicDir, dir)
	copyFile(t, filepath.Join(frontendRoot, "index.html"), filepath.Join(dir, "index.html"))

	if problems := FrontendProblems(dir); len(problems) > 0 {
		t.Errorf("frontend PWA problems:\n%s", strings.Join(problems, "\n"))
	}
}

func TestFrontendProblemsReportsEachMissingPart(t *testing.T) {
	problems := strings.Join(FrontendProblems(t.TempDir()), "\n")
	for _, want := range []string{TemplateFile, "index.html", "sw.js", "offline.html"} {
		if !strings.Contains(problems, want) {
			t.Errorf("empty frontend: no problem mentions %s:\n%s", want, problems)
		}
	}

	dir := t.TempDir()
	copyDir(t, publicDir, dir)
	writeFile(t, filepath.Join(dir, "index.html"), `<html><head><link rel="icon" href="/x.png"></head></html>`)
	if problems := FrontendProblems(dir); !containsText(problems, `rel="manifest"`) {
		t.Errorf("index.html without a manifest link: got %v", problems)
	}
}

// mutate builds the shipped manifest and changes one thing about it.
func mutate(t *testing.T, change func(m map[string]any)) []byte {
	t.Helper()
	m := buildManifest(t, "SickRock")
	change(m)
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func icons(m map[string]any) []any { return m["icons"].([]any) }

func TestValidateCatchesBrokenManifests(t *testing.T) {
	cases := []struct {
		name   string
		change func(m map[string]any)
		want   string
	}{
		{"no name", func(m map[string]any) { m["name"] = " " }, "name is empty"},
		{"no short name", func(m map[string]any) { delete(m, "short_name") }, "short_name is empty"},
		{"absolute start_url", func(m map[string]any) { m["start_url"] = "https://evil.example/" }, "start_url"},
		{"protocol-relative start_url", func(m map[string]any) { m["start_url"] = "//evil.example/" }, "start_url"},
		{"relative scope", func(m map[string]any) { m["scope"] = "app/" }, "scope"},
		{"missing id", func(m map[string]any) { delete(m, "id") }, "id"},
		{"start_url outside scope", func(m map[string]any) { m["scope"] = "/app/"; m["start_url"] = "/" }, "outside scope"},
		{"bad display", func(m map[string]any) { m["display"] = "window" }, "display"},
		{"bad theme colour", func(m map[string]any) { m["theme_color"] = "grey" }, "theme_color"},
		{"bad background colour", func(m map[string]any) { m["background_color"] = "#12345" }, "background_color"},
		{"no icons", func(m map[string]any) { m["icons"] = []any{} }, "no icons"},
		{"no 512 icon", func(m map[string]any) {
			var kept []any
			for _, icon := range icons(m) {
				if icon.(map[string]any)["sizes"] != "512x512" {
					kept = append(kept, icon)
				}
			}
			m["icons"] = kept
		}, "512x512"},
		{"192 icon only maskable", func(m map[string]any) {
			for _, icon := range icons(m) {
				if icon.(map[string]any)["sizes"] == "192x192" {
					icon.(map[string]any)["purpose"] = "maskable"
				}
			}
		}, "192x192"},
		{"icon on another site", func(m map[string]any) { icons(m)[0].(map[string]any)["src"] = "https://cdn.example/i.png" }, "icons[0] src"},
		{"icon without type", func(m map[string]any) { delete(icons(m)[0].(map[string]any), "type") }, "icons[0] type"},
		{"icon with bad size", func(m map[string]any) { icons(m)[0].(map[string]any)["sizes"] = "72" }, "icons[0] size"},
		{"icon with bad purpose", func(m map[string]any) { icons(m)[0].(map[string]any)["purpose"] = "round" }, "purpose"},
		{"screenshot form factor", func(m map[string]any) {
			m["screenshots"].([]any)[0].(map[string]any)["form_factor"] = "tablet"
		}, "form_factor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := Validate(mutate(t, tc.change))
			if !containsText(problems, tc.want) {
				t.Errorf("want a problem mentioning %q, got %v", tc.want, problems)
			}
		})
	}
}

func TestValidateRejectsNonJSON(t *testing.T) {
	if problems := Validate([]byte("<!doctype html>")); len(problems) != 1 || !strings.Contains(problems[0], "JSON") {
		t.Errorf("HTML instead of a manifest: got %v", problems)
	}
}

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPNGSize(t *testing.T) {
	width, height, err := pngSize(encodePNG(t, 37, 91))
	if err != nil || width != 37 || height != 91 {
		t.Errorf("pngSize = %d, %d, %v; want 37, 91", width, height, err)
	}
	for _, data := range [][]byte{nil, []byte("not a png at all, really not"), []byte("<!doctype html><html><head></head></html>")} {
		if _, _, err := pngSize(data); err == nil {
			t.Errorf("pngSize(%q) succeeded", data)
		}
	}
}

func TestCheckFilesCatchesMissingWrongAndFakeImages(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "icons", "right.png"), string(encodePNG(t, 192, 192)))
	writeFile(t, filepath.Join(dir, "icons", "wrong.png"), string(encodePNG(t, 100, 100)))
	writeFile(t, filepath.Join(dir, "icons", "html.png"), "<!doctype html><html></html>")

	manifest := []byte(`{"icons":[
		{"src":"/icons/right.png","sizes":"192x192","type":"image/png"},
		{"src":"/icons/wrong.png","sizes":"512x512","type":"image/png"},
		{"src":"/icons/html.png","sizes":"192x192","type":"image/png"},
		{"src":"/icons/missing.png","sizes":"192x192","type":"image/png"},
		{"src":"/../../etc/passwd","sizes":"192x192","type":"image/png"}
	]}`)
	problems := CheckFiles(manifest, dir)
	for _, want := range []string{"wrong.png is 100x100", "html.png is not a PNG", "missing.png is missing", "passwd is missing"} {
		if !containsText(problems, want) {
			t.Errorf("want a problem mentioning %q, got %v", want, problems)
		}
	}
	if containsText(problems, "right.png") {
		t.Errorf("right.png is fine but was reported: %v", problems)
	}
}

func TestHasManifestLink(t *testing.T) {
	cases := map[string]bool{
		`<link rel="manifest" href="/manifest.json">`:                true,
		`<head><link href="/manifest.json" rel="manifest" /></head>`: true,
		`<link rel="manifest" href="/manifest.template.json">`:       false,
		`<link rel="icon" href="/manifest.json">`:                    false,
		`<!-- <link rel="manifest" href="/manifest.json" -->`:        false,
		``: false,
	}
	for html, want := range cases {
		if got := HasManifestLink(html); got != want {
			t.Errorf("HasManifestLink(%q) = %v, want %v", html, got, want)
		}
	}
}

func TestShippedIndexHTMLLinksTheManifest(t *testing.T) {
	index, err := os.ReadFile(filepath.Join(frontendRoot, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !HasManifestLink(string(index)) {
		t.Errorf(`frontend/index.html has no <link rel="manifest" href="%s">`, Path)
	}
}

func serve(t *testing.T, h Handler, method string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, Path, nil))
	return rec
}

func TestHandlerServesAValidManifestForTheTitle(t *testing.T) {
	h := Handler{FrontendDir: publicDir, Title: func(context.Context) (string, error) { return "Acme Stock", nil }}
	rec := serve(t, h, http.MethodGet)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != ContentType {
		t.Errorf("Content-Type = %q, want %q", got, ContentType)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if problems := Validate(rec.Body.Bytes()); len(problems) > 0 {
		t.Errorf("served manifest is invalid: %v", problems)
	}
	m, _ := Parse(rec.Body.Bytes())
	if m.Name != "Acme Stock" || m.ShortName != "Acme Stock" {
		t.Errorf("name = %q/%q, want Acme Stock", m.Name, m.ShortName)
	}
}

func TestHandlerFallsBackToTheDefaultTitle(t *testing.T) {
	for name, title := range map[string]TitleFunc{
		"nil func":   nil,
		"error":      func(context.Context) (string, error) { return "", errors.New("database down") },
		"empty":      func(context.Context) (string, error) { return "  ", nil },
		"slow title": func(ctx context.Context) (string, error) { <-ctx.Done(); return "late", ctx.Err() },
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			rec := serve(t, Handler{FrontendDir: publicDir, Title: title}, http.MethodGet)
			if elapsed := time.Since(start); elapsed > titleTimeout+time.Second {
				t.Errorf("took %s; the title lookup must time out", elapsed)
			}
			m, err := Parse(rec.Body.Bytes())
			if rec.Code != http.StatusOK || err != nil || m.Name != DefaultTitle {
				t.Errorf("status %d, name %q, err %v; want 200 and %q", rec.Code, m.Name, err, DefaultTitle)
			}
		})
	}
}

func TestHandlerMethodsAndFailures(t *testing.T) {
	h := Handler{FrontendDir: publicDir}

	head := serve(t, h, http.MethodHead)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != ContentType {
		t.Errorf("HEAD: status %d, %d body bytes, type %q", head.Code, head.Body.Len(), head.Header().Get("Content-Type"))
	}

	post := serve(t, h, http.MethodPost)
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST: status %d, Allow %q", post.Code, post.Header().Get("Allow"))
	}

	missing := serve(t, Handler{FrontendDir: t.TempDir()}, http.MethodGet)
	if missing.Code != http.StatusInternalServerError || missing.Header().Get("Content-Type") == ContentType {
		t.Errorf("missing template: status %d, type %q; want 500 and no manifest", missing.Code, missing.Header().Get("Content-Type"))
	}

	broken := t.TempDir()
	writeFile(t, filepath.Join(broken, TemplateFile), "{not json")
	if rec := serve(t, Handler{FrontendDir: broken}, http.MethodGet); rec.Code != http.StatusInternalServerError {
		t.Errorf("broken template: status %d, want 500", rec.Code)
	}
}

func containsText(problems []string, text string) bool {
	for _, p := range problems {
		if strings.Contains(p, text) {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		copyFile(t, p, target)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandlerReadsTheTemplateFromFrontendSources(t *testing.T) {
	// A development server pointed at frontend/ (not dist/) finds the template in public/.
	rec := serve(t, Handler{FrontendDir: frontendRoot}, http.MethodGet)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if problems := Validate(rec.Body.Bytes()); len(problems) > 0 {
		t.Errorf("manifest from sources is invalid: %v", problems)
	}
}
