package health

import (
	"context"
	"crypto/sha256"
	"fmt"

	armhealth "github.com/jamesread/armature-health"

	"github.com/jamesread/SickRock/internal/pwa"
)

// PWACheck finds problems that stop SickRock installing as an app or working offline: a
// missing or invalid manifest template, icons that are missing or the wrong size, no manifest
// link in index.html, or a missing service worker or offline page. They come from the
// deployed frontend files, so the fix is a corrected build rather than a setting.
type PWACheck struct {
	FrontendDir string
}

func (c PWACheck) ID() string    { return "pwa.installable" }
func (c PWACheck) Title() string { return "Installable app (PWA)" }

func (c PWACheck) Run(_ context.Context) ([]armhealth.Issue, error) {
	problems := pwa.FrontendProblems(c.FrontendDir)
	issues := make([]armhealth.Issue, 0, len(problems))
	for _, problem := range problems {
		sum := sha256.Sum256([]byte(problem))
		issues = append(issues, armhealth.Issue{
			Key:      fmt.Sprintf("pwa:%x", sum[:8]),
			Severity: armhealth.SeverityWarning,
			Title:    "Installable app: " + problem,
			Detail: fmt.Sprintf("SickRock may not install as an app, or work offline, until this is fixed. "+
				"It comes from the frontend files in %s, so rebuild or redeploy the frontend.", c.FrontendDir),
		})
	}
	return issues, nil
}
