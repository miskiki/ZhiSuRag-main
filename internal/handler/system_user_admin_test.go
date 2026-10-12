package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// v2.19 账号管理 handler 单测。
// 锁定：裸 JSON 响应约定、审计动作、自我保护 400、错误映射
// （404 / 409 / 400）、userAdminSvc 缺失时的 503 降级。
// ============================================================================

// systemAdminUserSvcStub 是账号管理 handler 测试专用的窄接口 stub。嵌入
// 接口保持仓库惯例：未桩化的方法一旦被调用即显式崩溃。
type systemAdminUserSvcStub struct {
	interfaces.SystemUserAdminService

	listResp *types.AdminUserListResponse
	detail   *types.AdminUserDetail
	user     *types.User
	sessions []types.LoginSession
	oldRole  types.TenantRole
	newRole  types.TenantRole

	errList    error
	errDetail  error
	errUpdate  error
	errStatus  error
	errForce   error
	errSess    error
	errRevoke  error
	errRole    error
	errRemove  error

	// call records
	lastActorID  string
	lastTargetID string
	lastIsActive bool
	lastQuery    types.AdminUserListQuery
	lastReq      types.AdminUpdateUserRequest
	lastSession  string
	lastTenant   uint64
	lastRole     types.TenantRole
}

func (s *systemAdminUserSvcStub) AdminListUsers(
	_ context.Context, q types.AdminUserListQuery,
) (*types.AdminUserListResponse, error) {
	s.lastQuery = q
	return s.listResp, s.errList
}
func (s *systemAdminUserSvcStub) AdminGetUserDetail(
	_ context.Context, userID string,
) (*types.AdminUserDetail, error) {
	s.lastTargetID = userID
	return s.detail, s.errDetail
}
func (s *systemAdminUserSvcStub) AdminUpdateUserProfile(
	_ context.Context, actorID, userID string, req types.AdminUpdateUserRequest,
) (*types.User, error) {
	s.lastActorID, s.lastTargetID, s.lastReq = actorID, userID, req
	return s.user, s.errUpdate
}
func (s *systemAdminUserSvcStub) AdminSetUserStatus(
	_ context.Context, actorID, userID string, isActive bool,
) (*types.User, error) {
	s.lastActorID, s.lastTargetID, s.lastIsActive = actorID, userID, isActive
	return s.user, s.errStatus
}
func (s *systemAdminUserSvcStub) AdminForceLogout(
	_ context.Context, actorID, userID string,
) error {
	s.lastActorID, s.lastTargetID = actorID, userID
	return s.errForce
}
func (s *systemAdminUserSvcStub) AdminListUserSessions(
	_ context.Context, userID string,
) ([]types.LoginSession, error) {
	s.lastTargetID = userID
	return s.sessions, s.errSess
}
func (s *systemAdminUserSvcStub) AdminRevokeUserSession(
	_ context.Context, actorID, userID, sessionID string,
) error {
	s.lastActorID, s.lastTargetID, s.lastSession = actorID, userID, sessionID
	return s.errRevoke
}
func (s *systemAdminUserSvcStub) AdminUpdateMembershipRole(
	_ context.Context, actorID, userID string, tenantID uint64, role types.TenantRole,
) (types.TenantRole, types.TenantRole, error) {
	s.lastActorID, s.lastTargetID, s.lastTenant, s.lastRole = actorID, userID, tenantID, role
	return s.oldRole, s.newRole, s.errRole
}
func (s *systemAdminUserSvcStub) AdminRemoveMembership(
	_ context.Context, actorID, userID string, tenantID uint64,
) error {
	s.lastActorID, s.lastTargetID, s.lastTenant = actorID, userID, tenantID
	return s.errRemove
}

// adminTargetUserSvc supplies GetUserByID for the handlers that resolve the
// target account before delegating (force-logout / session revoke /
// membership ops).
type adminTargetUserSvc struct {
	interfaces.UserService
	target *types.User
	err    error
}

func (s *adminTargetUserSvc) GetUserByID(context.Context, string) (*types.User, error) {
	return s.target, s.err
}

func adminUserRouter(h *SystemHandler, actorID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, actorID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/system/admin/users", h.AdminListUsers)
	r.GET("/system/admin/users/:user_id", h.AdminGetUserDetail)
	r.PUT("/system/admin/users/:user_id", h.AdminUpdateUser)
	r.PUT("/system/admin/users/:user_id/status", h.AdminSetUserStatus)
	r.POST("/system/admin/users/:user_id/force-logout", h.AdminForceLogout)
	r.GET("/system/admin/users/:user_id/sessions", h.AdminListUserSessions)
	r.DELETE("/system/admin/users/:user_id/sessions/:session_id", h.AdminRevokeUserSession)
	r.PUT("/system/admin/users/:user_id/memberships/:tenant_id", h.AdminUpdateMembershipRole)
	r.DELETE("/system/admin/users/:user_id/memberships/:tenant_id", h.AdminRemoveMembership)
	return r
}

