package server

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/iam/rolegrants"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/armature-iam/rbac"
	armstore "github.com/jamesread/armature-iam/store"
)

func iamUserRow(u *armstore.UserAccountRow) *sickrockpb.IamUser {
	if u == nil {
		return nil
	}
	return &sickrockpb.IamUser{
		Id:        toInt32(u.ID),
		Username:  u.Username,
		CreatedBy: u.CreatedBy,
	}
}

func rbacRoleRow(r *armstore.RBACRoleRow) *sickrockpb.RbacRole {
	if r == nil {
		return nil
	}
	permIDs := make([]int32, 0, len(r.PermissionIDs))
	for _, id := range r.PermissionIDs {
		permIDs = append(permIDs, toInt32(id))
	}
	return &sickrockpb.RbacRole{
		Id:            toInt32(r.ID),
		Name:          r.Name,
		Description:   r.Description,
		PermissionIds: permIDs,
	}
}

func (s *SickRockServer) ListUsers(ctx context.Context, _ *connect.Request[sickrockpb.ListUsersRequest]) (*connect.Response[sickrockpb.ListUsersResponse], error) {
	rows, err := s.auth.Store.ListUserAccounts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list users: %w", err))
	}
	out := make([]*sickrockpb.IamUser, 0, len(rows))
	for i := range rows {
		out = append(out, iamUserRow(&rows[i]))
	}
	return connect.NewResponse(&sickrockpb.ListUsersResponse{Users: out}), nil
}

func (s *SickRockServer) GetUser(ctx context.Context, req *connect.Request[sickrockpb.GetUserRequest]) (*connect.Response[sickrockpb.GetUserResponse], error) {
	userID := int(req.Msg.GetUserId())
	if userID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}
	user, err := s.auth.Store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}
	return connect.NewResponse(&sickrockpb.GetUserResponse{User: iamUserRow(user)}), nil
}

func (s *SickRockServer) CreateUser(ctx context.Context, req *connect.Request[sickrockpb.CreateUserRequest]) (*connect.Response[sickrockpb.CreateUserResponse], error) {
	username := strings.TrimSpace(req.Msg.GetUsername())
	password := req.Msg.GetPassword()
	if err := s.validateNewUser(ctx, username, password); err != nil {
		return nil, err
	}
	hash, err := hashOptionalPassword(password)
	if err != nil {
		return nil, err
	}
	id, err := s.auth.Store.CreateUserAccount(ctx, username, hash, armstore.UserCreatedByAdmin)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create user: %w", err))
	}
	if err = s.auth.Store.EnsureUserInEveryoneGroup(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("ensure everyone group: %w", err))
	}
	user, err := s.auth.Store.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load created user"))
	}
	return connect.NewResponse(&sickrockpb.CreateUserResponse{User: iamUserRow(user)}), nil
}

// validateNewUser checks the username is set and free, and an optional password is long enough.
func (s *SickRockServer) validateNewUser(ctx context.Context, username, password string) error {
	if username == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("username is required"))
	}
	if password != "" && len(password) < 8 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password must be at least 8 characters"))
	}
	if existing, _ := s.auth.Store.GetUserByUsername(ctx, username); existing != nil {
		return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("username already taken"))
	}
	return nil
}

// hashOptionalPassword hashes password, or returns "" (no local password) when it is empty.
func hashOptionalPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := iam.HashPassword(password)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("hash password: %w", err))
	}
	return hash, nil
}

func (s *SickRockServer) DeleteUser(ctx context.Context, req *connect.Request[sickrockpb.DeleteUserRequest]) (*connect.Response[sickrockpb.DeleteUserResponse], error) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	targetID := int(req.Msg.GetUserId())
	if targetID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}
	if targetID == au.User.ID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot delete your own account"))
	}
	if err := s.auth.Store.DeleteUserAccount(ctx, targetID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete user: %w", err))
	}
	return connect.NewResponse(&sickrockpb.DeleteUserResponse{Success: true}), nil
}

func (s *SickRockServer) ChangePassword(ctx context.Context, req *connect.Request[sickrockpb.ChangePasswordRequest]) (*connect.Response[sickrockpb.ChangePasswordResponse], error) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	current := req.Msg.GetCurrentPassword()
	newPassword := req.Msg.GetNewPassword()
	failure, err := s.checkPasswordChange(ctx, au.User.ID, current, newPassword)
	if err != nil {
		return nil, err
	}
	if failure != "" {
		return connect.NewResponse(&sickrockpb.ChangePasswordResponse{Success: false, Message: failure}), nil
	}
	hash, err := iam.HashPassword(newPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("hash password: %w", err))
	}
	if err := s.auth.Store.UpdateUserPassword(ctx, au.User.ID, hash); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update password: %w", err))
	}
	return connect.NewResponse(&sickrockpb.ChangePasswordResponse{Success: true, Message: "password updated"}), nil
}

