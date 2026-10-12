package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// v2.19 账号管理（SystemAdmin）
// ----------------------------------------------------------------------------
// 平台级账号治理端点，全部挂 /api/v1/system/admin/users*（组级
// g.SystemAdmin() 守卫继承，见 routes_auth_tenant.go）。响应为裸 JSON
//（无 {code,msg,data} 包装），对齐 /system/admin 家族约定；审计统一走
// emitAdminAudit（TenantID=0 system-scope 行）。
//
// 自我保护：所有写操作在服务层拒绝 target==caller（ErrAdminSelfAction →
// 400），操作者对自己账号的变更走个人面板（资料/登录设备/空间成员）。
// 成员关系操作的 rbac.member_* 审计由 TenantMemberService 内部落库，
// 这里补发 system.membership_* 平台侧行以溯源管理通道。
// ============================================================================

// adminUserSvc returns the narrow account-management service view. The
// dig graph wires *userService, which implements both interfaces; direct
// struct construction (unit tests) may leave it nil — handlers then
// degrade to 503 instead of panicking.
func (h *SystemHandler) adminUserSvc() (interfaces.SystemUserAdminService, bool) {
	if h.userAdminSvc == nil {
		return nil, false
	}
	return h.userAdminSvc, true
}

// adminUserError is the concrete error mapper used by every handler below.
func (h *SystemHandler) adminUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAdminSelfAction),
		errors.Is(err, service.ErrLastOwner),
		errors.Is(err, service.ErrInvalidTenantRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
	case errors.Is(err, service.ErrMembershipNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUserEmailExists),
		errors.Is(err, service.ErrUserUsernameExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Account management operation failed"})
	}
}

// AdminListUsers godoc
// @Summary      平台用户列表（账号管理）
// @Description  分页返回全部账号，支持关键字（用户名/邮箱，不区分大小写）与 is_active / is_system_admin 过滤
// @Tags         System Admin
// @Produce      json
// @Param        search          query  string  false  "关键字（匹配用户名或邮箱）"
// @Param        page            query  int     false  "页码（默认 1）"
// @Param        page_size       query  int     false  "每页条数（默认 20，上限 100）"
// @Param        is_active       query  bool    false  "按启用状态过滤"
// @Param        is_system_admin query  bool    false  "按系统管理员过滤"
// @Success      200  {object}  types.AdminUserListResponse
// @Failure      403  {object}  map[string]interface{}  "Forbidden: not a system admin"
// @Router       /system/admin/users [get]
func (h *SystemHandler) AdminListUsers(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	query := types.AdminUserListQuery{
		Search:   strings.TrimSpace(c.Query("search")),
		Page:     atoiDefault(c.Query("page"), 1),
		PageSize: atoiDefault(c.Query("page_size"), 20),
	}
	if v := c.Query("is_active"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid is_active value"})
			return
		}
		query.IsActive = &b
	}
	if v := c.Query("is_system_admin"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid is_system_admin value"})
			return
		}
		query.IsSystemAdmin = &b
	}

	resp, err := svc.AdminListUsers(ctx, query)
	if err != nil {
		logger.Errorf(ctx, "Admin list users failed: %v", err)
		h.adminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AdminGetUserDetail godoc
// @Summary      账号详情（含跨空间成员关系）
// @Description  返回账号资料与其全部空间成员关系（角色/状态/空间名）
// @Tags         System Admin
// @Produce      json
// @Param        user_id  path  string  true  "用户 ID"
// @Success      200  {object}  types.AdminUserDetail
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Router       /system/admin/users/{user_id} [get]
func (h *SystemHandler) AdminGetUserDetail(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	detail, err := svc.AdminGetUserDetail(ctx, c.Param("user_id"))
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// AdminUpdateUser godoc
// @Summary      编辑账号资料（昵称/邮箱）
// @Description  PATCH 语义：仅提交的字段变更。邮箱/用户名冲突返回 409；不能编辑自己（走个人资料页）
// @Tags         System Admin
// @Accept       json
// @Produce      json
// @Param        user_id  path  string  true  "用户 ID"
// @Param        request  body  types.AdminUpdateUserRequest  true  "变更字段"
// @Success      200  {object}  types.UserInfo
// @Failure      400  {object}  map[string]interface{}  "self-action / validation"
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Failure      409  {object}  map[string]interface{}  "identity conflict"
// @Router       /system/admin/users/{user_id} [put]
func (h *SystemHandler) AdminUpdateUser(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	var req types.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}
	if req.Username == nil && req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	userID := c.Param("user_id")
	callerID, _ := types.UserIDFromContext(ctx)
	user, err := svc.AdminUpdateUserProfile(ctx, callerID, userID, req)
	if err != nil {
		h.adminUserError(c, err)
		return
	}

	changed := make([]string, 0, 2)
	if req.Username != nil {
		changed = append(changed, "username")
	}
	if req.Email != nil {
		changed = append(changed, "email")
	}
	h.emitAdminAudit(ctx, types.AuditActionSystemUserUpdated, user, map[string]any{
		"target_email":    user.Email,
		"target_username": user.Username,
		"changed_fields":  changed,
	})
	c.JSON(http.StatusOK, user.ToUserInfo())
}

// AdminSetUserStatus godoc
// @Summary      启用/禁用账号
// @Description  禁用后登录被拒且全部在线令牌立即撤销（下一请求 401）；幂等；不能操作自己
// @Tags         System Admin
// @Accept       json
// @Produce      json
// @Param        user_id  path  string  true  "用户 ID"
// @Param        request  body  types.AdminSetUserStatusRequest  true  "目标状态"
// @Success      200  {object}  types.UserInfo
// @Failure      400  {object}  map[string]interface{}  "self-action / invalid body"
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Router       /system/admin/users/{user_id}/status [put]
func (h *SystemHandler) AdminSetUserStatus(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	var req types.AdminSetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: is_active is required"})
		return
	}

	userID := c.Param("user_id")
	callerID, _ := types.UserIDFromContext(ctx)
	user, err := svc.AdminSetUserStatus(ctx, callerID, userID, *req.IsActive)
	if err != nil {
		h.adminUserError(c, err)
		return
	}

	h.emitAdminAudit(ctx, types.AuditActionSystemUserStatusChanged, user, map[string]any{
		"target_email":     user.Email,
		"target_username":  user.Username,
		"is_active":        *req.IsActive,
		"sessions_revoked": !*req.IsActive,
	})
	c.JSON(http.StatusOK, user.ToUserInfo())
}

