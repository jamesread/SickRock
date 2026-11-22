package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/buildinfo"
	"github.com/jamesread/SickRock/internal/uiusage"
	armlayer "github.com/jamesread/armature-iam/layer"
	log "github.com/sirupsen/logrus"
)

// UI usage tracking: a user who opts in ("Track and log UI usage") has their navigation through
// routes, the sidebar and the header recorded by the client and appended to their own JSONL file
// (uiusage.Store), which only they can download.

// UIUsageLogPath is where a user downloads their own log, below the /api prefix.
const UIUsageLogPath = "/ui-usage.jsonl"

var errNoUIUsageStore = connect.NewError(connect.CodeUnavailable, errors.New("UI usage logging is not available on this server"))

// SetUIUsageStore sets where UI usage logs are kept. Without one, tracking cannot be turned on.
func (s *SickRockServer) SetUIUsageStore(store *uiusage.Store) {
	s.uiUsage = store
}

func (s *SickRockServer) GetUIUsageSettings(ctx context.Context, _ *connect.Request[sickrockpb.GetUIUsageSettingsRequest]) (*connect.Response[sickrockpb.GetUIUsageSettingsResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	enabled, err := s.repo.UIUsageTracking(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load UI usage setting: %w", err))
	}
	res := &sickrockpb.GetUIUsageSettingsResponse{TrackingEnabled: enabled && s.uiUsage != nil, MaxLines: uiusage.MaxLines}
	if s.uiUsage != nil {
		count, err := s.uiUsage.LineCount(userID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read UI usage log: %w", err))
		}
		res.LineCount = toInt32(count)
	}
	return connect.NewResponse(res), nil
}

func (s *SickRockServer) SetUIUsageTracking(ctx context.Context, req *connect.Request[sickrockpb.SetUIUsageTrackingRequest]) (*connect.Response[sickrockpb.SetUIUsageTrackingResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	enabled := req.Msg.GetEnabled()
	if enabled && s.uiUsage == nil {
		return nil, errNoUIUsageStore
	}
	if err := s.repo.SetUIUsageTracking(ctx, userID, enabled); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save UI usage setting: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetUIUsageTrackingResponse{TrackingEnabled: enabled}), nil
}

// RecordUIUsage appends the caller's events to their log. Events from a user who has not opted in
// are dropped, and the response tells the client to stop.
func (s *SickRockServer) RecordUIUsage(ctx context.Context, req *connect.Request[sickrockpb.RecordUIUsageRequest]) (*connect.Response[sickrockpb.RecordUIUsageResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	enabled, err := s.repo.UIUsageTracking(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load UI usage setting: %w", err))
	}
	if !enabled || s.uiUsage == nil {
		return connect.NewResponse(&sickrockpb.RecordUIUsageResponse{}), nil
	}
	events := make([]uiusage.Event, 0, len(req.Msg.GetEvents()))
	for _, e := range req.Msg.GetEvents() {
		events = append(events, uiusage.Event{
			Kind:        e.GetKind(),
			FromPath:    e.GetFromPath(),
			ToPath:      e.GetToPath(),
			Label:       e.GetLabel(),
			TimestampMs: e.GetTimestampMs(),
			SessionID:   e.GetSessionId(),
			AppVersion:  e.GetAppVersion(),
		})
	}
	n, err := s.uiUsage.Record(userID, events, buildinfo.Version, time.Now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write UI usage log: %w", err))
	}
	return connect.NewResponse(&sickrockpb.RecordUIUsageResponse{Accepted: toInt32(n), TrackingEnabled: true}), nil
}

func (s *SickRockServer) ClearUIUsageLog(ctx context.Context, _ *connect.Request[sickrockpb.ClearUIUsageLogRequest]) (*connect.Response[sickrockpb.ClearUIUsageLogResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if s.uiUsage != nil {
		if err := s.uiUsage.Clear(userID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("clear UI usage log: %w", err))
		}
	}
	return connect.NewResponse(&sickrockpb.ClearUIUsageLogResponse{}), nil
}

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// UIUsageLogHandler serves GET /api/ui-usage.jsonl: the signed-in user's own log, as a download.
// It must be wrapped by the auth layer.
func (s *SickRockServer) UIUsageLogHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		au := armlayer.UserFromContext(r.Context())
		if au == nil || au.User == nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if s.uiUsage == nil {
			http.Error(w, "UI usage logging is not available on this server", http.StatusNotFound)
			return
		}
		b, err := s.uiUsage.Read(au.User.ID)
		if err != nil {
			log.WithError(err).Error("read UI usage log")
			http.Error(w, "could not read the log", http.StatusInternalServerError)
			return
		}
		name := unsafeFilenameChars.ReplaceAllString(au.User.Username, "_")
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="sickrock-ui-usage-%s.jsonl"`, name))
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(b)
		}
	})
}
