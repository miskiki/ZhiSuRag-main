package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserPreferences holds per-user preferences persisted server-side
// so they sync across devices/browsers. Fields are pointers so we can
// distinguish "client didn't send this key" (leave existing value alone)
// from "client explicitly set false" — the partial-update merge in
// UpdateUserPreferences relies on this.
//
// Adding a new preference key:
//  1. Add a *T field below + JSON tag (snake_case, must match the front-end key).
//  2. Extend the merge logic in service.UserService.UpdateUserPreferences.
//  3. Surface the new knob in the frontend settings store.
//
// No DB DDL is required — preferences is a single jsonb column.
type UserPreferences struct {
	// LastActiveTenantID remembers the last workspace the user actively
	// switched into, so a fresh login (new device, cleared browser, new
	// refresh token) lands them back in that workspace instead of always
	// bouncing to their home workspace. Login / RefreshToken validate that
	// the workspace still exists and the user still has an active membership
	// (or CanAccessAllTenants) before honouring this preference; an
	// invalid pointer is best-effort cleared and the user falls back to
	// home.
	//
	// nil  = no preference (use user.TenantID, i.e. home)
	// *0   = "clear preference" sentinel for the partial-update endpoint
	//        (UpdateUserPreferences turns this into nil). Otherwise treat
	//        a stored *0 the same as nil.
	// *N   = preferred workspace id.
	LastActiveTenantID *uint64 `json:"last_active_tenant_id,omitempty"`

	// OidcOnlyLogin is set server-side when an account is auto-provisioned
	// via OIDC with a random password the user never received. The profile
	// UI hides self-service password rotation until the user sets a known
	// password via ChangePassword (which clears this flag).
	OidcOnlyLogin *bool `json:"oidc_only_login,omitempty"`
}

// Value implements driver.Valuer so GORM persists UserPreferences as
// JSON text (Postgres jsonb column / SQLite TEXT). Empty struct serialises
// to "{}", matching the NOT NULL DEFAULT '{}' column constraint.
func (p UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements sql.Scanner so GORM can hydrate UserPreferences back
// from the underlying column. Accept []byte (Postgres jsonb / SQLite blob)
// and string (some drivers hand TEXT as string) for portability.
func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*p = UserPreferences{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("UserPreferences.Scan: unsupported type")
	}
	if len(data) == 0 {
		*p = UserPreferences{}
		return nil
	}
	return json.Unmarshal(data, p)
}

// User represents a user in the system
type User struct {
	// Unique identifier of the user
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// Username of the user
	Username string `json:"username"   gorm:"type:varchar(100);uniqueIndex;not null"`
	// Email address of the user
	Email string `json:"email"      gorm:"type:varchar(255);uniqueIndex;not null"`
	// Hashed password of the user
	PasswordHash string `json:"-"          gorm:"type:varchar(255);not null"`
	// Avatar URL of the user
	Avatar string `json:"avatar"     gorm:"type:varchar(500)"`
	// Workspace ID that the user belongs to
	TenantID uint64 `json:"tenant_id"  gorm:"index"`
	// Whether the user is active
	IsActive bool `json:"is_active"  gorm:"default:true"`
	// Whether the user can access all workspaces (cross-workspace access)
	CanAccessAllTenants bool `json:"can_access_all_tenants" gorm:"default:false"`
	// Whether the user is a system administrator (independent of workspace roles)
	IsSystemAdmin bool `json:"is_system_admin" gorm:"default:false;index"`
	// Per-user UI/feature preferences.
	// Stored as JSON (jsonb on Postgres, TEXT on SQLite) via the
	// driver.Valuer / sql.Scanner methods on UserPreferences.
	Preferences UserPreferences `json:"preferences" gorm:"type:jsonb;not null;default:'{}'"`
	// Creation time of the user
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the user
	UpdatedAt time.Time `json:"updated_at"`
	// Deletion time of the user
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// Association relationship, not stored in the database
	Tenant *Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}

