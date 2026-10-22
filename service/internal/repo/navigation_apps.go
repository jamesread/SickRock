package repo

import (
	"context"
	"fmt"
)

// NavigationApp groups sidebar navigation links under a titled section.
type NavigationApp struct {
	Title     string
	MemberIDs []int
	ID        int
	Ordinal   int
}

// GetNavigationApps returns apps ordered by ordinal, title, id with member navigation item ids per app.
func (r *Repository) GetNavigationApps(ctx context.Context) ([]NavigationApp, error) {
	apps, err := r.listNavigationApps(ctx)
	if err != nil || len(apps) == 0 {
		return apps, err
	}

	membersByApp, err := r.navigationAppMembers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		apps[i].MemberIDs = membersByApp[apps[i].ID]
	}
	return apps, nil
}

// listNavigationApps returns the sidebar apps in display order, without their members.
func (r *Repository) listNavigationApps(ctx context.Context) ([]NavigationApp, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, title, ordinal
		FROM apps
		ORDER BY ordinal ASC, title ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	apps := make([]NavigationApp, 0, 8)
	for rows.Next() {
		var app NavigationApp
		if err = rows.Scan(&app.ID, &app.Title, &app.Ordinal); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return apps, nil
}

// navigationAppMembers returns each app's navigation item ids in link order.
func (r *Repository) navigationAppMembers(ctx context.Context) (map[int][]int, error) {
	linkRows, err := r.db.QueryxContext(ctx, `
		SELECT app_id, navigation_item_id
		FROM app_navigation_links
		ORDER BY app_id ASC, ordinal ASC, navigation_item_id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = linkRows.Close() }()

	membersByApp := make(map[int][]int)
	for linkRows.Next() {
		var appID, navID int
		if err := linkRows.Scan(&appID, &navID); err != nil {
			return nil, err
		}
		membersByApp[appID] = append(membersByApp[appID], navID)
	}
	return membersByApp, linkRows.Err()
}

// NavigationItemIDsInApps returns navigation_item_id values assigned to any app.
func (r *Repository) NavigationItemIDsInApps(ctx context.Context) (map[int]struct{}, error) {
	rows, err := r.db.QueryxContext(ctx, `SELECT navigation_item_id FROM app_navigation_links`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

// AssignNavigationItemToApp links a table_navigation row to a sidebar app.
func (r *Repository) AssignNavigationItemToApp(ctx context.Context, appID, navigationItemID, ordinal int) error {
	if appID <= 0 || navigationItemID <= 0 {
		return fmt.Errorf("app id and navigation item id are required")
	}
	if ordinal <= 0 {
		ordinal = 99
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app_navigation_links (app_id, navigation_item_id, ordinal)
		VALUES (?, ?, ?)`,
		appID, navigationItemID, ordinal,
	)
	return err
}
