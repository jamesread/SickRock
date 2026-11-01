// Package pwa serves SickRock's web app manifest and checks that it, and the files it
// references, make SickRock installable as a Progressive Web App.
//
// The manifest is built on the server from the frontend's manifest.template.json, with the
// instance's app title (the "appTitle" setting) as its name. index.html links to it with
// <link rel="manifest" href="/manifest.json">.
package pwa

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	// DefaultTitle is the app name when no title is set.
	DefaultTitle = "SickRock"
	// TemplateFile is the manifest template, in the frontend directory.
	TemplateFile = "manifest.template.json"
	// Path is where the manifest is served.
	Path = "/manifest.json"
	// ContentType is the manifest's media type.
	ContentType = "application/manifest+json"
	// MaxTitleLength caps the name, in characters.
	MaxTitleLength = 100
)

// CleanTitle makes an app title safe for the manifest: trimmed, single-spaced, without
// control characters and at most MaxTitleLength characters. An empty title is DefaultTitle.
func CleanTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		if unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
	}
	cleaned := strings.Join(strings.Fields(b.String()), " ")
	if runes := []rune(cleaned); len(runes) > MaxTitleLength {
		cleaned = strings.TrimSpace(string(runes[:MaxTitleLength]))
	}
	if cleaned == "" {
		return DefaultTitle
	}
	return cleaned
}

// Build returns the manifest for an app title: the template with name and short_name set.
// Every other field of the template is kept as it is.
func Build(template []byte, title string) ([]byte, error) {
	var manifest map[string]any
	if err := json.Unmarshal(template, &manifest); err != nil {
		return nil, fmt.Errorf("manifest template is not a JSON object: %w", err)
	}
	if manifest == nil {
		return nil, errors.New("manifest template is not a JSON object")
	}
	name := CleanTitle(title)
	manifest["name"] = name
	manifest["short_name"] = name

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// Titles such as "R&D" must not be escaped to &.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Image is an icon or screenshot entry of a manifest.
type Image struct {
	Src        string `json:"src"`
	Sizes      string `json:"sizes"`
	Type       string `json:"type"`
	Purpose    string `json:"purpose"`
	FormFactor string `json:"form_factor"`
	Label      string `json:"label"`
}

// Manifest holds the manifest fields that are checked.
type Manifest struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ShortName       string  `json:"short_name"`
	StartURL        string  `json:"start_url"`
	Scope           string  `json:"scope"`
	Display         string  `json:"display"`
	ThemeColor      string  `json:"theme_color"`
	BackgroundColor string  `json:"background_color"`
	Icons           []Image `json:"icons"`
	Screenshots     []Image `json:"screenshots"`
}

var (
	colorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	sizePattern  = regexp.MustCompile(`^([1-9][0-9]*)x([1-9][0-9]*)$`)

	validDisplays     = map[string]bool{"fullscreen": true, "standalone": true, "minimal-ui": true, "browser": true}
	validImageTypes   = map[string]bool{"image/png": true, "image/webp": true, "image/svg+xml": true}
	validFormFactors  = map[string]bool{"wide": true, "narrow": true}
	validIconPurposes = map[string]bool{"any": true, "maskable": true, "monochrome": true}
	// Chrome needs a 192px and a 512px icon to offer installation.
	requiredIconSizes = []string{"192x192", "512x512"}
)

// Parse decodes a manifest.
func Parse(manifest []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifest, &m); err != nil {
		return m, fmt.Errorf("manifest is not valid JSON: %w", err)
	}
	return m, nil
}

