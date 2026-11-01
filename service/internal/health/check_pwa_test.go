package health

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	armhealth "github.com/jamesread/armature-health"
)

func TestPWACheckReportsABrokenFrontendWithStableKeys(t *testing.T) {
	check := PWACheck{FrontendDir: t.TempDir()}
	first, err := check.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 4 {
		t.Fatalf("empty frontend: want an issue for each missing part, got %d: %+v", len(first), first)
	}
	second, _ := check.Run(context.Background())
	for i := range first {
		issue := first[i]
		if issue.Severity != armhealth.SeverityWarning || !strings.HasPrefix(issue.Key, "pwa:") || issue.Title == "" {
			t.Errorf("issue %+v: want a warning with a pwa: key and a title", issue)
		}
		if second[i].Key != issue.Key {
			t.Errorf("key changed between runs: %q then %q", issue.Key, second[i].Key)
		}
	}
}

func TestPWACheckPassesForTheShippedFrontend(t *testing.T) {
	dir := t.TempDir()
	public := filepath.Join("..", "..", "..", "frontend", "public")
	err := filepath.Walk(public, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(public, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		return copyTestFile(p, filepath.Join(dir, rel))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := copyTestFile(filepath.Join("..", "..", "..", "frontend", "index.html"), filepath.Join(dir, "index.html")); err != nil {
		t.Fatal(err)
	}

	issues, err := PWACheck{FrontendDir: dir}.Run(context.Background())
	if err != nil || len(issues) > 0 {
		t.Fatalf("shipped frontend: issues %+v, err %v", issues, err)
	}
}

func copyTestFile(from, to string) error {
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
