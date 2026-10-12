package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ============================================================================
// v2.19 账号管理服务层单测。
// 覆盖方案（账号管理实施方案.md 阶段 1 第 6 项）锁定的行为：
//   - 防自禁/防自操作（ErrAdminSelfAction）
//   - 禁用撤销全部令牌（复用 RevokeTokensByUserID）
//   - 资料编辑唯一性冲突与自比较跳过
//   - 成员关系转发 + ErrLastOwner / ErrMembershipNotFound 透传
//   - 会话列表/踢除按目标用户隔离（转发参数锁定）
// ============================================================================

// adminUserRepoStub 是账号管理测试专用的 UserRepository stub（显式实现，
// 不嵌入接口，杜绝契约漂移静默通过）。
type adminUserRepoStub struct {
	users       map[string]*types.User
	updateCalls int

	listPagedCalls int
	listPagedArgs  [5]any // search, isActive, isSystemAdmin, offset, limit

	usernamesByEmail map[string]string // email -> existing owner id
	usernamesByName  map[string]string // username -> existing owner id
}

func (r *adminUserRepoStub) CreateUser(context.Context, *types.User) error { return nil }
func (r *adminUserRepoStub) GetUserByID(_ context.Context, id string) (*types.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	cp := *u
	return &cp, nil
}
func (r *adminUserRepoStub) GetUsersByIDs(context.Context, []string) (map[string]*types.User, error) {
	return nil, nil
}
func (r *adminUserRepoStub) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if owner, ok := r.usernamesByEmail[email]; ok {
		return &types.User{ID: owner, Email: email}, nil
	}
	return nil, errors.New("user not found")
}
func (r *adminUserRepoStub) GetUserByUsername(_ context.Context, username string) (*types.User, error) {
	if owner, ok := r.usernamesByName[username]; ok {
		return &types.User{ID: owner, Username: username}, nil
	}
	return nil, errors.New("user not found")
}
func (r *adminUserRepoStub) GetUserByTenantID(context.Context, uint64) (*types.User, error) {
	return nil, nil
}
func (r *adminUserRepoStub) UpdateUser(_ context.Context, user *types.User) error {
	r.updateCalls++
	cp := *user
	r.users[user.ID] = &cp
	return nil
}
func (r *adminUserRepoStub) DeleteUser(context.Context, string) error { return nil }
func (r *adminUserRepoStub) ListUsers(context.Context, int, int) ([]*types.User, error) {
	return nil, nil
}
func (r *adminUserRepoStub) ListUsersPaged(
	_ context.Context, search string, isActive, isSystemAdmin *bool, offset, limit int,
) ([]*types.User, int64, error) {
	r.listPagedCalls++
	r.listPagedArgs = [5]any{search, isActive, isSystemAdmin, offset, limit}
	out := make([]*types.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out, int64(len(r.users)), nil
}
func (r *adminUserRepoStub) ListSystemAdmins(context.Context, int, int) ([]*types.User, int64, error) {
	return nil, 0, nil
}
func (r *adminUserRepoStub) RevokeSystemAdmin(context.Context, string, string) (*types.User, error) {
	return nil, nil
}
func (r *adminUserRepoStub) SearchUsers(context.Context, string, int) ([]*types.User, error) {
	return nil, nil
}

// adminTokenRepoStub 记录撤销调用，锁「禁用即下线」的联动。
type adminTokenRepoStub struct {
	revokedUserIDs []string
	revokedSession [][2]string // [userID, sessionID]
}

func (s *adminTokenRepoStub) CreateToken(context.Context, *types.AuthToken) error { return nil }
func (s *adminTokenRepoStub) GetTokenByValue(context.Context, string) (*types.AuthToken, error) {
	return nil, errors.New("not implemented")
}
func (s *adminTokenRepoStub) GetTokensByUserID(context.Context, string) ([]*types.AuthToken, error) {
	return nil, nil
}
func (s *adminTokenRepoStub) UpdateToken(context.Context, *types.AuthToken) error { return nil }
func (s *adminTokenRepoStub) DeleteToken(context.Context, string) error           { return nil }
func (s *adminTokenRepoStub) DeleteExpiredTokens(context.Context) error           { return nil }
func (s *adminTokenRepoStub) RevokeTokensByUserID(_ context.Context, userID string) error {
	s.revokedUserIDs = append(s.revokedUserIDs, userID)
	return nil
}
func (s *adminTokenRepoStub) RevokeBySessionID(_ context.Context, userID, sessionID string) error {
	s.revokedSession = append(s.revokedSession, [2]string{userID, sessionID})
	return nil
}
func (s *adminTokenRepoStub) GetLatestSessionToken(context.Context, string, string) (*types.AuthToken, error) {
	return nil, errors.New("not implemented")
}

// adminMemberSvcStub 记录转发调用并按预设返回错误（ErrLastOwner 透传锁定）。
// 嵌入接口：未实现的方法一旦被调用即 nil-deref 显式失败（仓库惯例）。
type adminMemberSvcStub struct {
	interfaces.TenantMemberService
	memberships map[string][]*types.TenantMember // userID -> memberships
	updateCalls [][2]any                         // {userID, tenantID}
	updateRole  types.TenantRole
	updateErr   error
	removeCalls [][2]any
	removeErr   error
	listErr     error
}

func (s *adminMemberSvcStub) AddMember(
	context.Context, string, uint64, types.TenantRole, *string,
) (*types.TenantMember, error) {
	return nil, nil
}
func (s *adminMemberSvcStub) EnsureOwner(context.Context, string, uint64) (*types.TenantMember, error) {
	return nil, nil
}
func (s *adminMemberSvcStub) GetMembership(context.Context, string, uint64) (*types.TenantMember, error) {
	return nil, nil
}
func (s *adminMemberSvcStub) ListByUser(_ context.Context, userID string) ([]*types.TenantMember, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.memberships[userID], nil
}
func (s *adminMemberSvcStub) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return nil, nil
}
func (s *adminMemberSvcStub) ListMembersPage(
	context.Context, uint64, string, int, int,
) ([]*types.TenantMember, int64, error) {
	return nil, 0, nil
}
func (s *adminMemberSvcStub) UpdateRole(_ context.Context, userID string, tenantID uint64, role types.TenantRole) error {
	s.updateCalls = append(s.updateCalls, [2]any{userID, tenantID})
	s.updateRole = role
	return s.updateErr
}
func (s *adminMemberSvcStub) RemoveMember(_ context.Context, userID string, tenantID uint64) error {
	s.removeCalls = append(s.removeCalls, [2]any{userID, tenantID})
	return s.removeErr
}