// AdminForceLogout godoc
// @Summary      强制下线账号全部会话
// @Description  撤销目标账号全部活跃令牌（下一请求 401 跳登录），不改变启用状态；不能操作自己
// @Tags         System Admin
// @Produce      json
// @Param        user_id  path  string  true  "用户 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Router       /system/admin/users/{user_id}/force-logout [post]
func (h *SystemHandler) AdminForceLogout(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	userID := c.Param("user_id")
	callerID, _ := types.UserIDFromContext(ctx)
	target, err := h.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	if err := svc.AdminForceLogout(ctx, callerID, userID); err != nil {
		h.adminUserError(c, err)
		return
	}

	h.emitAdminAudit(ctx, types.AuditActionSystemUserForceLogout, target, map[string]any{
		"target_email":     target.Email,
		"target_username":  target.Username,
		"sessions_revoked": true,
	})
	c.JSON(http.StatusOK, gin.H{"message": "All sessions for the account have been revoked"})
}

// AdminListUserSessions godoc
// @Summary      查看账号的活跃登录会话
// @Description  返回目标账号的全部活跃会话（设备/IP/建立时间/最近活跃），口径与个人「登录设备」面板一致
// @Tags         System Admin
// @Produce      json
// @Param        user_id  path  string  true  "用户 ID"
// @Success      200  {array}  types.LoginSession
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Router       /system/admin/users/{user_id}/sessions [get]
func (h *SystemHandler) AdminListUserSessions(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	sessions, err := svc.AdminListUserSessions(ctx, c.Param("user_id"))
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// AdminRevokeUserSession godoc
// @Summary      踢除账号的一个登录会话
// @Description  撤销目标账号指定 session_id 的全部令牌；不能操作自己（自己的会话走个人面板）
// @Tags         System Admin
// @Produce      json
// @Param        user_id     path  string  true  "用户 ID"
// @Param        session_id  path  string  true  "会话 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}  "self-action / empty session id"
// @Failure      404  {object}  map[string]interface{}  "User not found"
// @Router       /system/admin/users/{user_id}/sessions/{session_id} [delete]
func (h *SystemHandler) AdminRevokeUserSession(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	userID := c.Param("user_id")
	sessionID := c.Param("session_id")
	callerID, _ := types.UserIDFromContext(ctx)
	target, err := h.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	if err := svc.AdminRevokeUserSession(ctx, callerID, userID, sessionID); err != nil {
		h.adminUserError(c, err)
		return
	}

	h.emitAdminAudit(ctx, types.AuditActionSystemSessionRevoked, target, map[string]any{
		"target_email":    target.Email,
		"target_username": target.Username,
		"session_id":      sessionID,
	})
	c.JSON(http.StatusOK, gin.H{"message": "Session revoked"})
}

// AdminUpdateMembershipRole godoc
// @Summary      调整账号在某空间的角色
// @Description  通过系统管理员通道修改目标用户在指定空间的成员角色；最后一位 Owner 保护沿用成员服务
// @Tags         System Admin
// @Accept       json
// @Produce      json
// @Param        user_id    path  string  true  "用户 ID"
// @Param        tenant_id  path  int     true  "空间 ID"
// @Param        request    body  types.AdminUpdateMembershipRoleRequest  true  "新角色"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}  "self-action / invalid role / last owner"
// @Failure      404  {object}  map[string]interface{}  "User or membership not found"
// @Router       /system/admin/users/{user_id}/memberships/{tenant_id} [put]
func (h *SystemHandler) AdminUpdateMembershipRole(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	var req types.AdminUpdateMembershipRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: role is required"})
		return
	}

	userID := c.Param("user_id")
	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil || tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant id"})
		return
	}

	callerID, _ := types.UserIDFromContext(ctx)
	target, err := h.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	oldRole, newRole, err := svc.AdminUpdateMembershipRole(ctx, callerID, userID, tenantID, req.Role)
	if err != nil {
		h.adminUserError(c, err)
		return
	}

	// rbac.member_role_changed（tenant-scope）已由成员服务落库；补一条
	// system-scope 行，让平台审计面板能溯源「系统管理员通道」的变更。
	h.emitAdminAudit(ctx, types.AuditActionSystemMembershipRoleChanged, target, map[string]any{
		"target_email":    target.Email,
		"target_username": target.Username,
		"tenant_id":       tenantID,
		"old_role":        string(oldRole),
		"new_role":        string(newRole),
	})
	c.JSON(http.StatusOK, gin.H{"message": "Member role updated", "old_role": string(oldRole), "new_role": string(newRole)})
}

