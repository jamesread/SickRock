package server

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
)

func TestSessionIDFromRequest(t *testing.T) {
	newReq := func(headers map[string]string) connect.AnyRequest {
		req := connect.NewRequest(&sickrockpb.PingRequest{})
		for k, v := range headers {
			req.Header().Set(k, v)
		}
		return req
	}

	assert.Equal(t, "sess", sessionIDFromRequest(newReq(map[string]string{"Session-Token": "sess", "Authorization": "Bearer other"})))
	assert.Equal(t, "tok", sessionIDFromRequest(newReq(map[string]string{"Authorization": "Bearer tok"})))
	assert.Empty(t, sessionIDFromRequest(newReq(map[string]string{"Authorization": "Bearer sk_apikey"})), "API keys are not sessions")
	assert.Equal(t, "cookie", sessionIDFromRequest(newReq(map[string]string{"Cookie": "a=b; " + iam.CookieName + "=cookie"})))
	assert.Empty(t, sessionIDFromRequest(newReq(nil)))
}

func TestTableAuditEventType(t *testing.T) {
	assert.Equal(t, audit.EventTableDenied, tableAuditEventType(repo.ResourceActionEdit, false))
	assert.Equal(t, audit.EventTableView, tableAuditEventType(repo.ResourceActionView, true))
	assert.Equal(t, audit.EventTableInsert, tableAuditEventType(repo.ResourceActionInsert, true))
	assert.Equal(t, audit.EventTableEdit, tableAuditEventType(repo.ResourceActionEdit, true))
	assert.Equal(t, audit.EventTableDelete, tableAuditEventType(repo.ResourceActionDelete, true))
	assert.Equal(t, audit.EventTableView, tableAuditEventType(repo.ResourceActionStart, true), "unknown actions audit as view")
}

func TestCreateForeignKeyRequestProblem(t *testing.T) {
	valid := &sickrockpb.CreateForeignKeyRequest{
		TableName: "orders", ColumnName: "customer_id",
		ReferencedTable: "customers", ReferencedColumn: "id",
		OnDeleteAction: "CASCADE", OnUpdateAction: "NO ACTION",
	}
	assert.Empty(t, createForeignKeyRequestProblem(valid))

	missing := &sickrockpb.CreateForeignKeyRequest{TableName: "orders", OnDeleteAction: "CASCADE", OnUpdateAction: "CASCADE"}
	assert.Contains(t, createForeignKeyRequestProblem(missing), "required")

	badAction := &sickrockpb.CreateForeignKeyRequest{
		TableName: "orders", ColumnName: "customer_id",
		ReferencedTable: "customers", ReferencedColumn: "id",
		OnDeleteAction: "DROP", OnUpdateAction: "CASCADE",
	}
	assert.Contains(t, createForeignKeyRequestProblem(badAction), "Invalid action")
}

func TestPasswordProblems(t *testing.T) {
	assert.NotEmpty(t, passwordResetProblem("", "longenough"))
	assert.NotEmpty(t, passwordResetProblem("bob", "short"))
	assert.Empty(t, passwordResetProblem("bob", "longenough"))

	assert.NotEmpty(t, newPasswordProblem("", "longenough"))
	assert.NotEmpty(t, newPasswordProblem("current", "short"))
	assert.Empty(t, newPasswordProblem("current", "longenough"))
}

func TestColumnSemanticForType(t *testing.T) {
	assert.Equal(t, repo.SemanticTypeUserRef, columnSemanticForType(repo.SemanticTypeUserRef))
	assert.Equal(t, repo.SemanticTypeDatetime, columnSemanticForType("datetime"))
	assert.Empty(t, columnSemanticForType("string"))
}

func TestRoleIDsForNames(t *testing.T) {
	assert.Equal(t, []int32{2, 1}, roleIDsForNames([]string{"member", "admin", "gone"}, map[string]int{"admin": 1, "member": 2}))
}

func TestJoinNonEmpty(t *testing.T) {
	assert.Equal(t, "a", joinNonEmpty("a", ""))
	assert.Equal(t, "a\n\nb", joinNonEmpty("a", "b"))
}