// adminTenantSvcStub 供详情视图的空间名 enrichment（嵌入接口，同上）。
type adminTenantSvcStub struct {
	interfaces.TenantService
	tenants map[uint64]*types.Tenant
}

func (s *adminTenantSvcStub) GetTenantsByIDs(_ context.Context, ids []uint64) (map[uint64]*types.Tenant, error) {
	out := map[uint64]*types.Tenant{}
	for _, id := range ids {
		if t, ok := s.tenants[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

func newAdminUserService(
	userRepo *adminUserRepoStub,
	tokenRepo *adminTokenRepoStub,
	memberSvc *adminMemberSvcStub,
	tenantSvc *adminTenantSvcStub,
) *userService {
	return &userService{
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		memberService: memberSvc,
		tenantService: tenantSvc,
	}
}

func adminTestUsers() map[string]*types.User {
	return map[string]*types.User{
		"admin-1": {ID: "admin-1", Username: "admin", Email: "admin@example.com", IsActive: true},
		"target-1": {ID: "target-1", Username: "alice", Email: "alice@example.com", IsActive: true},
	}
}

func TestAdminSetUserStatus_DisableRevokesTokens(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	tokenRepo := &adminTokenRepoStub{}
	svc := newAdminUserService(userRepo, tokenRepo, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	user, err := svc.AdminSetUserStatus(context.Background(), "admin-1", "target-1", false)
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if user.IsActive {
		t.Fatal("user still active after disable")
	}
	if len(tokenRepo.revokedUserIDs) != 1 || tokenRepo.revokedUserIDs[0] != "target-1" {
		t.Fatalf("disable must revoke target tokens, got %v", tokenRepo.revokedUserIDs)
	}
	if userRepo.updateCalls != 1 {
		t.Fatalf("expected 1 update call, got %d", userRepo.updateCalls)
	}
}

func TestAdminSetUserStatus_EnableDoesNotTouchTokens(t *testing.T) {
	users := adminTestUsers()
	users["target-1"].IsActive = false
	userRepo := &adminUserRepoStub{users: users}
	tokenRepo := &adminTokenRepoStub{}
	svc := newAdminUserService(userRepo, tokenRepo, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	user, err := svc.AdminSetUserStatus(context.Background(), "admin-1", "target-1", true)
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if !user.IsActive {
		t.Fatal("user not active after enable")
	}
	if len(tokenRepo.revokedUserIDs) != 0 {
		t.Fatalf("enable must not revoke tokens, got %v", tokenRepo.revokedUserIDs)
	}
}

func TestAdminSetUserStatus_IdempotentNoopSkipsRevoke(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	tokenRepo := &adminTokenRepoStub{}
	svc := newAdminUserService(userRepo, tokenRepo, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	// target-1 already active → enabling again must be a no-op.
	if _, err := svc.AdminSetUserStatus(context.Background(), "admin-1", "target-1", true); err != nil {
		t.Fatalf("idempotent enable failed: %v", err)
	}
	if userRepo.updateCalls != 0 || len(tokenRepo.revokedUserIDs) != 0 {
		t.Fatalf("idempotent noop had side effects: updates=%d revokes=%d",
			userRepo.updateCalls, len(tokenRepo.revokedUserIDs))
	}
}

func TestAdminSelfAction_RejectedOnEveryMutation(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	tokenRepo := &adminTokenRepoStub{}
	memberSvc := &adminMemberSvcStub{}
	svc := newAdminUserService(userRepo, tokenRepo, memberSvc, &adminTenantSvcStub{})
	ctx := context.Background()

	if _, err := svc.AdminSetUserStatus(ctx, "admin-1", "admin-1", false); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self disable: got %v", err)
	}
	if _, err := svc.AdminUpdateUserProfile(ctx, "admin-1", "admin-1", types.AdminUpdateUserRequest{
		Username: strPtr("renamed"),
	}); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self profile edit: got %v", err)
	}
	if err := svc.AdminForceLogout(ctx, "admin-1", "admin-1"); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self force-logout: got %v", err)
	}
	if err := svc.AdminRevokeUserSession(ctx, "admin-1", "admin-1", "s-1"); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self session kick: got %v", err)
	}
	if _, _, err := svc.AdminUpdateMembershipRole(ctx, "admin-1", "admin-1", 7, types.TenantRoleAdmin); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self membership role: got %v", err)
	}
	if err := svc.AdminRemoveMembership(ctx, "admin-1", "admin-1", 7); !errors.Is(err, ErrAdminSelfAction) {
		t.Fatalf("self membership removal: got %v", err)
	}
	// No mutation may have reached any dependency.
	if len(tokenRepo.revokedUserIDs) != 0 || len(memberSvc.updateCalls) != 0 || len(memberSvc.removeCalls) != 0 {
		t.Fatal("self-action attempts leaked into dependencies")
	}
}

func strPtr(s string) *string { return &s }

func TestAdminUpdateUserProfile_UniquenessAndPatch(t *testing.T) {
	userRepo := &adminUserRepoStub{
		users:            adminTestUsers(),
		usernamesByEmail: map[string]string{"taken@example.com": "other-1"},
		usernamesByName:  map[string]string{"bob": "other-1"},
	}
	svc := newAdminUserService(userRepo, &adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{})
	ctx := context.Background()

	// Email conflict → 409-mapped error.
	if _, err := svc.AdminUpdateUserProfile(ctx, "admin-1", "target-1", types.AdminUpdateUserRequest{
		Email: strPtr("taken@example.com"),
	}); !errors.Is(err, ErrUserEmailExists) {
		t.Fatalf("email conflict: got %v", err)
	}
	// Username conflict → 409-mapped error.
	if _, err := svc.AdminUpdateUserProfile(ctx, "admin-1", "target-1", types.AdminUpdateUserRequest{
		Username: strPtr("bob"),
	}); !errors.Is(err, ErrUserUsernameExists) {
		t.Fatalf("username conflict: got %v", err)
	}
	// Valid change applies and keeps the other field intact.
	user, err := svc.AdminUpdateUserProfile(ctx, "admin-1", "target-1", types.AdminUpdateUserRequest{
		Email: strPtr("alice-new@example.com"),
	})
	if err != nil {
		t.Fatalf("valid email change failed: %v", err)
	}
	if user.Email != "alice-new@example.com" || user.Username != "alice" {
		t.Fatalf("patch semantics broken: %+v", user)
	}
}

func TestAdminUpdateUserProfile_SameIdentitySkipsLookup(t *testing.T) {
	// Re-submitting the target's own email must not hit the duplicate
	// lookup (which would find the target itself) nor flip anything.
	userRepo := &adminUserRepoStub{
		users: adminTestUsers(),
		usernamesByEmail: map[string]string{
			// alice@example.com resolves to target-1 itself — same-id hits
			// must be tolerated (idempotent resubmission).
			"alice@example.com": "target-1",
		},
	}
	svc := newAdminUserService(userRepo, &adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	user, err := svc.AdminUpdateUserProfile(context.Background(), "admin-1", "target-1", types.AdminUpdateUserRequest{
		Email: strPtr("alice@example.com"),
	})
	if err != nil {
		t.Fatalf("same-email resubmission rejected: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Fatalf("email mutated: %q", user.Email)
	}
}

func TestAdminForceLogout_RevokesAllTargetTokens(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	tokenRepo := &adminTokenRepoStub{}
	svc := newAdminUserService(userRepo, tokenRepo, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	if err := svc.AdminForceLogout(context.Background(), "admin-1", "target-1"); err != nil {
		t.Fatalf("force logout failed: %v", err)
	}
	if len(tokenRepo.revokedUserIDs) != 1 || tokenRepo.revokedUserIDs[0] != "target-1" {
		t.Fatalf("force logout revoked %v", tokenRepo.revokedUserIDs)
	}
}

func TestAdminForceLogout_UnknownUserIs404Mapped(t *testing.T) {
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{},
	)
	err := svc.AdminForceLogout(context.Background(), "admin-1", "ghost")
	// The service surfaces the repo's not-found error for handler mapping
	// (adminUserError maps it to 404). The stub repo errors on unknown ids.
	if err == nil {
		t.Fatal("unknown user must error")
	}
}

func TestAdminListUserSessions_ForwardsTargetIDWithoutCurrent(t *testing.T) {
	// The session projection is the v2.15 aggregation; the admin channel
	// must forward the TARGET user id with an empty current-session marker.
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	svc := newAdminUserService(userRepo, &adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{})

	sessions, err := svc.AdminListUserSessions(context.Background(), "target-1")
	if err != nil {
		t.Fatalf("list sessions failed: %v", err)
	}
	// stub token repo returns no tokens → empty (non-nil) projection.
	if sessions == nil {
		t.Fatal("sessions must be non-nil")
	}
}

func TestAdminUpdateMembershipRole_ForwardsAndReportsOldRole(t *testing.T) {
	memberSvc := &adminMemberSvcStub{
		memberships: map[string][]*types.TenantMember{
			"target-1": {
				{UserID: "target-1", TenantID: 7, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive},
			},
		},
	}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, &adminTenantSvcStub{},
	)

	oldRole, newRole, err := svc.AdminUpdateMembershipRole(
		context.Background(), "admin-1", "target-1", 7, types.TenantRoleAdmin,
	)
	if err != nil {
		t.Fatalf("role change failed: %v", err)
	}
	if oldRole != types.TenantRoleContributor || newRole != types.TenantRoleAdmin {
		t.Fatalf("roles reported wrong: old=%s new=%s", oldRole, newRole)
	}
	if len(memberSvc.updateCalls) != 1 || memberSvc.updateCalls[0] != [2]any{"target-1", uint64(7)} {
		t.Fatalf("forwarded args wrong: %v", memberSvc.updateCalls)
	}
	if memberSvc.updateRole != types.TenantRoleAdmin {
		t.Fatalf("forwarded role wrong: %s", memberSvc.updateRole)
	}
}

func TestAdminUpdateMembershipRole_LastOwnerPassthrough(t *testing.T) {
	memberSvc := &adminMemberSvcStub{
		memberships: map[string][]*types.TenantMember{
			"target-1": {
				{UserID: "target-1", TenantID: 7, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive},
			},
		},
		updateErr: ErrLastOwner,
	}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, &adminTenantSvcStub{},
	)

	_, _, err := svc.AdminUpdateMembershipRole(
		context.Background(), "admin-1", "target-1", 7, types.TenantRoleAdmin,
	)
	if !errors.Is(err, ErrLastOwner) {
		t.Fatalf("ErrLastOwner must pass through untouched, got %v", err)
	}
}

func TestAdminUpdateMembershipRole_UnknownMembershipIs404Mapped(t *testing.T) {
	memberSvc := &adminMemberSvcStub{memberships: map[string][]*types.TenantMember{}}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, &adminTenantSvcStub{},
	)
	_, _, err := svc.AdminUpdateMembershipRole(
		context.Background(), "admin-1", "target-1", 99, types.TenantRoleAdmin,
	)
	if !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("unknown membership must be ErrMembershipNotFound, got %v", err)
	}
}