// AuthToken represents an authentication token
type AuthToken struct {
	// Unique identifier of the token
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// User ID that owns this token
	UserID string `json:"user_id"    gorm:"type:varchar(36);index;not null"`
	// Token value (JWT or other format)
	Token string `json:"token"      gorm:"type:text;not null"`
	// Token type (access_token, refresh_token)
	TokenType string `json:"token_type" gorm:"type:varchar(50);not null"`
	// Token expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// Whether the token is revoked
	IsRevoked bool `json:"is_revoked" gorm:"default:false"`
	// Creation time of the token
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the token
	UpdatedAt time.Time `json:"updated_at"`

	// v2.15 多端会话管理（migration 000088）：一次登录会话的标识，同一登录
	// 签发的 access + refresh 令牌共享；刷新轮换继承该值以保持会话连续。
	// 令牌在迁移前签发或无 UA 场景下为空（UI 显示为早期会话）。
	SessionID string `json:"session_id,omitempty"  gorm:"type:varchar(36);index"`
	// 设备友好描述（登录时从 User-Agent 解析，如 "Chrome 126 · Windows"）
	DeviceLabel string `json:"device_label,omitempty" gorm:"type:varchar(255);default:''"`
	// 会话建立时的客户端 IP（展示与排障用）
	DeviceIP string `json:"device_ip,omitempty"   gorm:"type:varchar(64);default:''"`

	// Association relationship
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// LoginSession is the projected view of one login session for the
// multi-device management panel (v2.15 多端会话管理)。One row per
// session_id; the newest live token of the session supplies the fields.
type LoginSession struct {
	// Session identifier (shared by the access + refresh tokens of the login)
	SessionID string `json:"session_id"`
	// Human-readable device description parsed from the login User-Agent
	DeviceLabel string `json:"device_label"`
	// Client IP at session establishment
	DeviceIP string `json:"device_ip"`
	// Creation time of the session (first token of the login)
	CreatedAt time.Time `json:"created_at"`
	// Expiry of the refresh token (session lifetime upper bound)
	ExpiresAt time.Time `json:"expires_at"`
	// LastRefreshAt is the update time of the newest token of the session
	// (proxy for "last active" — refresh rotation touches it)
	LastRefreshAt time.Time `json:"last_refresh_at"`
	// Current marks the session the management request came from.
	Current bool `json:"current"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type OIDCAuthURLResponse struct {
	Success             bool   `json:"success"`
	ProviderDisplayName string `json:"provider_display_name,omitempty"`
	AuthorizationURL    string `json:"authorization_url,omitempty"`
	State               string `json:"state,omitempty"`
	// Nonce is bound to an HttpOnly cookie on /auth/oidc/url and verified
	// on callback; omitted from JSON so clients cannot replay it alone.
	Nonce string `json:"-"`
}

type OIDCConfigResponse struct {
	Success             bool   `json:"success"`
	Enabled             bool   `json:"enabled"`
	ProviderDisplayName string `json:"provider_display_name,omitempty"`
}

type OIDCCallbackResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
	// Tenant carries the active tenant for the issued token. The field
	// name is preserved for backward compatibility with existing frontend
	// OIDC callback handling; LoginResponse uses ActiveTenant for the
	// same data.
	Tenant *Tenant `json:"tenant,omitempty"`
	// Memberships mirrors LoginResponse.Memberships so the OIDC flow
	// produces the same role information available to password logins.
	// Always populated (length >= 1 for an authenticated user).
	Memberships  []Membership `json:"memberships"`
	Token        string       `json:"token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	IsNewUser    bool         `json:"is_new_user,omitempty"`
}

type OIDCUserInfo struct {
	Subject  string                 `json:"subject,omitempty"`
	Username string                 `json:"username,omitempty"`
	Email    string                 `json:"email,omitempty"`
	Claims   map[string]interface{} `json:"claims,omitempty"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=2,max=50"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`

	// TenantProvisioning is server-controlled registration context. It is
	// deliberately excluded from JSON so a public caller cannot choose its
	// own tenancy semantics. Empty preserves the historical behaviour and is
	// treated as create_personal by UserService.Register.
	TenantProvisioning TenantProvisioningMode `json:"-"`
}

// AdminCreateUserRequest is the payload for a SystemAdmin provisioning a
// new local user via POST /api/v1/system/admin/users/create.
//
// Password is optional: when absent (or null), the service generates a
// random one and returns it exactly once. Any provided value, the
// empty string included, is subject to the password policy.
type AdminCreateUserRequest struct {
	Username string  `json:"username" binding:"required,min=2,max=50"`
	Email    string  `json:"email"    binding:"required,email"`
	Password *string `json:"password"`
}

// TenantProvisioningMode controls what UserService.Register does after it
// has validated the identity fields. Joining an existing tenant is
// orchestrated by the invitation handler because the invitation token is the
// authority for the target tenant and role.
type TenantProvisioningMode string

const (
	TenantProvisioningCreatePersonal TenantProvisioningMode = "create_personal"
	TenantProvisioningTenantless     TenantProvisioningMode = "tenantless"
)

func (m TenantProvisioningMode) IsValid() bool {
	return m == TenantProvisioningCreatePersonal || m == TenantProvisioningTenantless
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
	// ActiveTenant is the workspace whose ID is encoded in the issued JWT;
	// future requests are scoped to it until the client calls /auth/switch-tenant.
	// Defaults to the user's home workspace on a fresh login.
	ActiveTenant *Tenant `json:"active_tenant,omitempty"`
	// Memberships lists every workspace the user can authenticate into,
	// along with their role in each. Always populated (length 1 for users
	// who only belong to their home workspace) so frontends can render a
	// workspace switcher without a follow-up request. Serialised without
	// omitempty so the field is always present as a JSON array (possibly
	// empty) — the "always populated" contract relies on the server side
	// guaranteeing a non-nil slice.
	Memberships  []Membership `json:"memberships"`
	Token        string       `json:"token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	Success bool    `json:"success"`
	Message string  `json:"message,omitempty"`
	User    *User   `json:"user,omitempty"`
	Tenant  *Tenant `json:"tenant,omitempty"`
}

// UserInfo represents user information for API responses
type UserInfo struct {
	ID                  string          `json:"id"`
	Username            string          `json:"username"`
	Email               string          `json:"email"`
	Avatar              string          `json:"avatar"`
	TenantID            uint64          `json:"tenant_id"`
	IsActive            bool            `json:"is_active"`
	CanAccessAllTenants bool            `json:"can_access_all_tenants"`
	IsSystemAdmin       bool            `json:"is_system_admin"`
	Preferences         UserPreferences `json:"preferences"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// ToUserInfo converts User to UserInfo (without sensitive data)
func (u *User) ToUserInfo() *UserInfo {
	return &UserInfo{
		ID:                  u.ID,
		Username:            u.Username,
		Email:               u.Email,
		Avatar:              u.Avatar,
		TenantID:            u.TenantID,
		IsActive:            u.IsActive,
		CanAccessAllTenants: u.CanAccessAllTenants,
		IsSystemAdmin:       u.IsSystemAdmin,
		Preferences:         u.Preferences,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
}

// ============================================================================
// v2.19 账号管理（SystemAdmin 平台级账号治理）请求/响应类型。
// 全部挂 /api/v1/system/admin/users* 路由族，响应为裸 JSON（无
// {code,msg,data} 包装），对齐 /system/admin 家族约定。
// ============================================================================

// AdminUserListQuery is the validated list request for
// GET /api/v1/system/admin/users. Search matches username OR email
// case-insensitively; the two bool pointers are tri-state filters
// (nil = no filter).
type AdminUserListQuery struct {
	Search        string
	IsActive      *bool
	IsSystemAdmin *bool
	Page          int
	PageSize      int
}

// AdminUserListResponse is the paged payload returned by the admin user
// list endpoint. Users is always non-nil so callers can serialise an
// empty JSON array.
type AdminUserListResponse struct {
	Users    []*UserInfo `json:"users"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// AdminMembershipView projects one (user, tenant) membership row for the
// admin account-detail panel: role + lifecycle status + human workspace
// name. Deliberately separate from Membership (login-response shape) so
// the auth payload stays frozen while the admin view evolves.
type AdminMembershipView struct {
	TenantID   uint64             `json:"tenant_id"`
	TenantName string             `json:"tenant_name"`
	Role       TenantRole         `json:"role"`
	Status     TenantMemberStatus `json:"status"`
	JoinedAt   time.Time          `json:"joined_at"`
}

// AdminUserDetail is the payload of GET /api/v1/system/admin/users/:id —
// the account profile plus every workspace membership the admin can
// inspect/adjust.
type AdminUserDetail struct {
	User        *UserInfo             `json:"user"`
	Memberships []AdminMembershipView `json:"memberships"`
}

// AdminUpdateUserRequest edits another user's profile. Pointer fields are
// PATCH semantics: nil = leave unchanged. Email uniqueness is enforced by
// the service (409 on conflict).
type AdminUpdateUserRequest struct {
	Username *string `json:"username" binding:"omitempty,min=2,max=50"`
	Email    *string `json:"email"    binding:"omitempty,email"`
}

// AdminSetUserStatusRequest enables/disables an account. Disabling
// additionally revokes every live token so the account is locked out on
// the next request (login is already rejected for inactive users).
type AdminSetUserStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// AdminUpdateMembershipRoleRequest sets the target user's role inside one
// workspace. Last-owner protection is enforced by TenantMemberService.
type AdminUpdateMembershipRoleRequest struct {
	Role TenantRole `json:"role" binding:"required"`
}
