package repo

import (
	"context"
	"encoding/json"
	"fmt"
)

type AuditLogEntry struct {
	EventType      string
	Message        string
	ActorUserID    int
	ActorUsername  string
	RelatedUser    string
	RelatedTC      string
	RelatedRowID   string
	IPAddress      string
	UserAgent      string
	Success        bool
	Details        map[string]string
}

func (r *Repository) InsertAuditLog(ctx context.Context, e AuditLogEntry) error {
	if e.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	detailsJSON := "{}"
	if len(e.Details) > 0 {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return err
		}
		detailsJSON = string(b)
	}
	success := 0
	if e.Success {
		success = 1
	}
	var actorID any
	if e.ActorUserID > 0 {
		actorID = e.ActorUserID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (
			event_type, message, actor_user_id, actor_username,
			related_user, related_tc, related_row_id,
			ip_address, user_agent, success, details_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.EventType,
		e.Message,
		actorID,
		e.ActorUsername,
		e.RelatedUser,
		e.RelatedTC,
		e.RelatedRowID,
		e.IPAddress,
		e.UserAgent,
		success,
		detailsJSON,
	)
	return err
}