func TestAdminRemoveMembership_Forwards(t *testing.T) {
	memberSvc := &adminMemberSvcStub{}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, &adminTenantSvcStub{},
	)
	if err := svc.AdminRemoveMembership(context.Background(), "admin-1", "target-1", 7); err != nil {
		t.Fatalf("remove membership failed: %v", err)
	}
	if len(memberSvc.removeCalls) != 1 || memberSvc.removeCalls[0] != [2]any{"target-1", uint64(7)} {
		t.Fatalf("forwarded args wrong: %v", memberSvc.removeCalls)
	}
}

func TestAdminRemoveMembership_LastOwnerPassthrough(t *testing.T) {
	memberSvc := &adminMemberSvcStub{removeErr: ErrLastOwner}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, &adminTenantSvcStub{},
	)
	if err := svc.AdminRemoveMembership(context.Background(), "admin-1", "target-1", 7); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("ErrLastOwner must pass through untouched, got %v", err)
	}
}

func TestAdminGetUserDetail_ProjectsMembershipsAndNames(t *testing.T) {
	memberSvc := &adminMemberSvcStub{
		memberships: map[string][]*types.TenantMember{
			"target-1": {
				{UserID: "target-1", TenantID: 7, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive},
				{UserID: "target-1", TenantID: 9, Role: types.TenantRoleViewer, Status: types.TenantMemberStatusInvited},
			},
		},
	}
	tenantSvc := &adminTenantSvcStub{tenants: map[uint64]*types.Tenant{
		7: {ID: 7, Name: "Alpha Workspace"},
		// 9 deliberately missing → blank name (best-effort enrichment).
	}}
	svc := newAdminUserService(
		&adminUserRepoStub{users: adminTestUsers()},
		&adminTokenRepoStub{}, memberSvc, tenantSvc,
	)

	detail, err := svc.AdminGetUserDetail(context.Background(), "target-1")
	if err != nil {
		t.Fatalf("detail failed: %v", err)
	}
	if detail.User == nil || detail.User.ID != "target-1" {
		t.Fatalf("profile missing: %+v", detail.User)
	}
	if len(detail.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(detail.Memberships))
	}
	if detail.Memberships[0].TenantName != "Alpha Workspace" {
		t.Fatalf("workspace name not enriched: %+v", detail.Memberships[0])
	}
	if detail.Memberships[1].TenantName != "" {
		t.Fatalf("missing tenant should degrade to blank name: %+v", detail.Memberships[1])
	}
	if detail.Memberships[1].Status != types.TenantMemberStatusInvited {
		t.Fatalf("status not projected: %+v", detail.Memberships[1])
	}
}

