package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// UserService defines the user service interface
type UserService interface {
	// Register creates a new user account
	Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error)
	// Login authenticates a user and returns tokens
	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	// GetOIDCAuthorizationURL builds the third-party OIDC authorization URL
	GetOIDCAuthorizationURL(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error)
	// LoginWithOIDC exchanges the callback code, auto-provisions users if needed, and completes login.
	// provisioning is the default tenant mode for a newly auto-created user
	// (resolved by the caller from auth.default_tenant_mode).
	LoginWithOIDC(ctx context.Context, code, redirectURI string, provisioning types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error)
	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs batch-fetches users by id, returning a map keyed by
	// user id. Missing ids are simply absent from the result; the call
	// is not an error when some ids resolve to no row. Used on hot list
	// endpoints (tenant members, audit logs) to avoid N+1 queries.
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// GetUserByUsername gets a user by username
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	// GetUserByTenantID gets the first user (owner) of a tenant
	GetUserByTenantID(ctx context.Context, tenantID uint64) (*types.User, error)
	// UpdateUser updates user information
	UpdateUser(ctx context.Context, user *types.User) error
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, id string) error
	// ChangePassword changes user password
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	// AdminResetPassword replaces a user's password without requiring the old
	// password and revokes all of that user's existing sessions. Callers must
	// enforce the system-admin and cannot-reset-self guards before invoking it.
	AdminResetPassword(ctx context.Context, userID string, newPassword string) error
	// ValidatePassword validates user password
	ValidatePassword(ctx context.Context, userID string, password string) error
	// GenerateTokens generates access and refresh tokens for user
	GenerateTokens(ctx context.Context, user *types.User) (accessToken, refreshToken string, err error)
	// BuildLoginMemberships projects the user's tenant memberships into
	// the login-response shape. activeTenant is reused (without an extra
	// lookup) for the matching row's TenantName. The slice is guaranteed
	// non-nil so callers can serialise it as an empty JSON array when the
	// membership table is unavailable.
	BuildLoginMemberships(ctx context.Context, user *types.User, activeTenant *types.Tenant) []types.Membership
	// SwitchTenant issues a new token pair scoped to targetTenantID and
	// returns the corresponding LoginResponse. The caller's previous
	// refresh token (passed in for revocation) is invalidated. Membership
	// is verified via the TenantMember service before tokens are issued.
	SwitchTenant(ctx context.Context, user *types.User, targetTenantID uint64, currentRefreshToken string) (*types.LoginResponse, error)
	// ValidateToken validates an access token. It returns the user
	// referenced by the token plus the active tenant ID encoded in the
	// JWT's `tenant_id` claim — the latter lets the auth middleware
	// honour /auth/switch-tenant sessions that were minted with a
	// non-home tenant. Falls back to user.TenantID when the claim is
	// missing (old tokens issued before tenant-level RBAC).
	ValidateToken(ctx context.Context, token string) (*types.User, uint64, error)
	// RefreshToken refreshes access token using refresh token
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token string) error
	// Logout revokes every outstanding access/refresh token for the user
	// identified by the presented JWT.
	Logout(ctx context.Context, token string) error
	// ListActiveSessions projects the caller's live login sessions
	// (v2.15 会话管理)：one row per session_id, newest token wins.
	// currentSessionID (may be empty for pre-000088 tokens) is echoed back
	// so the UI can highlight "this device" — it is NEVER used to filter.
	ListActiveSessions(ctx context.Context, userID, currentSessionID string) ([]types.LoginSession, error)
	// RevokeSession kicks one login session of the caller. Revoking the
	// caller's own current session is rejected (self-lockout guard).
	RevokeSession(ctx context.Context, userID, currentSessionID, targetSessionID string) error
	// ParseSessionIDFromToken extracts the login session id from a signed
	// access token without expiry validation (handler helper for the
	// session panel's "current device" highlight).
	ParseSessionIDFromToken(token string) (string, error)
	// GetCurrentUser gets current user from context
	GetCurrentUser(ctx context.Context) (*types.User, error)
	// SearchUsers searches users by username or email
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
	// ListSystemAdmins lists users with IsSystemAdmin=true.
	// Returns the page of admins plus the total count (for pagination UI);
	// callers pass offset/limit to page through results. Used by the
	// /api/v1/system/admin/list endpoint, gated to SystemAdmin callers.
	ListSystemAdmins(ctx context.Context, offset, limit int) ([]*types.User, int64, error)
	// AdminCreateUser provisions a new local user on behalf of a
	// SystemAdmin. When req.Password is nil, a random password is generated
	// and returned exactly once as the second result. provisioning is
	// resolved by the caller from the shared auth.default_tenant_mode policy.
	AdminCreateUser(
		ctx context.Context, req *types.AdminCreateUserRequest, provisioning types.TenantProvisioningMode,
	) (*types.User, string, error)
	// RevokeSystemAdmin removes system-admin privileges with the
	// last-admin/self-revoke checks performed atomically.
	RevokeSystemAdmin(ctx context.Context, userID, actorID string) (*types.User, error)
	// UpdateUserPreferences partially updates the calling user's
	// preferences blob (PATCH semantics: only keys present in `patch`
	// overwrite existing values). Returns the updated, persisted prefs.
	UpdateUserPreferences(ctx context.Context, userID string, patch types.UserPreferences) (types.UserPreferences, error)
}