// checkPasswordChange validates a password change, returning a user-facing failure message when the
// passwords are missing, the new one is too short, or the current one is wrong.
func (s *SickRockServer) checkPasswordChange(ctx context.Context, userID int, current, newPassword string) (string, error) {
	if problem := newPasswordProblem(current, newPassword); problem != "" {
		return problem, nil
	}
	user, err := s.auth.Store.GetUserByID(ctx, userID)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		return "user not found", nil
	}
	ok, err := iam.VerifyPassword(user.PasswordHash, current)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return "current password is incorrect", nil
	}
	return "", nil
}

// newPasswordProblem returns why the submitted passwords are unusable, or "".
func newPasswordProblem(current, newPassword string) string {
	if current == "" || newPassword == "" {
		return "current and new password are required"
	}
	if len(newPassword) < 8 {
		return "new password must be at least 8 characters"
	}
	return ""
}

func (s *SickRockServer) ListRbacPermissions(ctx context.Context, _ *connect.Request[sickrockpb.ListRbacPermissionsRequest]) (*connect.Response[sickrockpb.ListRbacPermissionsResponse], error) {
	perms, err := s.auth.Store.ListRBACPermissions(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list permissions: %w", err))
	}
	out := make([]*sickrockpb.RbacPermission, 0, len(perms))
	for _, p := range perms {
		out = append(out, &sickrockpb.RbacPermission{
			Id:          toInt32(p.ID),
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return connect.NewResponse(&sickrockpb.ListRbacPermissionsResponse{Permissions: out}), nil
}

func (s *SickRockServer) ListRbacRoles(ctx context.Context, _ *connect.Request[sickrockpb.ListRbacRolesRequest]) (*connect.Response[sickrockpb.ListRbacRolesResponse], error) {
	roles, err := s.auth.Store.ListRBACRoles(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list roles: %w", err))
	}
	out := make([]*sickrockpb.RbacRole, 0, len(roles))
	for i := range roles {
		out = append(out, rbacRoleRow(&roles[i]))
	}
	return connect.NewResponse(&sickrockpb.ListRbacRolesResponse{Roles: out}), nil
}

func (s *SickRockServer) CreateRbacRole(ctx context.Context, req *connect.Request[sickrockpb.CreateRbacRoleRequest]) (*connect.Response[sickrockpb.CreateRbacRoleResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}
	if iam.IsSystemRole(name) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reserved role name"))
	}
	permIDs := int32SliceToInt(req.Msg.GetPermissionIds())
	id, err := s.auth.Store.CreateRBACRole(ctx, name, strings.TrimSpace(req.Msg.GetDescription()), permIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("create role: %w", err))
	}
	role, err := s.auth.Store.GetRBACRole(ctx, id)
	if err != nil || role == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load created role"))
	}
	return connect.NewResponse(&sickrockpb.CreateRbacRoleResponse{Role: rbacRoleRow(role)}), nil
}

func (s *SickRockServer) UpdateRbacRole(ctx context.Context, req *connect.Request[sickrockpb.UpdateRbacRoleRequest]) (*connect.Response[sickrockpb.UpdateRbacRoleResponse], error) {
	roleID := int(req.Msg.GetRoleId())
	if roleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role_id is required"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}
	if err := s.requireEditableRBACRole(ctx, roleID, name); err != nil {
		return nil, err
	}
	permIDs := int32SliceToInt(req.Msg.GetPermissionIds())
	if err := s.auth.Store.UpdateRBACRole(ctx, roleID, name, strings.TrimSpace(req.Msg.GetDescription()), permIDs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update role: %w", err))
	}
	role, err := s.auth.Store.GetRBACRole(ctx, roleID)
	if err != nil || role == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load updated role"))
	}
	return connect.NewResponse(&sickrockpb.UpdateRbacRoleResponse{Role: rbacRoleRow(role)}), nil
}

// requireEditableRBACRole checks the role exists, is not the superuser role, and is not being renamed
// to a reserved system role name.
func (s *SickRockServer) requireEditableRBACRole(ctx context.Context, roleID int, newName string) error {
	existing, err := s.auth.Store.GetRBACRole(ctx, roleID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("get role: %w", err))
	}
	if existing == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("role not found"))
	}
	if existing.Name == rbac.RoleSuperuser {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot modify system role %s", rbac.RoleSuperuser))
	}
	if iam.IsSystemRole(newName) && newName != existing.Name {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reserved role name"))
	}
	return nil
}