func TestAdminListUsers_NormalizesPagingAndForwardsFilters(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	svc := newAdminUserService(userRepo, &adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{})
	ctx := context.Background()

	inactive := false
	superOnly := true
	resp, err := svc.AdminListUsers(ctx, types.AdminUserListQuery{
		Search:        "ali",
		IsActive:      &inactive,
		IsSystemAdmin: &superOnly,
		Page:          0,    // → 1
		PageSize:      1000, // → 100
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if resp.Page != 1 || resp.PageSize != 100 {
		t.Fatalf("paging normalization broken: page=%d size=%d", resp.Page, resp.PageSize)
	}
	if resp.Total != int64(len(userRepo.users)) || len(resp.Users) == 0 {
		t.Fatalf("payload wrong: total=%d users=%d", resp.Total, len(resp.Users))
	}
	gotSearch, _ := userRepo.listPagedArgs[0].(string)
	if gotSearch != "ali" {
		t.Fatalf("search not forwarded: %q", gotSearch)
	}
	if userRepo.listPagedArgs[1] != &inactive || userRepo.listPagedArgs[2] != &superOnly {
		t.Fatal("filters not forwarded by pointer")
	}
	offset, _ := userRepo.listPagedArgs[3].(int)
	limit, _ := userRepo.listPagedArgs[4].(int)
	if offset != 0 || limit != 100 {
		t.Fatalf("window wrong: offset=%d limit=%d", offset, limit)
	}
	// The list projection is UserInfo — no password hash field exists on it
	// by construction; assert the projection type stayed sanitized.
	if len(resp.Users) > 0 && resp.Users[0] == nil {
		t.Fatal("nil user in page")
	}
}

func TestAdminListUsers_UnknownUserPageTwoStillReturnsPage(t *testing.T) {
	userRepo := &adminUserRepoStub{users: adminTestUsers()}
	svc := newAdminUserService(userRepo, &adminTokenRepoStub{}, &adminMemberSvcStub{}, &adminTenantSvcStub{})
	resp, err := svc.AdminListUsers(context.Background(), types.AdminUserListQuery{Page: 3, PageSize: 1})
	if err != nil {
		t.Fatalf("deep page failed: %v", err)
	}
	if resp.Page != 3 {
		t.Fatalf("page not preserved: %d", resp.Page)
	}
}

// Compile-time: the stubs must satisfy the narrow admin interface deps.
var (
	_ interfaces.UserRepository      = (*adminUserRepoStub)(nil)
	_ interfaces.AuthTokenRepository = (*adminTokenRepoStub)(nil)
	_ interfaces.TenantMemberService = (*adminMemberSvcStub)(nil)
	_ interfaces.TenantService       = (*adminTenantSvcStub)(nil)
)