// AdminRemoveMembership godoc
// @Summary      将账号移出空间
// @Description  通过系统管理员通道把目标用户移出指定空间；移出即撤销其全部令牌（沿用成员服务语义）
// @Tags         System Admin
// @Produce      json
// @Param        user_id    path  string  true  "用户 ID"
// @Param        tenant_id  path  int     true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}  "self-action / last owner"
// @Failure      404  {object}  map[string]interface{}  "User or membership not found"
// @Router       /system/admin/users/{user_id}/memberships/{tenant_id} [delete]
func (h *SystemHandler) AdminRemoveMembership(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	svc, ok := h.adminUserSvc()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Account management service unavailable"})
		return
	}

	userID := c.Param("user_id")
	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil || tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant id"})
		return
	}

	callerID, _ := types.UserIDFromContext(ctx)
	target, err := h.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		h.adminUserError(c, err)
		return
	}
	if err := svc.AdminRemoveMembership(ctx, callerID, userID, tenantID); err != nil {
		h.adminUserError(c, err)
		return
	}

	// rbac.member_removed（tenant-scope）已由成员服务落库；补 system-scope 行。
	h.emitAdminAudit(ctx, types.AuditActionSystemMembershipRemoved, target, map[string]any{
		"target_email":    target.Email,
		"target_username": target.Username,
		"tenant_id":       tenantID,
	})
	c.JSON(http.StatusOK, gin.H{"message": "Member removed from workspace"})
}

// atoiDefault parses a positive int query value, falling back to def on
// absence or garbage (a malformed page number should never 400 the list).
func atoiDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	if v, err := strconv.Atoi(raw); err == nil && v > 0 {
		return v
	}
	return def
}
