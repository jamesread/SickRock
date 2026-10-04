package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/rsscalendar"
)

func (s *SickRockServer) requireExportsManage(ctx context.Context) error {
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && (au.RBAC.IsSuperuser || au.RBAC.Has(exportsManagePermission)) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("exports.manage permission required"))
}

func rssFeedToProto(feed *repo.RssCalendarFeed) (*sickrockpb.RssCalendarFeed, error) {
	if feed == nil {
		return nil, nil
	}
	cfg, err := rsscalendar.ParseFieldMappingConfig(feed.FieldMappingJSON)
	if err != nil {
		return nil, err
	}
	mappings := make([]*sickrockpb.RssFieldMapping, 0, len(cfg.Mappings))
	for _, m := range cfg.Mappings {
		mappings = append(mappings, &sickrockpb.RssFieldMapping{
			RssField: m.RssField,
			Column:   m.Column,
		})
	}
	out := &sickrockpb.RssCalendarFeed{
		Id:                     int32(feed.ID),
		Name:                   feed.Name,
		FeedUrl:                feed.FeedURL,
		TableConfiguration:     feed.TableConfiguration,
		FieldMappings:          mappings,
		UniqueColumn:           feed.UniqueColumn,
		RefreshIntervalMinutes: int32(feed.RefreshIntervalMinutes),
		Enabled:                feed.Enabled,
		LastRefreshStatus:      feed.LastRefreshStatus,
		LastRefreshMessage:     feed.LastRefreshMessage,
		LastItemsCreated:       int32(feed.LastItemsCreated),
		LastItemsUpdated:       int32(feed.LastItemsUpdated),
	}
	if feed.LastRefreshAt.Valid {
		out.LastRefreshAt = feed.LastRefreshAt.Time.Unix()
	}
	return out, nil
}

func rssFeedFromProto(msg *sickrockpb.RssCalendarFeed) (*repo.RssCalendarFeed, string, error) {
	if msg == nil {
		return nil, "", fmt.Errorf("feed is required")
	}
	mappings := make([]rsscalendar.FieldMapping, 0, len(msg.GetFieldMappings()))
	for _, m := range msg.GetFieldMappings() {
		if m == nil {
			continue
		}
		mappings = append(mappings, rsscalendar.FieldMapping{
			RssField: m.GetRssField(),
			Column:   m.GetColumn(),
		})
	}
	cfg := rsscalendar.FieldMappingConfig{Mappings: mappings}
	encoded, err := rsscalendar.EncodeFieldMappingConfig(cfg)
	if err != nil {
		return nil, "", err
	}
	interval := int(msg.GetRefreshIntervalMinutes())
	if interval <= 0 {
		interval = 60
	}
	feed := &repo.RssCalendarFeed{
		ID:                     int(msg.GetId()),
		Name:                   strings.TrimSpace(msg.GetName()),
		FeedURL:                strings.TrimSpace(msg.GetFeedUrl()),
		TableConfiguration:     strings.TrimSpace(msg.GetTableConfiguration()),
		FieldMappingJSON:       encoded,
		UniqueColumn:           strings.TrimSpace(msg.GetUniqueColumn()),
		RefreshIntervalMinutes: interval,
		Enabled:                msg.GetEnabled(),
	}
	return feed, encoded, nil
}

func (s *SickRockServer) ListRssCalendarFeeds(ctx context.Context, _ *connect.Request[sickrockpb.ListRssCalendarFeedsRequest]) (*connect.Response[sickrockpb.ListRssCalendarFeedsResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListRssCalendarFeeds(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*sickrockpb.RssCalendarFeed, 0, len(rows))
	for _, row := range rows {
		pb, err := rssFeedToProto(&row)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, pb)
	}
	return connect.NewResponse(&sickrockpb.ListRssCalendarFeedsResponse{Feeds: out}), nil
}

func (s *SickRockServer) GetRssCalendarFeed(ctx context.Context, req *connect.Request[sickrockpb.GetRssCalendarFeedRequest]) (*connect.Response[sickrockpb.GetRssCalendarFeedResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	feed, err := s.repo.GetRssCalendarFeedByID(ctx, int(req.Msg.GetId()))
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	pb, err := rssFeedToProto(feed)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.GetRssCalendarFeedResponse{Feed: pb}), nil
}