// Validate returns what is wrong with a manifest, or nothing when browsers can install the app
// from it. It checks the fields install prompts depend on, not every optional member.
func Validate(manifest []byte) []string {
	m, err := Parse(manifest)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if strings.TrimSpace(m.Name) == "" {
		add("name is empty")
	}
	if strings.TrimSpace(m.ShortName) == "" {
		add("short_name is empty")
	}
	for field, value := range map[string]string{"id": m.ID, "start_url": m.StartURL, "scope": m.Scope} {
		if !isSitePath(value) {
			add("%s %q is not a path on this site (it must start with a single /)", field, value)
		}
	}
	if isSitePath(m.StartURL) && isSitePath(m.Scope) && !strings.HasPrefix(m.StartURL, m.Scope) {
		add("start_url %q is outside scope %q", m.StartURL, m.Scope)
	}
	if !validDisplays[m.Display] {
		add("display %q is not one of fullscreen, standalone, minimal-ui or browser", m.Display)
	}
	if !colorPattern.MatchString(m.ThemeColor) {
		add("theme_color %q is not a #rgb or #rrggbb colour", m.ThemeColor)
	}
	if !colorPattern.MatchString(m.BackgroundColor) {
		add("background_color %q is not a #rgb or #rrggbb colour", m.BackgroundColor)
	}

	if len(m.Icons) == 0 {
		add("there are no icons")
	}
	pngSizes := map[string]bool{}
	for i, icon := range m.Icons {
		problems = append(problems, validateImage(fmt.Sprintf("icons[%d]", i), icon)...)
		for _, purpose := range strings.Fields(icon.Purpose) {
			if !validIconPurposes[purpose] {
				add("icons[%d] purpose %q is not any, maskable or monochrome", i, purpose)
			}
		}
		if icon.Type == "image/png" && (icon.Purpose == "" || strings.Contains(icon.Purpose, "any")) {
			for _, size := range strings.Fields(icon.Sizes) {
				pngSizes[size] = true
			}
		}
	}
	for _, size := range requiredIconSizes {
		if !pngSizes[size] {
			add("there is no %s PNG icon (browsers need one to install the app)", size)
		}
	}

	for i, shot := range m.Screenshots {
		problems = append(problems, validateImage(fmt.Sprintf("screenshots[%d]", i), shot)...)
		if shot.FormFactor != "" && !validFormFactors[shot.FormFactor] {
			add("screenshots[%d] form_factor %q is not wide or narrow", i, shot.FormFactor)
		}
	}
	return problems
}

func validateImage(where string, image Image) []string {
	var problems []string
	if !isSitePath(image.Src) {
		problems = append(problems, fmt.Sprintf("%s src %q is not a path on this site", where, image.Src))
	}
	if !validImageTypes[image.Type] {
		problems = append(problems, fmt.Sprintf("%s type %q is not image/png, image/webp or image/svg+xml", where, image.Type))
	}
	sizes := strings.Fields(image.Sizes)
	if len(sizes) == 0 {
		problems = append(problems, fmt.Sprintf("%s has no sizes", where))
	}
	for _, size := range sizes {
		if size != "any" && !sizePattern.MatchString(size) {
			problems = append(problems, fmt.Sprintf("%s size %q is not WIDTHxHEIGHT", where, size))
		}
	}
	return problems
}

// isSitePath reports whether value is an absolute path on this origin ("/x"), not a URL or a
// protocol-relative "//host" reference.
func isSitePath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, `\`)
}

// CheckFiles returns problems with the files a manifest references: each icon and screenshot
// must exist under root (the frontend directory) and, for PNGs, be a PNG of the declared size.
func CheckFiles(manifest []byte, root string) []string {
	m, err := Parse(manifest)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	check := func(where string, image Image) {
		if !isSitePath(image.Src) {
			return // reported by Validate
		}
		file := filepath.Join(root, filepath.FromSlash(path.Clean(image.Src)))
		data, err := os.ReadFile(file)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %s is missing from the frontend", where, image.Src))
			return
		}
		if image.Type != "image/png" {
			return
		}
		width, height, err := pngSize(data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %s is not a PNG: %v", where, image.Src, err))
			return
		}
		declared := strings.Fields(image.Sizes)
		actual := strconv.Itoa(width) + "x" + strconv.Itoa(height)
		if len(declared) == 1 && declared[0] != "any" && declared[0] != actual {
			problems = append(problems, fmt.Sprintf("%s %s is %s, but the manifest says %s", where, image.Src, actual, declared[0]))
		}
	}
	for i, icon := range m.Icons {
		check(fmt.Sprintf("icons[%d]", i), icon)
	}
	for i, shot := range m.Screenshots {
		check(fmt.Sprintf("screenshots[%d]", i), shot)
	}
	return problems
}

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// pngSize reads a PNG's width and height from its IHDR chunk.
func pngSize(data []byte) (int, int, error) {
	if len(data) < 24 || !bytes.Equal(data[:8], pngSignature) {
		return 0, 0, errors.New("no PNG signature")
	}
	if string(data[12:16]) != "IHDR" {
		return 0, 0, errors.New("first chunk is not IHDR")
	}
	return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), nil
}