func doAdminUserReq(
	t *testing.T, r *gin.Engine, method, path string, body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newAdminUserHandler(svc *systemAdminUserSvcStub, target *types.User, audits *capturingAuditService) *SystemHandler {
	return &SystemHandler{
		userSvc:      &adminTargetUserSvc{target: target},
		userAdminSvc: svc,
		auditSvc:     audits,
	}
}

func TestAdminSetUserStatus_DisableAuditsAndReturnsUser(t *testing.T) {
	audits := &capturingAuditService{}
	svc := &systemAdminUserSvcStub{
		user: &types.User{ID: "target-1", Username: "alice", Email: "alice@example.com", IsActive: false},
	}
	h := newAdminUserHandler(svc, nil, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1/status",
		map[string]any{"is_active": false})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.lastActorID != "admin-1" || svc.lastTargetID != "target-1" || svc.lastIsActive {
		t.Fatalf("forwarded args wrong: actor=%s target=%s active=%v",
			svc.lastActorID, svc.lastTargetID, svc.lastIsActive)
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemUserStatusChanged {
		t.Fatalf("unexpected audit entries: %+v", audits.entries)
	}
}

func TestAdminSetUserStatus_SelfActionIs400(t *testing.T) {
	audits := &capturingAuditService{}
	svc := &systemAdminUserSvcStub{errStatus: service.ErrAdminSelfAction}
	h := newAdminUserHandler(svc, nil, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/admin-1/status",
		map[string]any{"is_active": false})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self disable must be 400, got %d", w.Code)
	}
	if len(audits.entries) != 0 {
		t.Fatal("self action must not audit")
	}
}

func TestAdminSetUserStatus_MissingIsActiveIs400(t *testing.T) {
	h := newAdminUserHandler(&systemAdminUserSvcStub{}, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1/status", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing is_active must be 400, got %d", w.Code)
	}
}

func TestAdminForceLogout_UnknownUserIs404(t *testing.T) {
	// Target lookup fails first — the handler turns that into a clean 404
	// before the service method is ever reached. The admin service must be
	// wired (non-nil) so the availability gate passes, and the repo's
	// canonical sentinel is what maps to 404.
	h := &SystemHandler{
		userSvc:      &adminTargetUserSvc{err: repository.ErrUserNotFound},
		userAdminSvc: &systemAdminUserSvcStub{},
	}
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPost, "/system/admin/users/ghost/force-logout", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown user must be 404, got %d", w.Code)
	}
}

