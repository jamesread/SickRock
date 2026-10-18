package health

import (
	"context"
	"fmt"
	"strings"

	"github.com/jamesread/SickRock/internal/repo"
)

// NavigationBrokenLinksCheck finds navigation links whose table configuration, dashboard or
// workflow was deleted (or that point at nothing), which silently disappear from the sidebar.
type NavigationBrokenLinksCheck struct {
	Repo *repo.Repository
}

func (c NavigationBrokenLinksCheck) ID() string    { return "navigation.broken_links" }
func (c NavigationBrokenLinksCheck) Title() string { return "Broken navigation links" }

func (c NavigationBrokenLinksCheck) Run(ctx context.Context) ([]Issue, error) {
	links, err := c.Repo.FindBrokenNavigationLinks(ctx)
	if err != nil {
		return nil, err
	}
	issues := make([]Issue, 0, len(links))
	for _, link := range links {
		var missing []string
		if link.MissingTable {
			missing = append(missing, fmt.Sprintf("table configuration #%d", link.TableConfiguration))
		}
		if link.MissingDashboard {
			missing = append(missing, fmt.Sprintf("dashboard #%d", link.DashboardID))
		}
		if link.MissingWorkflow {
			missing = append(missing, fmt.Sprintf("workflow #%d", link.WorkflowID))
		}

		title := fmt.Sprintf("Navigation link %s points to a deleted %s", navigationLabel(link), strings.Join(missing, " and "))
		detail := "The link is hidden from the sidebar and from users' bookmarks. Point it at an existing item, or delete it."
		if len(missing) == 0 {
			title = fmt.Sprintf("Navigation link %s does not point to anything", navigationLabel(link))
			detail = "It has no table configuration, dashboard or workflow set, so it never appears in the sidebar. Set a target, or delete it."
		}
		issues = append(issues, Issue{
			Key:      fmt.Sprintf("nav:%d", link.ID),
			Severity: SeverityWarning,
			Title:    title,
			Detail:   detail,
			Link:     fmt.Sprintf("/table/table_navigation/%d/edit", link.ID),
		})
	}
	return issues, nil
}

func navigationLabel(link repo.BrokenNavigationLink) string {
	switch {
	case link.Title != "":
		return fmt.Sprintf("%q (#%d)", link.Title, link.ID)
	case link.Name != "":
		return fmt.Sprintf("%q (#%d)", link.Name, link.ID)
	default:
		return fmt.Sprintf("#%d", link.ID)
	}
}

// BookmarksOrphanedCheck finds bookmarks pointing at deleted navigation links. They are skipped
// when bookmarks load, so this is clean-up rather than a user-facing fault.
type BookmarksOrphanedCheck struct {
	Repo *repo.Repository
}

func (c BookmarksOrphanedCheck) ID() string    { return "bookmarks.orphaned" }
func (c BookmarksOrphanedCheck) Title() string { return "Bookmarks to deleted navigation links" }

func (c BookmarksOrphanedCheck) Run(ctx context.Context) ([]Issue, error) {
	bookmarks, err := c.Repo.FindOrphanedBookmarks(ctx)
	if err != nil {
		return nil, err
	}
	issues := make([]Issue, 0, len(bookmarks))
	for _, b := range bookmarks {
		issues = append(issues, Issue{
			Key:      fmt.Sprintf("bookmark:%d", b.ID),
			Severity: SeverityInfo,
			Title:    fmt.Sprintf("Bookmark #%d (user #%d) points to deleted navigation link #%d", b.ID, b.UserID, b.NavigationItem),
			Detail:   "The bookmark is no longer shown. Delete the row from table_user_bookmarks to clean up.",
		})
	}
	return issues, nil
}