func (s *SickRockServer) SaveRssCalendarFeed(ctx context.Context, req *connect.Request[sickrockpb.SaveRssCalendarFeedRequest]) (*connect.Response[sickrockpb.SaveRssCalendarFeedResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	feed, _, err := rssFeedFromProto(req.Msg.GetFeed())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if feed.ID > 0 {
		if err := s.repo.UpdateRssCalendarFeed(ctx, feed); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		id, err := s.repo.CreateRssCalendarFeed(ctx, feed)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		feed.ID = id
	}
	saved, err := s.repo.GetRssCalendarFeedByID(ctx, feed.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := rssFeedToProto(saved)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SaveRssCalendarFeedResponse{Feed: pb}), nil
}

func (s *SickRockServer) DeleteRssCalendarFeed(ctx context.Context, req *connect.Request[sickrockpb.DeleteRssCalendarFeedRequest]) (*connect.Response[sickrockpb.DeleteRssCalendarFeedResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	if err := s.repo.DeleteRssCalendarFeed(ctx, int(req.Msg.GetId())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.DeleteRssCalendarFeedResponse{Deleted: true}), nil
}

func (s *SickRockServer) RefreshRssCalendarFeed(ctx context.Context, req *connect.Request[sickrockpb.RefreshRssCalendarFeedRequest]) (*connect.Response[sickrockpb.RefreshRssCalendarFeedResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	feedID := int(req.Msg.GetId())
	result, err := s.repo.RefreshRssCalendarFeedByID(ctx, feedID)
	if err != nil {
		log.WithError(err).WithField("feedID", feedID).Warn("RSS calendar: manual refresh failed")
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	feed, err := s.repo.GetRssCalendarFeedByID(ctx, int(req.Msg.GetId()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := rssFeedToProto(feed)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.RefreshRssCalendarFeedResponse{
		Feed:          pb,
		Message:       result.Message,
		ItemsCreated:  int32(result.Created),
		ItemsUpdated:  int32(result.Updated),
	}), nil
}

func (s *SickRockServer) PreviewRssCalendarFeed(ctx context.Context, req *connect.Request[sickrockpb.PreviewRssCalendarFeedRequest]) (*connect.Response[sickrockpb.PreviewRssCalendarFeedResponse], error) {
	if err := s.requireExportsManage(ctx); err != nil {
		return nil, err
	}
	feedURL := strings.TrimSpace(req.Msg.GetFeedUrl())
	parsed, err := rsscalendar.FetchFeed(ctx, feedURL)
	if err != nil {
		log.WithError(err).WithField("feedURL", feedURL).Warn("RSS calendar: preview failed")
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	fields := rsscalendar.DiscoverFields(parsed.Items)
	sample := make([]*sickrockpb.RssFieldMapping, 0)
	if len(parsed.Items) > 0 {
		values := rsscalendar.ItemFields(parsed.Items[0])
		for _, name := range fields {
			if v := strings.TrimSpace(values[name]); v != "" {
				sample = append(sample, &sickrockpb.RssFieldMapping{
					RssField: name,
					Column:   v,
				})
			}
		}
	}
	return connect.NewResponse(&sickrockpb.PreviewRssCalendarFeedResponse{
		RssFields:    fields,
		SampleValues: sample,
	}), nil
}

// RefreshDueRssCalendarFeeds is called by the background scheduler.
func (s *SickRockServer) RefreshDueRssCalendarFeeds(ctx context.Context) {
	feeds, err := s.repo.ListRssCalendarFeedsDueForRefresh(ctx, time.Now())
	if err != nil {
		return
	}
	for _, feed := range feeds {
		_, err := s.repo.RefreshRssCalendarFeed(ctx, &feed)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"feedID":  feed.ID,
				"feedURL": feed.FeedURL,
			}).Warn("RSS calendar: scheduled refresh failed")
			continue
		}
	}
}