func (s *SickRockServer) DeleteRbacRole(ctx context.Context, req *connect.Request[sickrockpb.DeleteRbacRoleRequest]) (*connect.Response[sickrockpb.DeleteRbacRoleResponse], error) {
	roleID := int(req.Msg.GetRoleId())
	if roleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role_id is required"))
	}
	if err := s.auth.Store.DeleteRBACRole(ctx, roleID); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("delete role: %w", err))
	}
	return connect.NewResponse(&sickrockpb.DeleteRbacRoleResponse{Success: true}), nil
}

func (s *SickRockServer) GetUserRbacRoles(ctx context.Context, req *connect.Request[sickrockpb.GetUserRbacRolesRequest]) (*connect.Response[sickrockpb.GetUserRbacRolesResponse], error) {
	userID := int(req.Msg.GetUserId())
	if userID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}
	if user, _ := s.auth.Store.GetUserByID(ctx, userID); user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}
	roleNames, err := s.auth.Store.GetUserRbacRoleNames(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get user roles: %w", err))
	}
	allRoles, err := s.auth.Store.ListRBACRoles(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list roles: %w", err))
	}
	nameToID := map[string]int{}
	for _, r := range allRoles {
		nameToID[r.Name] = r.ID
	}
	return connect.NewResponse(&sickrockpb.GetUserRbacRolesResponse{RoleIds: roleIDsForNames(roleNames, nameToID)}), nil
}

// roleIDsForNames maps role names to ids, skipping names with no role.
func roleIDsForNames(roleNames []string, nameToID map[string]int) []int32 {
	ids := make([]int32, 0, len(roleNames))
	for _, n := range roleNames {
		if id, ok := nameToID[n]; ok {
			ids = append(ids, toInt32(id))
		}
	}
	return ids
}

func (s *SickRockServer) GetUserEffectiveRoleGrants(ctx context.Context, req *connect.Request[sickrockpb.GetUserEffectiveRoleGrantsRequest]) (*connect.Response[sickrockpb.GetUserEffectiveRoleGrantsResponse], error) {
	userID := int(req.Msg.GetUserId())
	if userID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}
	if user, _ := s.auth.Store.GetUserByID(ctx, userID); user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}
	db := s.repo.DB()
	isSuper, err := rolegrants.UserIsSuperuser(ctx, db, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("superuser check: %w", err))
	}
	rows, err := rolegrants.ListForUser(ctx, db, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list role grants: %w", err))
	}
	grants := make([]*sickrockpb.UserEffectiveRoleGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, &sickrockpb.UserEffectiveRoleGrant{
			RoleId:    toInt32(row.RoleID),
			RoleName:  row.RoleName,
			GroupId:   toInt32(row.GroupID),
			GroupName: row.GroupName,
		})
	}
	return connect.NewResponse(&sickrockpb.GetUserEffectiveRoleGrantsResponse{
		IsSuperuser: isSuper,
		Grants:      grants,
	}), nil
}

