package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"

	"github.com/jamesread/SickRock/internal/rsscalendar"
)

// RssRefreshResult summarizes one feed refresh run.
type RssRefreshResult struct {
	Message string
	Created int
	Updated int
}

// RssCalendarFeed configures importing calendar events from an RSS/Atom URL.
type RssCalendarFeed struct {
	LastRefreshAt          sql.NullTime
	LastRefreshStatus      string
	FeedURL                string
	TableConfiguration     string
	FieldMappingJSON       string
	UniqueColumn           string
	FeedIDColumn           string // optional column each imported event records the feed ID in
	Name                   string
	LastRefreshMessage     string
	RefreshIntervalMinutes int
	ID                     int
	LastItemsCreated       int
	LastItemsUpdated       int
	Enabled                bool
}

func (r *Repository) ListRssCalendarFeeds(ctx context.Context) ([]RssCalendarFeed, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, name, feed_url, table_configuration, field_mapping_json, unique_column, feed_id_column,
		       refresh_interval_minutes, enabled, last_refresh_at, last_refresh_status,
		       last_refresh_message, last_items_created, last_items_updated
		FROM rss_calendar_feeds
		ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
		SELECT id, name, feed_url, table_configuration, field_mapping_json, unique_column, feed_id_column,
		       refresh_interval_minutes, enabled, last_refresh_at, last_refresh_status,
		       last_refresh_message, last_items_created, last_items_updated
		FROM rss_calendar_feeds
		WHERE id = ?`, id)
	feed, err := scanRssCalendarFeedRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	if err := normalizeRssCalendarFeed(feed); err != nil {
		return 0, err
	}

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO rss_calendar_feeds (
			name, feed_url, table_configuration, field_mapping_json, unique_column, feed_id_column,
			refresh_interval_minutes, enabled, last_refresh_status, last_refresh_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '')`,
		feed.Name,
		feed.FeedURL,
		feed.TableConfiguration,
		feed.FieldMappingJSON,
		feed.UniqueColumn,
		feed.FeedIDColumn,
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
	if err := normalizeRssCalendarFeed(feed); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE rss_calendar_feeds SET
			name = ?,
			feed_url = ?,
			table_configuration = ?,
			field_mapping_json = ?,
			unique_column = ?,
			feed_id_column = ?,
			refresh_interval_minutes = ?,
			enabled = ?,
			sr_updated = `+r.sqlCurrentTimestampExpr()+`
		WHERE id = ?`,
		feed.Name,
		feed.FeedURL,
		feed.TableConfiguration,
		feed.FieldMappingJSON,
		feed.UniqueColumn,
		feed.FeedIDColumn,
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
	return r.refreshRssCalendarFeed(ctx, feed, time.Now())
}

func (r *Repository) refreshRssCalendarFeed(ctx context.Context, feed *RssCalendarFeed, now time.Time) (RssRefreshResult, error) {
	cfg, err := rsscalendar.ParseFieldMappingConfig(feed.FieldMappingJSON)
	if err != nil {
		return r.failRssRefresh(ctx, feed.ID, now, err)
	}
	return r.syncAndRecordRssRefresh(ctx, feed, cfg, now)
}

func (r *Repository) syncAndRecordRssRefresh(ctx context.Context, feed *RssCalendarFeed, cfg rsscalendar.FieldMappingConfig, now time.Time) (RssRefreshResult, error) {
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
		return r.failRssRefresh(ctx, feed.ID, now, err)
	}

	result, err := r.syncRssFeedItems(ctx, feed, cfg.Mappings, parsed.Items)
	if err != nil {
		return r.failRssRefresh(ctx, feed.ID, now, err)
	}

	_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feed.ID, "ok", result.Message, result.Created, result.Updated, now)
	return result, nil
}

func (r *Repository) failRssRefresh(ctx context.Context, feedID int, now time.Time, err error) (RssRefreshResult, error) {
	_ = r.UpdateRssCalendarFeedRefreshStatus(ctx, feedID, "error", err.Error(), 0, 0, now)
	return RssRefreshResult{}, err
}

func (r *Repository) syncRssFeedItems(
	ctx context.Context,
	feed *RssCalendarFeed,
	mappings []rsscalendar.FieldMapping,
	items []*gofeed.Item,
) (RssRefreshResult, error) {
	tableConfiguration := strings.TrimSpace(feed.TableConfiguration)
	if tableConfiguration == "" {
		return RssRefreshResult{}, fmt.Errorf("table configuration is required")
	}
	uniqueColumn := strings.TrimSpace(feed.UniqueColumn)

	result := RssRefreshResult{}
	for _, item := range items {
		itemValues := rsscalendar.ItemFields(item)
		fields := rssFeedItemFields(feed, itemValues, mappings)
		if len(fields) == 0 {
			continue
		}
		updated, err := r.upsertRssFeedItem(ctx, tableConfiguration, uniqueColumn, itemValues["guid"], fields)
		if err != nil {
			return result, err
		}
		if updated {
			result.Updated++
		} else {
			result.Created++
		}
	}

	result.Message = rssRefreshMessage(result)
	return result, nil
}

// rssFeedItemFields maps a feed item's values to table fields and, when the feed has a feed ID column
// and any mapping matched, records the feed's ID in that column.
func rssFeedItemFields(feed *RssCalendarFeed, itemValues map[string]string, mappings []rsscalendar.FieldMapping) map[string]string {
	fields := rsscalendar.ApplyMappings(itemValues, mappings)
	if feedIDColumn := strings.TrimSpace(feed.FeedIDColumn); feedIDColumn != "" && len(fields) > 0 {
		fields[feedIDColumn] = strconv.Itoa(feed.ID)
	}
	return fields
}

func rssRefreshMessage(result RssRefreshResult) string {
	if result.Created == 0 && result.Updated == 0 {
		return "No items imported (empty feed or no mappings matched)."
	}
	return fmt.Sprintf("Imported %d new, updated %d existing row(s).", result.Created, result.Updated)
}

// upsertRssFeedItem updates the row whose uniqueColumn matches the item (falling back to its guid when
// the mapped value is empty), or creates a new row. It reports whether an existing row was updated.
func (r *Repository) upsertRssFeedItem(ctx context.Context, tableConfiguration, uniqueColumn, guid string, fields map[string]string) (bool, error) {
	existing, err := r.findRssFeedRow(ctx, tableConfiguration, uniqueColumn, guid, fields)
	if err != nil {
		return false, err
	}
	if existing == nil {
		_, err = r.CreateItemInTable(ctx, tableConfiguration, fields)
		return false, err
	}
	merged := map[string]string{}
	for k, v := range existing.Fields {
		if v != nil {
			merged[k] = fmt.Sprint(v)
		}
	}
	maps.Copy(merged, fields)
	_, err = r.EditItemInTableWithFields(ctx, tableConfiguration, existing.ID, "", merged)
	return true, err
}

// findRssFeedRow returns the existing row matching the item's unique value, or nil. When the mapped
// unique value is empty the guid is used and also written into fields.
func (r *Repository) findRssFeedRow(ctx context.Context, tableConfiguration, uniqueColumn, guid string, fields map[string]string) (*Item, error) {
	if uniqueColumn == "" {
		return nil, nil
	}
	uniqueValue := strings.TrimSpace(fields[uniqueColumn])
	if uniqueValue == "" {
		uniqueValue = strings.TrimSpace(guid)
		if uniqueValue == "" {
			return nil, nil
		}
		fields[uniqueColumn] = uniqueValue
	}
	existing, err := r.ListItemsInTable(ctx, tableConfiguration, map[string]string{uniqueColumn: uniqueValue})
	if err != nil || len(existing) == 0 {
		return nil, err
	}
	return &existing[0], nil
}

func scanRssCalendarFeedRow(scanner interface {
	StructScan(dest any) error
}) (RssCalendarFeed, error) {
	var row struct {
		LastRefreshAt          flexibleTime `db:"last_refresh_at"`
		LastRefreshStatus      string       `db:"last_refresh_status"`
		FeedURL                string       `db:"feed_url"`
		TableConfiguration     string       `db:"table_configuration"`
		FieldMappingJSON       string       `db:"field_mapping_json"`
		UniqueColumn           string       `db:"unique_column"`
		FeedIDColumn           string       `db:"feed_id_column"`
		Name                   string       `db:"name"`
		LastRefreshMessage     string       `db:"last_refresh_message"`
		RefreshIntervalMinutes int          `db:"refresh_interval_minutes"`
		Enabled                int          `db:"enabled"`
		ID                     int          `db:"id"`
		LastItemsCreated       int          `db:"last_items_created"`
		LastItemsUpdated       int          `db:"last_items_updated"`
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
		FeedIDColumn:           row.FeedIDColumn,
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

func (ft *flexibleTime) Scan(value any) error {
	if value == nil {
		return ft.clear()
	}
	return ft.scanValue(value)
}

func (ft *flexibleTime) clear() error {
	ft.Valid = false
	ft.Time = time.Time{}
	return nil
}

func (ft *flexibleTime) scanValue(value any) error {
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

// normalizeRssCalendarFeed trims a feed's fields, checks the required ones are set and applies defaults
// (60 minute refresh interval, empty field mapping).
func normalizeRssCalendarFeed(feed *RssCalendarFeed) error {
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
	return nil
}
