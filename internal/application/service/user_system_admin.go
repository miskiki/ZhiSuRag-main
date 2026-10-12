package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ErrAdminSelfAction is returned when a system administrator attempts a
// platform-level account-management mutation on their own account. The
// personal surfaces (用户资料 / 登录设备 / 空间成员管理) remain the only
// path for self-service changes, so the admin channel can never lock the
// operator out by accident (v2.19 账号管理自我保护规则 #1).
var ErrAdminSelfAction = errors.New("cannot perform account management actions on your own account")

// adminUserProfileUsernameBounds mirrors the AdminCreateUserRequest binding
// (2-50 characters) so an admin edit can't smuggle a username the create
// path would reject.
const (
	adminUsernameMinLen = 2
	adminUsernameMaxLen = 50
)

// AdminListUsers returns one page of the platform user directory
// (v2.19 账号管理). Page/pageSize are normalised defensively: page < 1 → 1,
// pageSize out of range → clamped to [1, 100] with a 20 default.
func (s *userService) AdminListUsers(
	ctx context.Context, query types.AdminUserListQuery,
) (*types.AdminUserListResponse, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	users, total, err := s.userRepo.ListUsersPaged(
		ctx, query.Search, query.IsActive, query.IsSystemAdmin,
		(query.Page-1)*query.PageSize, query.PageSize,
	)
	if err != nil {
		return nil, err
	}

	items := make([]*types.UserInfo, 0, len(users))
	for _, u := range users {
		items = append(items, u.ToUserInfo())
	}
	return &types.AdminUserListResponse{
		Users:    items,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

// AdminGetUserDetail returns the target account's profile plus every
// workspace membership (role / lifecycle status / workspace name).
// Workspace-name resolution is best-effort: a lookup failure degrades to
// empty names (logged) instead of failing the whole detail view — the
// membership rows themselves are the operational payload.
func (s *userService) AdminGetUserDetail(
	ctx context.Context, userID string,
) (*types.AdminUserDetail, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	memberships, err := s.memberService.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	tenantIDs := make([]uint64, 0, len(memberships))
	for _, m := range memberships {
		if m != nil {
			tenantIDs = append(tenantIDs, m.TenantID)
		}
	}
	tenants := map[uint64]*types.Tenant{}
	if len(tenantIDs) > 0 {
		tenants, err = s.tenantService.GetTenantsByIDs(ctx, tenantIDs)
		if err != nil {
			// Cosmetic-only failure: keep the detail payload, blank names.
			logger.Warnf(ctx, "AdminGetUserDetail: tenant name lookup failed for user %s: %v", userID, err)
			tenants = map[uint64]*types.Tenant{}
		}
	}

	views := make([]types.AdminMembershipView, 0, len(memberships))
	for _, m := range memberships {
		if m == nil {
			continue
		}
		name := ""
		if t := tenants[m.TenantID]; t != nil {
			name = t.Name
		}
		views = append(views, types.AdminMembershipView{
			TenantID:   m.TenantID,
			TenantName: name,
			Role:       m.Role,
			Status:     m.Status,
			JoinedAt:   m.JoinedAt,
		})
	}
	return &types.AdminUserDetail{User: user.ToUserInfo(), Memberships: views}, nil
}

// AdminUpdateUserProfile PATCHes another user's username/email with the
// same uniqueness checks the registration path applies. Only provided
// fields change; unchanged values skip the duplicate lookups entirely.
func (s *userService) AdminUpdateUserProfile(
	ctx context.Context, actorID, userID string, req types.AdminUpdateUserRequest,
) (*types.User, error) {
	if actorID == userID {
		return nil, ErrAdminSelfAction
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Username != nil {
		name := strings.TrimSpace(*req.Username)
		if n := utf8.RuneCountInString(name); n < adminUsernameMinLen || n > adminUsernameMaxLen {
			return nil, errors.New("username must be 2-50 characters")
		}
		if name != user.Username {
			if existing, _ := s.userRepo.GetUserByUsername(ctx, name); existing != nil && existing.ID != userID {
				return nil, ErrUserUsernameExists
			}
			user.Username = name
		}
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			return nil, errors.New("email is required")
		}
		if email != user.Email {
			if existing, _ := s.userRepo.GetUserByEmail(ctx, email); existing != nil && existing.ID != userID {
				return nil, ErrUserEmailExists
			}
			user.Email = email
		}
	}

	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// AdminSetUserStatus enables/disables an account. Disabling additionally
// revokes every live token — the JWT middleware does not re-check
// users.is_active per request, but every request validates token revocation,
// so the revoke IS the lockout (next request 401). Login/OIDC paths already
// reject inactive accounts ("Account is disabled"), so no new login checks
// are needed. Idempotent on the current status.
func (s *userService) AdminSetUserStatus(
	ctx context.Context, actorID, userID string, isActive bool,
) (*types.User, error) {
	if actorID == userID {
		return nil, ErrAdminSelfAction
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.IsActive == isActive {
		return user, nil
	}

	user.IsActive = isActive
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	if !isActive {
		if err := s.tokenRepo.RevokeTokensByUserID(ctx, userID); err != nil {
			logger.Errorf(ctx, "AdminSetUserStatus: failed to revoke tokens for disabled user %s: %v", userID, err)
			return nil, err
		}
	}
	return user, nil
}

// AdminForceLogout revokes every live session of the target account without
// touching is_active. The user-existence check gives the handler clean 404
// semantics (the token repo alone would silently succeed on unknown ids).
func (s *userService) AdminForceLogout(ctx context.Context, actorID, userID string) error {
	if actorID == userID {
		return ErrAdminSelfAction
	}
	if _, err := s.userRepo.GetUserByID(ctx, userID); err != nil {
		return err
	}
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// AdminListUserSessions projects the target account's live login sessions
// in the exact shape the personal panel consumes (v2.15 aggregation), with
// no "current device" marker — the admin request never belongs to the
// target's session set (self-action guard upstream).
func (s *userService) AdminListUserSessions(
	ctx context.Context, userID string,
) ([]types.LoginSession, error) {
	if _, err := s.userRepo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.ListActiveSessions(ctx, userID, "")
}

// AdminRevokeUserSession kicks one login session of the target account.
// Self-action is rejected (an admin kicks their own sessions via the
// personal 登录设备 panel, which also protects the "current device").
func (s *userService) AdminRevokeUserSession(
	ctx context.Context, actorID, userID, sessionID string,
) error {
	if actorID == userID {
		return ErrAdminSelfAction
	}
	if _, err := s.userRepo.GetUserByID(ctx, userID); err != nil {
		return err
	}
	return s.RevokeSession(ctx, userID, "", sessionID)
}

// AdminUpdateMembershipRole changes the target user's role inside one
// workspace through TenantMemberService — which owns the role validation
// and the atomic last-owner protection — and reports the previous role so
// the handler can audit old→new in one row.
func (s *userService) AdminUpdateMembershipRole(
	ctx context.Context, actorID, userID string, tenantID uint64, role types.TenantRole,
) (oldRole, newRole types.TenantRole, err error) {
	if actorID == userID {
		return "", "", ErrAdminSelfAction
	}
	memberships, err := s.memberService.ListByUser(ctx, userID)
	if err != nil {
		return "", "", err
	}
	var current *types.TenantMember
	for _, m := range memberships {
		if m != nil && m.TenantID == tenantID {
			current = m
			break
		}
	}
	if current == nil {
		return "", "", ErrMembershipNotFound
	}
	oldRole = current.Role
	if err := s.memberService.UpdateRole(ctx, userID, tenantID, role); err != nil {
		return "", "", err
	}
	return oldRole, role, nil
}

// AdminRemoveMembership removes the target user from one workspace via
// TenantMemberService — inheriting its last-owner protection, removal
// audit (rbac.member_removed) and the post-removal token revocation that
// keeps stale JWTs from serving 403-only UI.
func (s *userService) AdminRemoveMembership(
	ctx context.Context, actorID, userID string, tenantID uint64,
) error {
	if actorID == userID {
		return ErrAdminSelfAction
	}
	return s.memberService.RemoveMember(ctx, userID, tenantID)
}

// Compile-time guard: *userService must keep satisfying the narrow admin
// interface.
var _ interfaces.SystemUserAdminService = (*userService)(nil)