func (s *SickRockServer) GetUserGroupRbacRoles(ctx context.Context, req *connect.Request[sickrockpb.GetUserGroupRbacRolesRequest]) (*connect.Response[sickrockpb.GetUserGroupRbacRolesResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	ids, err := s.auth.Store.GetUserGroupRbacRoleIDs(ctx, groupID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get group roles: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetUserGroupRbacRolesResponse{RoleIds: intSliceToInt32(ids)}), nil
}

func (s *SickRockServer) SetUserGroupRbacRoles(ctx context.Context, req *connect.Request[sickrockpb.SetUserGroupRbacRolesRequest]) (*connect.Response[sickrockpb.SetUserGroupRbacRolesResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	if err := s.auth.Store.SetUserGroupRbacRoles(ctx, groupID, int32SliceToInt(req.Msg.GetRoleIds())); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("set group roles: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetUserGroupRbacRolesResponse{Success: true}), nil
}

func (s *SickRockServer) ListGroupTableRoleGrants(ctx context.Context, req *connect.Request[sickrockpb.ListGroupTableRoleGrantsRequest]) (*connect.Response[sickrockpb.ListGroupTableRoleGrantsResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	rows, err := s.repo.ListGroupTableRoleGrants(ctx, groupID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list table role grants: %w", err))
	}
	return connect.NewResponse(&sickrockpb.ListGroupTableRoleGrantsResponse{Grants: groupTableRoleGrantsToProto(rows)}), nil
}

func groupTableRoleGrantsToProto(rows []repo.GroupTableRoleGrant) []*sickrockpb.GroupTableRoleGrant {
	out := make([]*sickrockpb.GroupTableRoleGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, &sickrockpb.GroupTableRoleGrant{
			TableKey: row.TableKey,
			RoleId:   toInt32(row.RoleID),
		})
	}
	return out
}

func (s *SickRockServer) SetGroupTableRoleGrants(ctx context.Context, req *connect.Request[sickrockpb.SetGroupTableRoleGrantsRequest]) (*connect.Response[sickrockpb.SetGroupTableRoleGrantsResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	grants := make([]repo.GroupTableRoleGrant, 0, len(req.Msg.GetGrants()))
	for _, g := range req.Msg.GetGrants() {
		if g == nil {
			continue
		}
		tableKey := strings.TrimSpace(g.GetTableKey())
		if tableKey == audit.TableConfiguration {
			continue
		}
		grants = append(grants, repo.GroupTableRoleGrant{
			TableKey: tableKey,
			RoleID:   int(g.GetRoleId()),
		})
	}
	if err := s.repo.SetGroupTableRoleGrants(ctx, groupID, grants); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("set table role grants: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetGroupTableRoleGrantsResponse{Success: true}), nil
}

func (s *SickRockServer) ListTableShareGrants(ctx context.Context, req *connect.Request[sickrockpb.ListTableShareGrantsRequest]) (*connect.Response[sickrockpb.ListTableShareGrantsResponse], error) {
	tableKey := strings.TrimSpace(req.Msg.GetTableKey())
	if tableKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_key is required"))
	}
	if _, err := s.repo.GetTableConfiguration(ctx, tableKey); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("table not found"))
	}
	if tableKey == audit.TableConfiguration {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("audit logs use audit.view, not table share grants"))
	}
	rows, err := s.repo.ListTableShareGrants(ctx, tableKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list table share grants: %w", err))
	}
	out := make([]*sickrockpb.TableShareGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, &sickrockpb.TableShareGrant{
			GroupId:   toInt32(row.GroupID),
			GroupName: row.GroupName,
			RoleId:    toInt32(row.RoleID),
		})
	}
	return connect.NewResponse(&sickrockpb.ListTableShareGrantsResponse{Grants: out}), nil
}

func (s *SickRockServer) SetTableShareGrants(ctx context.Context, req *connect.Request[sickrockpb.SetTableShareGrantsRequest]) (*connect.Response[sickrockpb.SetTableShareGrantsResponse], error) {
	tableKey := strings.TrimSpace(req.Msg.GetTableKey())
	if tableKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_key is required"))
	}
	if tableKey == audit.TableConfiguration {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("audit logs use audit.view, not table share grants"))
	}
	if _, err := s.repo.GetTableConfiguration(ctx, tableKey); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("table not found"))
	}
	grants := make([]repo.TableShareGrant, 0, len(req.Msg.GetGrants()))
	for _, g := range req.Msg.GetGrants() {
		if g == nil {
			continue
		}
		grants = append(grants, repo.TableShareGrant{
			GroupID: int(g.GetGroupId()),
			RoleID:  int(g.GetRoleId()),
		})
	}
	if err := s.repo.SetTableShareGrants(ctx, tableKey, grants); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("set table share grants: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetTableShareGrantsResponse{Success: true}), nil
}

func (s *SickRockServer) GetRbacRoleUsers(ctx context.Context, req *connect.Request[sickrockpb.GetRbacRoleUsersRequest]) (*connect.Response[sickrockpb.GetRbacRoleUsersResponse], error) {
	roleID := int(req.Msg.GetRoleId())
	if roleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role_id is required"))
	}
	if role, _ := s.auth.Store.GetRBACRole(ctx, roleID); role == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("role not found"))
	}
	names, err := s.auth.Store.ListRbacRoleUsernames(ctx, roleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list role users: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetRbacRoleUsersResponse{Usernames: names}), nil
}

func (s *SickRockServer) GetRbacRoleGroups(ctx context.Context, req *connect.Request[sickrockpb.GetRbacRoleGroupsRequest]) (*connect.Response[sickrockpb.GetRbacRoleGroupsResponse], error) {
	roleID := int(req.Msg.GetRoleId())
	if roleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role_id is required"))
	}
	if role, _ := s.auth.Store.GetRBACRole(ctx, roleID); role == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("role not found"))
	}
	names, err := s.auth.Store.ListRbacRoleGroupNames(ctx, roleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list role groups: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetRbacRoleGroupsResponse{GroupNames: names}), nil
}

