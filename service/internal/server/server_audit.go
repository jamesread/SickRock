package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam/systemroles"
	"github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

func (s *SickRockServer) requireAuditView(ctx context.Context) error {
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return nil
	}
	if au.RBAC != nil && au.RBAC.Has(systemroles.PermissionAuditView) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", systemroles.PermissionAuditView))
}

func (s *SickRockServer) requireAuditLogTableAccess(ctx context.Context, tcName string) error {
	if tcName != audit.TableConfiguration {
		return nil
	}
	return s.requireAuditView(ctx)
}

func (s *SickRockServer) denyAuditLogMutation(ctx context.Context, tcName string) error {
	if tcName != audit.TableConfiguration {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("audit logs are append-only"))
}

func (s *SickRockServer) recordAudit(ctx context.Context, e repo.AuditLogEntry) {
	if e.EventType == "" {
		return
	}
	go func(entry repo.AuditLogEntry) {
		auditCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.repo.InsertAuditLog(auditCtx, entry); err != nil {
			log.WithError(err).WithField("event_type", entry.EventType).Warn("audit log insert failed")
		}
	}(e)
}

func (s *SickRockServer) auditActorFromContext(ctx context.Context) (userID int, username string) {
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return 0, ""
	}
	return au.User.ID, au.User.Username
}

func (s *SickRockServer) auditTableAccess(ctx context.Context, tcName string, action repo.ResourceGrantAction, rowID string, success bool, message string) {
	if tcName == "" || tcName == audit.TableConfiguration {
		return
	}
	eventType := audit.EventTableDenied
	if success {
		switch action {
		case repo.ResourceActionView:
			eventType = audit.EventTableView
		case repo.ResourceActionInsert:
			eventType = audit.EventTableInsert
		case repo.ResourceActionEdit:
			eventType = audit.EventTableEdit
		case repo.ResourceActionDelete:
			eventType = audit.EventTableDelete
		default:
			eventType = audit.EventTableView
		}
	}
	actorID, actorName := s.auditActorFromContext(ctx)
	s.recordAudit(ctx, repo.AuditLogEntry{
		EventType:     eventType,
		Message:       message,
		ActorUserID:   actorID,
		ActorUsername: actorName,
		RelatedTC:     tcName,
		RelatedRowID:  rowID,
		Success:       success,
	})
}
