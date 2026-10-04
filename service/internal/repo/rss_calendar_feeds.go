package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"

	"github.com/jamesread/SickRock/internal/rsscalendar"
)

// RssRefreshResult summarizes one feed refresh run.
type RssRefreshResult struct {
	Created int
	Updated int
	Message string
}

// RssCalendarFeed configures importing calendar events from an RSS/Atom URL.
type RssCalendarFeed struct {
	ID                     int
	Name                   string
	FeedURL                string
	TableConfiguration     string
	FieldMappingJSON       string
	UniqueColumn           string
	RefreshIntervalMinutes int
	Enabled                bool
	LastRefreshAt          sql.NullTime
	LastRefreshStatus      string
	LastRefreshMessage     string
	LastItemsCreated       int
	LastItemsUpdated       int
}

func (r *Repository) ListRssCalendarFeeds(ctx context.Context) ([]RssCalendarFeed, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, name, feed_url, table_configuration, field_mapping_json, unique_column,
		       refresh_interval_minutes, enabled, last_refresh_at, last_refresh_status,
		       last_refresh_message, last_items_created, last_items_updated
		FROM rss_calendar_feeds
		ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RssCalendarFeed, 0, 8)
	for rows.Next() {
		feed, err := scanRssCalendarFeedRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, feed)
	}
	return out, rows.Err()
}

func (r *Repository) GetRssCalendarFeedByID(ctx context.Context, id int) (*RssCalendarFeed, error) {
	row := r.db.QueryRowxContext(ctx, `
		SELECT id, name, feed_url, table_configuration, field_mapping_json, unique_column,
		       refresh_interval_minutes, enabled, last_refresh_at, last_refresh_status,
		       last_refresh_message, last_items_created, last_items_updated
		FROM rss_calendar_feeds
		WHERE id = ?`, id)
	feed, err := scanRssCalendarFeedRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("rss feed not found")
		}
		return nil, err
	}
	return &feed, nil
}