func TestAdminForceLogout_SuccessAudits(t *testing.T) {
	audits := &capturingAuditService{}
	target := &types.User{ID: "target-1", Username: "alice", Email: "alice@example.com"}
	svc := &systemAdminUserSvcStub{}
	h := newAdminUserHandler(svc, target, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPost, "/system/admin/users/target-1/force-logout", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemUserForceLogout {
		t.Fatalf("unexpected audit: %+v", audits.entries)
	}
}

func TestAdminRevokeUserSession_AuditsSessionID(t *testing.T) {
	audits := &capturingAuditService{}
	target := &types.User{ID: "target-1", Username: "alice", Email: "alice@example.com"}
	svc := &systemAdminUserSvcStub{}
	h := newAdminUserHandler(svc, target, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodDelete, "/system/admin/users/target-1/sessions/s-abc", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.lastSession != "s-abc" {
		t.Fatalf("session id not forwarded: %q", svc.lastSession)
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemSessionRevoked {
		t.Fatalf("unexpected audit: %+v", audits.entries)
	}
}

func TestAdminUpdateUser_ConflictIs409(t *testing.T) {
	svc := &systemAdminUserSvcStub{errUpdate: service.ErrUserEmailExists}
	h := newAdminUserHandler(svc, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1",
		map[string]any{"email": "taken@example.com"})
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict must be 409, got %d", w.Code)
	}
}

func TestAdminUpdateUser_NoFieldsIs400(t *testing.T) {
	h := newAdminUserHandler(&systemAdminUserSvcStub{}, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty patch must be 400, got %d", w.Code)
	}
}

func TestAdminUpdateMembershipRole_AuditsOldAndNewRole(t *testing.T) {
	audits := &capturingAuditService{}
	target := &types.User{ID: "target-1", Username: "alice", Email: "alice@example.com"}
	svc := &systemAdminUserSvcStub{oldRole: types.TenantRoleContributor, newRole: types.TenantRoleAdmin}
	h := newAdminUserHandler(svc, target, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1/memberships/7",
		map[string]any{"role": "admin"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.lastTenant != 7 || svc.lastRole != types.TenantRoleAdmin {
		t.Fatalf("forwarded args wrong: tenant=%d role=%s", svc.lastTenant, svc.lastRole)
	}
	if len(audits.entries) != 1 ||
		audits.entries[0].Action != types.AuditActionSystemMembershipRoleChanged {
		t.Fatalf("unexpected audit: %+v", audits.entries)
	}
}

func TestAdminUpdateMembershipRole_LastOwnerIs400(t *testing.T) {
	svc := &systemAdminUserSvcStub{errRole: service.ErrLastOwner}
	h := newAdminUserHandler(svc, &types.User{ID: "target-1"}, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodPut, "/system/admin/users/target-1/memberships/7",
		map[string]any{"role": "admin"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("last owner must be 400, got %d", w.Code)
	}
}

func TestAdminRemoveMembership_Audits(t *testing.T) {
	audits := &capturingAuditService{}
	target := &types.User{ID: "target-1", Username: "alice", Email: "alice@example.com"}
	svc := &systemAdminUserSvcStub{}
	h := newAdminUserHandler(svc, target, audits)
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodDelete, "/system/admin/users/target-1/memberships/7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(audits.entries) != 1 ||
		audits.entries[0].Action != types.AuditActionSystemMembershipRemoved {
		t.Fatalf("unexpected audit: %+v", audits.entries)
	}
}

func TestAdminListUsers_ReturnsRawPagedPayload(t *testing.T) {
	svc := &systemAdminUserSvcStub{listResp: &types.AdminUserListResponse{
		Users: []*types.UserInfo{{ID: "target-1", Username: "alice"}},
		Total: 1, Page: 2, PageSize: 20,
	}}
	h := newAdminUserHandler(svc, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodGet,
		"/system/admin/users?search=ali&page=2&page_size=20&is_active=false", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.lastQuery.Search != "ali" || svc.lastQuery.Page != 2 || svc.lastQuery.PageSize != 20 {
		t.Fatalf("query not parsed: %+v", svc.lastQuery)
	}
	if svc.lastQuery.IsActive == nil || *svc.lastQuery.IsActive {
		t.Fatal("is_active filter not parsed")
	}
	var resp types.AdminUserListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("raw JSON contract broken: %v", err)
	}
	if resp.Total != 1 || len(resp.Users) != 1 {
		t.Fatalf("payload wrong: %+v", resp)
	}
}

func TestAdminListUserSessions_ReturnsRawArray(t *testing.T) {
	svc := &systemAdminUserSvcStub{sessions: []types.LoginSession{
		{SessionID: "s-1", DeviceLabel: "Chrome 126 · Windows", DeviceIP: "10.0.0.8"},
	}}
	h := newAdminUserHandler(svc, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodGet, "/system/admin/users/target-1/sessions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var sessions []types.LoginSession
	if err := json.Unmarshal(w.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("raw array contract broken: %v", err)
	}
	if len(sessions) != 1 || sessions[0].SessionID != "s-1" {
		t.Fatalf("payload wrong: %+v", sessions)
	}
}

func TestAdminGetUserDetail_ReturnsDetail(t *testing.T) {
	svc := &systemAdminUserSvcStub{detail: &types.AdminUserDetail{
		User:        &types.UserInfo{ID: "target-1"},
		Memberships: []types.AdminMembershipView{{TenantID: 7, TenantName: "Alpha", Role: types.TenantRoleAdmin}},
	}}
	h := newAdminUserHandler(svc, nil, &capturingAuditService{})
	r := adminUserRouter(h, "admin-1")

	w := doAdminUserReq(t, r, http.MethodGet, "/system/admin/users/target-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var detail types.AdminUserDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("detail contract broken: %v", err)
	}
	if len(detail.Memberships) != 1 || detail.Memberships[0].TenantName != "Alpha" {
		t.Fatalf("detail wrong: %+v", detail)
	}
}

func TestAdminUserHandlers_NilServiceDegradesTo503(t *testing.T) {
	// A partially-built handler (unit-test construction without the type
	// assertion in NewSystemHandler) must 503, never panic.
	h := &SystemHandler{}
	r := adminUserRouter(h, "admin-1")

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/system/admin/users", nil},
		{http.MethodGet, "/system/admin/users/target-1", nil},
		{http.MethodPut, "/system/admin/users/target-1", map[string]any{"email": "a@b.c"}},
		{http.MethodPut, "/system/admin/users/target-1/status", map[string]any{"is_active": true}},
		{http.MethodPost, "/system/admin/users/target-1/force-logout", nil},
		{http.MethodGet, "/system/admin/users/target-1/sessions", nil},
		{http.MethodDelete, "/system/admin/users/target-1/sessions/s-1", nil},
		{http.MethodPut, "/system/admin/users/target-1/memberships/7", map[string]any{"role": "admin"}},
		{http.MethodDelete, "/system/admin/users/target-1/memberships/7", nil},
	}
	for _, tc := range cases {
		w := doAdminUserReq(t, r, tc.method, tc.path, tc.body)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s must degrade to 503, got %d", tc.method, tc.path, w.Code)
		}
	}
}
