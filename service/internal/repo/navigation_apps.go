package repo

import (
	"context"
	"fmt"
)

// NavigationApp groups sidebar navigation links under a titled section.
type NavigationApp struct {
	ID        int
	Title     string
	Ordinal   int
	MemberIDs []int // navigation_item_id in display order (app_navigation_links.ordinal)
}

// GetNavigationApps returns apps ordered by ordinal, title, id with member navigation item ids per app.
func (r *Repository) GetNavigationApps(ctx context.Context) ([]NavigationApp, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, title, ordinal
		FROM apps
		ORDER BY ordinal ASC, title ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]NavigationApp, 0, 8)
	for rows.Next() {
		var app NavigationApp
		if err := rows.Scan(&app.ID, &app.Title, &app.Ordinal); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return apps, nil
	}

	linkRows, err := r.db.QueryxContext(ctx, `
		SELECT app_id, navigation_item_id, ordinal
		FROM app_navigation_links
		ORDER BY app_id ASC, ordinal ASC, navigation_item_id ASC`)
	if err != nil {
		return nil, err
	}
	defer linkRows.Close()

	membersByApp := make(map[int][]int)
	for linkRows.Next() {
		var appID, navID, ordinal int
		if err := linkRows.Scan(&appID, &navID, &ordinal); err != nil {
			return nil, err
		}
		_ = ordinal
		membersByApp[appID] = append(membersByApp[appID], navID)
	}
	if err := linkRows.Err(); err != nil {
		return nil, err
	}

	for i := range apps {
		apps[i].MemberIDs = membersByApp[apps[i].ID]
	}
	return apps, nil
}

// NavigationItemIDsInApps returns navigation_item_id values assigned to any app.
func (r *Repository) NavigationItemIDsInApps(ctx context.Context) (map[int]struct{}, error) {
	rows, err := r.db.QueryxContext(ctx, `SELECT navigation_item_id FROM app_navigation_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