// SystemUserAdminService carries the platform-level account-management
// operations (v2.19 账号管理): cross-account CRUD-without-delete, session
// governance and cross-workspace role adjustments, all reserved for
// SystemAdmin callers. Implemented by the same *userService that satisfies
// UserService; the narrow interface exists so handlers and tests can depend
// on only these operations (the fat UserService interface stays untouched
// for its many existing stubs).
//
// Self-action guard: every mutating method takes actorID and rejects
// actorID == userID with service.ErrAdminSelfAction — a system admin
// manages their own profile/sessions/memberships through the personal
// surfaces (用户资料 / 登录设备 / 空间成员管理), not through this channel.
type SystemUserAdminService interface {
	// AdminListUsers returns one page of the platform user directory.
	AdminListUsers(ctx context.Context, query types.AdminUserListQuery) (*types.AdminUserListResponse, error)
	// AdminGetUserDetail returns a target account's profile plus every
	// workspace membership (role/status/workspace name).
	AdminGetUserDetail(ctx context.Context, userID string) (*types.AdminUserDetail, error)
	// AdminUpdateUserProfile PATCHes username/email with uniqueness checks
	// and returns the updated user.
	AdminUpdateUserProfile(
		ctx context.Context, actorID, userID string, req types.AdminUpdateUserRequest,
	) (*types.User, error)
	// AdminSetUserStatus enables/disables an account. Disabling revokes all
	// the target's tokens so the lockout takes effect on the next request.
	// Idempotent: setting the current status is a no-op returning the user.
	AdminSetUserStatus(ctx context.Context, actorID, userID string, isActive bool) (*types.User, error)
	// AdminForceLogout revokes every live session of the target account
	// (without touching is_active).
	AdminForceLogout(ctx context.Context, actorID, userID string) error
	// AdminListUserSessions projects the target account's live login
	// sessions (same shape as the personal panel; no "current device").
	AdminListUserSessions(ctx context.Context, userID string) ([]types.LoginSession, error)
	// AdminRevokeUserSession kicks one login session of the target account.
	AdminRevokeUserSession(ctx context.Context, actorID, userID, sessionID string) error
	// AdminUpdateMembershipRole changes the target user's role inside one
	// workspace, returning the previous and new roles for audit. Last-owner
	// protection is inherited from TenantMemberService.
	AdminUpdateMembershipRole(
		ctx context.Context, actorID, userID string, tenantID uint64, role types.TenantRole,
	) (oldRole, newRole types.TenantRole, err error)
	// AdminRemoveMembership removes the target user from one workspace
	// (token revocation semantics inherited from TenantMemberService).
	AdminRemoveMembership(ctx context.Context, actorID, userID string, tenantID uint64) error
}

// UserRepository defines the user repository interface
type UserRepository interface {
	// CreateUser creates a user
	CreateUser(ctx context.Context, user *types.User) error
	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs batch-fetches users by id, returning a map keyed by
	// user id. Missing ids are simply absent from the result.
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// GetUserByUsername gets a user by username
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	// GetUserByTenantID gets the first user (owner) of a tenant
	GetUserByTenantID(ctx context.Context, tenantID uint64) (*types.User, error)
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *types.User) error
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, id string) error
	// ListUsers lists users with pagination
	ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error)
	// ListUsersPaged lists users with keyword search (username OR email,
	// case-insensitive) plus tri-state is_active / is_system_admin filters,
	// returning the page and the total count matching the same predicates
	// (v2.19 账号管理). Empty search / nil filters disable the respective
	// predicate. Ordered created_at DESC, id ASC for stable paging.
	ListUsersPaged(
		ctx context.Context, search string, isActive, isSystemAdmin *bool, offset, limit int,
	) ([]*types.User, int64, error)
	// ListSystemAdmins lists users where is_system_admin = true.
	// Walks the partial-friendly idx_users_is_system_admin index. Returns
	// the slice plus the total count for pagination metadata. Used by
	// the system-admin management endpoint.
	ListSystemAdmins(ctx context.Context, offset, limit int) ([]*types.User, int64, error)
	// RevokeSystemAdmin removes system-admin privileges with the
	// last-admin/self-revoke checks performed atomically.
	RevokeSystemAdmin(ctx context.Context, userID, actorID string) (*types.User, error)
	// SearchUsers searches users by username or email
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
}

// AuthTokenRepository defines the auth token repository interface
type AuthTokenRepository interface {
	// CreateToken creates an auth token
	CreateToken(ctx context.Context, token *types.AuthToken) error
	// GetTokenByValue gets a token by its value
	GetTokenByValue(ctx context.Context, tokenValue string) (*types.AuthToken, error)
	// GetTokensByUserID gets all tokens for a user
	GetTokensByUserID(ctx context.Context, userID string) ([]*types.AuthToken, error)
	// UpdateToken updates a token
	UpdateToken(ctx context.Context, token *types.AuthToken) error
	// DeleteToken deletes a token
	DeleteToken(ctx context.Context, id string) error
	// DeleteExpiredTokens deletes all expired tokens
	DeleteExpiredTokens(ctx context.Context) error
	// RevokeTokensByUserID revokes all tokens for a user
	RevokeTokensByUserID(ctx context.Context, userID string) error
	// RevokeBySessionID revokes every live token of one login session
	// (v2.15 会话管理：按 session_id 撤销该会话的 access + refresh 令牌)
	RevokeBySessionID(ctx context.Context, userID, sessionID string) error
	// GetLatestSessionToken returns the newest non-revoked, unexpired token
	// record for a (user, session) pair — the session view uses it to
	// project one row per login session.
	GetLatestSessionToken(ctx context.Context, userID, sessionID string) (*types.AuthToken, error)
}
