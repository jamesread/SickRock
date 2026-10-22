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
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) {
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

func (s *SickRockServer) denyAuditLogMutation(_ context.Context, tcName string) error {
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
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := s.repo.InsertAuditLog(auditCtx, entry); err != nil {
			log.WithError(err).WithField("event_type", entry.EventType).Warn("audit log insert failed")
		}
	}(e)
}

func (s *SickRockServer) auditActorFromContext(ctx context.Context) (userID int, username string) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return 0, ""
	}
	return au.User.ID, au.User.Username
}

func (s *SickRockServer) auditTableAccess(ctx context.Context, tcName string, action repo.ResourceGrantAction, rowID string, success bool, message string) {
	if tcName == "" || tcName == audit.TableConfiguration {
		return
	}
	actorID, actorName := s.auditActorFromContext(ctx)
	s.recordAudit(ctx, repo.AuditLogEntry{
		EventType:     tableAuditEventType(action, success),
		Message:       message,
		ActorUserID:   actorID,
		ActorUsername: actorName,
		RelatedTC:     tcName,
		RelatedRowID:  rowID,
		Success:       success,
	})
}

// tableAuditEventType returns the audit event for a table action: denied, or the action performed.
func tableAuditEventType(action repo.ResourceGrantAction, success bool) string {
	if !success {
		return audit.EventTableDenied
	}
	switch action {
	case repo.ResourceActionInsert:
		return audit.EventTableInsert
	case repo.ResourceActionEdit:
		return audit.EventTableEdit
	case repo.ResourceActionDelete:
		return audit.EventTableDelete
	default: // including view
		return audit.EventTableView
	}
}