func (r *Repository) CreateRssCalendarFeed(ctx context.Context, feed *RssCalendarFeed) (int, error) {
	if feed == nil {
		return 0, fmt.Errorf("feed is required")
	}
	feed.Name = strings.TrimSpace(feed.Name)
	feed.FeedURL = strings.TrimSpace(feed.FeedURL)
	feed.TableConfiguration = strings.TrimSpace(feed.TableConfiguration)
	if feed.Name == "" || feed.FeedURL == "" || feed.TableConfiguration == "" {
		return 0, fmt.Errorf("name, feed URL, and table configuration are required")
	}
	if feed.RefreshIntervalMinutes <= 0 {
		feed.RefreshIntervalMinutes = 60
	}
	if strings.TrimSpace(feed.FieldMappingJSON) == "" {
		feed.FieldMappingJSON = "{}"
	}

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO rss_calendar_feeds (
			name, feed_url, table_configuration, field_mapping_json, unique_column,
			refresh_interval_minutes, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		feed.Name,
		feed.FeedURL,
		feed.TableConfiguration,
		feed.FieldMappingJSON,
		feed.UniqueColumn,
		feed.RefreshIntervalMinutes,
		boolToInt(feed.Enabled),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *Repository) UpdateRssCalendarFeed(ctx context.Context, feed *RssCalendarFeed) error {
	if feed == nil || feed.ID <= 0 {
		return fmt.Errorf("feed id is required")
	}
	feed.Name = strings.TrimSpace(feed.Name)
	feed.FeedURL = strings.TrimSpace(feed.FeedURL)
	feed.TableConfiguration = strings.TrimSpace(feed.TableConfiguration)
	if feed.Name == "" || feed.FeedURL == "" || feed.TableConfiguration == "" {
		return fmt.Errorf("name, feed URL, and table configuration are required")
	}
	if feed.RefreshIntervalMinutes <= 0 {
		feed.RefreshIntervalMinutes = 60
	}
	if strings.TrimSpace(feed.FieldMappingJSON) == "" {
		feed.FieldMappingJSON = "{}"
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE rss_calendar_feeds SET
			name = ?,
			feed_url = ?,
			table_configuration = ?,
			field_mapping_json = ?,
			unique_column = ?,
			refresh_interval_minutes = ?,
			enabled = ?,
			sr_updated = `+r.sqlCurrentTimestampExpr()+`
		WHERE id = ?`,
		feed.Name,
		feed.FeedURL,
		feed.TableConfiguration,
		feed.FieldMappingJSON,
		feed.UniqueColumn,
		feed.RefreshIntervalMinutes,
		boolToInt(feed.Enabled),
		feed.ID,
	)
	return err
}

func (r *Repository) DeleteRssCalendarFeed(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rss_calendar_feeds WHERE id = ?`, id)
	return err
}

func (r *Repository) UpdateRssCalendarFeedRefreshStatus(
	ctx context.Context,
	id int,
	status string,
	message string,
	created int,
	updated int,
	at time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE rss_calendar_feeds SET
			last_refresh_at = ?,
			last_refresh_status = ?,
			last_refresh_message = ?,
			last_items_created = ?,
			last_items_updated = ?,
			sr_updated = `+r.sqlCurrentTimestampExpr()+`
		WHERE id = ?`,
		at,
		status,
		message,
		created,
		updated,
		id,
	)
	return err
}

// ListRssCalendarFeedsDueForRefresh returns enabled feeds past their refresh interval.
func (r *Repository) ListRssCalendarFeedsDueForRefresh(ctx context.Context, now time.Time) ([]RssCalendarFeed, error) {
	feeds, err := r.ListRssCalendarFeeds(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RssCalendarFeed, 0, len(feeds))
	for _, feed := range feeds {
		if !feed.Enabled {
			continue
		}
		interval := time.Duration(feed.RefreshIntervalMinutes) * time.Minute
		if interval <= 0 {
			interval = time.Hour
		}
		if !feed.LastRefreshAt.Valid {
			out = append(out, feed)
			continue
		}
		if now.Sub(feed.LastRefreshAt.Time) >= interval {
			out = append(out, feed)
		}
	}
	return out, nil
}

func (r *Repository) RefreshRssCalendarFeedByID(ctx context.Context, id int) (RssRefreshResult, error) {
	feed, err := r.GetRssCalendarFeedByID(ctx, id)
	if err != nil {
		return RssRefreshResult{}, err
	}
	return r.RefreshRssCalendarFeed(ctx, feed)
}

func (r *Repository) RefreshRssCalendarFeed(ctx context.Context, feed *RssCalendarFeed) (RssRefreshResult, error) {
	if feed == nil {
		return RssRefreshResult{}, fmt.Errorf("feed is required")
	}
	now := time.Now()
	cfg, err := rsscalendar.ParseFieldMappingConfig(feed.FieldMappingJSON)
	if err != nil {
		_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feed.ID, "error", err.Error(), 0, 0, now)
		return RssRefreshResult{}, err
	}

	log.WithFields(log.Fields{
		"feedID":             feed.ID,
		"feedName":           feed.Name,
		"feedURL":            feed.FeedURL,
		"tableConfiguration": feed.TableConfiguration,
	}).Info("RSS calendar: refreshing feed")

	parsed, err := rsscalendar.FetchFeed(ctx, feed.FeedURL)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"feedID":  feed.ID,
			"feedURL": feed.FeedURL,
		}).Warn("RSS calendar: refresh fetch/parse failed")
		_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feed.ID, "error", err.Error(), 0, 0, now)
		return RssRefreshResult{}, err
	}

	result, err := r.syncRssFeedItems(ctx, feed.TableConfiguration, feed.UniqueColumn, cfg.Mappings, parsed.Items)
	if err != nil {
		_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feed.ID, "error", err.Error(), 0, 0, now)
		return RssRefreshResult{}, err
	}

	_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feed.ID, "ok", result.Message, result.Created, result.Updated, now)
	return result, nil
}

func (r *Repository) syncRssFeedItems(
	ctx context.Context,
	tableConfiguration string,
	uniqueColumn string,
	mappings []rsscalendar.FieldMapping,
	items []*gofeed.Item,
) (RssRefreshResult, error) {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return RssRefreshResult{}, fmt.Errorf("table configuration is required")
	}
	uniqueColumn = strings.TrimSpace(uniqueColumn)

	result := RssRefreshResult{}
	for _, item := range items {
		itemValues := rsscalendar.ItemFields(item)
		fields := rsscalendar.ApplyMappings(itemValues, mappings)
		if len(fields) == 0 {
			continue
		}

		if uniqueColumn != "" {
			uniqueValue := strings.TrimSpace(fields[uniqueColumn])
			if uniqueValue == "" {
				uniqueValue = strings.TrimSpace(itemValues["guid"])
				if uniqueValue != "" {
					fields[uniqueColumn] = uniqueValue
				}
			}
			if uniqueValue != "" {
				existing, err := r.ListItemsInTable(ctx, tableConfiguration, map[string]string{uniqueColumn: uniqueValue})
				if err != nil {
					return result, err
				}
				if len(existing) > 0 {
					rowID := existing[0].ID
					merged := map[string]string{}
					for k, v := range existing[0].Fields {
						if v != nil {
							merged[k] = fmt.Sprint(v)
						}
					}
					for k, v := range fields {
						merged[k] = v
					}
					_, err := r.EditItemInTableWithFields(ctx, tableConfiguration, rowID, "", merged)
					if err != nil {
						return result, err
					}
					result.Updated++
					continue
				}
			}
		}

		_, err := r.CreateItemInTable(ctx, tableConfiguration, fields)
		if err != nil {
			return result, err
		}
		result.Created++
	}

	if result.Created == 0 && result.Updated == 0 {
		result.Message = "No items imported (empty feed or no mappings matched)."
	} else {
		result.Message = fmt.Sprintf("Imported %d new, updated %d existing row(s).", result.Created, result.Updated)
	}
	return result, nil
}

func scanRssCalendarFeedRow(scanner interface {
	StructScan(dest interface{}) error
}) (RssCalendarFeed, error) {
	var row struct {
		ID                     int            `db:"id"`
		Name                   string         `db:"name"`
		FeedURL                string         `db:"feed_url"`
		TableConfiguration     string         `db:"table_configuration"`
		FieldMappingJSON       string         `db:"field_mapping_json"`
		UniqueColumn           string         `db:"unique_column"`
		RefreshIntervalMinutes int            `db:"refresh_interval_minutes"`
		Enabled                int            `db:"enabled"`
		LastRefreshAt          flexibleTime   `db:"last_refresh_at"`
		LastRefreshStatus      string         `db:"last_refresh_status"`
		LastRefreshMessage     string         `db:"last_refresh_message"`
		LastItemsCreated       int            `db:"last_items_created"`
		LastItemsUpdated       int            `db:"last_items_updated"`
	}
	if err := scanner.StructScan(&row); err != nil {
		return RssCalendarFeed{}, err
	}
	return RssCalendarFeed{
		ID:                     row.ID,
		Name:                   row.Name,
		FeedURL:                row.FeedURL,
		TableConfiguration:     row.TableConfiguration,
		FieldMappingJSON:       row.FieldMappingJSON,
		UniqueColumn:           row.UniqueColumn,
		RefreshIntervalMinutes: row.RefreshIntervalMinutes,
		Enabled:                row.Enabled != 0,
		LastRefreshAt:          row.LastRefreshAt.NullTime,
		LastRefreshStatus:      row.LastRefreshStatus,
		LastRefreshMessage:     row.LastRefreshMessage,
		LastItemsCreated:       row.LastItemsCreated,
		LastItemsUpdated:       row.LastItemsUpdated,
	}, nil
}

// flexibleTime scans SQLite datetime strings and MySQL time.Time values.
type flexibleTime struct {
	sql.NullTime
}

func (ft *flexibleTime) Scan(value interface{}) error {
	if value == nil {
		ft.Valid = false
		ft.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		ft.Time = v
		ft.Valid = true
		return nil
	case string:
		return ft.scanString(v)
	case []byte:
		return ft.scanString(string(v))
	default:
		return fmt.Errorf("unsupported last_refresh_at type %T", value)
	}
}

func (ft *flexibleTime) scanString(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		ft.Valid = false
		ft.Time = time.Time{}
		return nil
	}
	parsed, ok := parseFlexibleDateTime(raw)
	if !ok {
		ft.Valid = false
		ft.Time = time.Time{}
		return nil
	}
	ft.Time = parsed
	ft.Valid = true
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