func (s *SickRockServer) GetMyPermissionsAudit(ctx context.Context, _ *connect.Request[sickrockpb.GetMyPermissionsAuditRequest]) (*connect.Response[sickrockpb.GetMyPermissionsAuditResponse], error) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	groupNames, roleNames, isSuper, rows, err := s.auth.Store.GetMyPermissionsAudit(ctx, au.User.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("permissions audit: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetMyPermissionsAuditResponse{
		GroupNames:  groupNames,
		RoleNames:   roleNames,
		IsSuperuser: isSuper,
		Permissions: permissionAuditRowsToProto(rows),
	}), nil
}

func permissionAuditRowsToProto(rows []armstore.MyPermissionAuditRow) []*sickrockpb.MyPermissionAuditRow {
	outRows := make([]*sickrockpb.MyPermissionAuditRow, 0, len(rows))
	for _, r := range rows {
		outRows = append(outRows, &sickrockpb.MyPermissionAuditRow{
			Permission:     r.Permission,
			Granted:        r.Granted,
			GrantingGroups: r.GrantingGroups,
		})
	}
	return outRows
}

func (s *SickRockServer) ListUserGroups(ctx context.Context, _ *connect.Request[sickrockpb.ListUserGroupsRequest]) (*connect.Response[sickrockpb.ListUserGroupsResponse], error) {
	groups, err := s.auth.Store.ListUserGroups(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list groups: %w", err))
	}
	out := make([]*sickrockpb.UserGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, &sickrockpb.UserGroup{
			Id:          toInt32(g.ID),
			Name:        g.Name,
			MemberCount: toInt32(g.MemberCount),
		})
	}
	return connect.NewResponse(&sickrockpb.ListUserGroupsResponse{Groups: out}), nil
}

func (s *SickRockServer) CreateUserGroup(ctx context.Context, req *connect.Request[sickrockpb.CreateUserGroupRequest]) (*connect.Response[sickrockpb.CreateUserGroupResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}
	if rbac.IsSystemGroup(name) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reserved group name"))
	}
	id, err := s.auth.Store.CreateUserGroup(ctx, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("create group: %w", err))
	}
	g, err := s.auth.Store.GetUserGroupByID(ctx, id)
	if err != nil || g == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load created group"))
	}
	return connect.NewResponse(&sickrockpb.CreateUserGroupResponse{
		Group: &sickrockpb.UserGroup{
			Id:          toInt32(g.ID),
			Name:        g.Name,
			MemberCount: toInt32(g.MemberCount),
		},
	}), nil
}

func (s *SickRockServer) rejectSystemGroupDelete(ctx context.Context, groupID int) error {
	g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID)
	if g != nil && rbac.IsSystemGroup(g.Name) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot delete system group"))
	}
	return nil
}

func (s *SickRockServer) DeleteUserGroup(ctx context.Context, req *connect.Request[sickrockpb.DeleteUserGroupRequest]) (*connect.Response[sickrockpb.DeleteUserGroupResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if err := s.rejectSystemGroupDelete(ctx, groupID); err != nil {
		return nil, err
	}
	if err := s.auth.Store.DeleteUserGroup(ctx, groupID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete group: %w", err))
	}
	return connect.NewResponse(&sickrockpb.DeleteUserGroupResponse{Success: true}), nil
}

func (s *SickRockServer) GetUserGroupMembers(ctx context.Context, req *connect.Request[sickrockpb.GetUserGroupMembersRequest]) (*connect.Response[sickrockpb.GetUserGroupMembersResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	ids, err := s.auth.Store.ListUserGroupMemberIDs(ctx, groupID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list members: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetUserGroupMembersResponse{UserIds: intSliceToInt32(ids)}), nil
}

func (s *SickRockServer) SetUserGroupMembers(ctx context.Context, req *connect.Request[sickrockpb.SetUserGroupMembersRequest]) (*connect.Response[sickrockpb.SetUserGroupMembersResponse], error) {
	groupID := int(req.Msg.GetGroupId())
	if groupID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("group_id is required"))
	}
	if g, _ := s.auth.Store.GetUserGroupByID(ctx, groupID); g == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}
	if err := s.auth.Store.SetUserGroupMembers(ctx, groupID, int32SliceToInt(req.Msg.GetUserIds())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("set members: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetUserGroupMembersResponse{Success: true}), nil
}

func int32SliceToInt(in []int32) []int {
	out := make([]int, 0, len(in))
	for _, v := range in {
		out = append(out, int(v))
	}
	return out
}

func intSliceToInt32(in []int) []int32 {
	out := make([]int32, 0, len(in))
	for _, v := range in {
		out = append(out, toInt32(v))
	}
	return out
}
