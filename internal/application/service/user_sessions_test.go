package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v2.15 多端会话管理：ListActiveSessions 聚合与 RevokeSession 自踢保护的
// 服务层测试。仓储用内存桩（与仓储接口对齐），不触库。

type sessionTokenStub struct {
	tokens map[string]*types.AuthToken // keyed by token value
}

func (s *sessionTokenStub) CreateToken(_ context.Context, token *types.AuthToken) error {
	cp := *token
	s.tokens[cp.Token] = &cp
	return nil
}
func (s *sessionTokenStub) GetTokenByValue(_ context.Context, tokenValue string) (*types.AuthToken, error) {
	if tk, ok := s.tokens[tokenValue]; ok {
		cp := *tk
		return &cp, nil
	}
	return nil, assert.AnError
}
func (s *sessionTokenStub) GetTokensByUserID(_ context.Context, userID string) ([]*types.AuthToken, error) {
	var out []*types.AuthToken
	for _, tk := range s.tokens {
		if tk.UserID == userID {
			cp := *tk
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (s *sessionTokenStub) UpdateToken(_ context.Context, token *types.AuthToken) error {
	cp := *token
	s.tokens[cp.Token] = &cp
	return nil
}
func (s *sessionTokenStub) DeleteToken(_ context.Context, _ string) error { return nil }
func (s *sessionTokenStub) DeleteExpiredTokens(_ context.Context) error   { return nil }
func (s *sessionTokenStub) RevokeTokensByUserID(_ context.Context, userID string) error {
	for _, tk := range s.tokens {
		if tk.UserID == userID {
			tk.IsRevoked = true
		}
	}
	return nil
}
func (s *sessionTokenStub) RevokeBySessionID(_ context.Context, userID, sessionID string) error {
	for _, tk := range s.tokens {
		if tk.UserID == userID && tk.SessionID == sessionID {
			tk.IsRevoked = true
		}
	}
	return nil
}
func (s *sessionTokenStub) GetLatestSessionToken(_ context.Context, userID, sessionID string) (*types.AuthToken, error) {
	var latest *types.AuthToken
	for _, tk := range s.tokens {
		if tk.UserID == userID && tk.SessionID == sessionID && !tk.IsRevoked && tk.ExpiresAt.After(time.Now()) {
			if latest == nil || tk.CreatedAt.After(latest.CreatedAt) {
				cp := *tk
				latest = &cp
			}
		}
	}
	if latest == nil {
		return nil, assert.AnError
	}
	return latest, nil
}

func newSessionUserService(t *testing.T) (*userService, *sessionTokenStub) {
	t.Helper()
	stub := &sessionTokenStub{tokens: map[string]*types.AuthToken{}}
	return &userService{tokenRepo: stub}, stub
}

func liveToken(userID, sessionID, tokenValue string, age time.Duration) *types.AuthToken {
	now := time.Now()
	return &types.AuthToken{
		ID:        tokenValue + "-id",
		UserID:    userID,
		Token:     tokenValue,
		TokenType: "access_token",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now.Add(-age),
		UpdatedAt: now.Add(-age),
		SessionID: sessionID,
	}
}

func TestListActiveSessions_GroupsBySession(t *testing.T) {
	svc, stub := newSessionUserService(t)
	const uid = "u1"
	stub.tokens["tok-a1"] = liveToken(uid, "sess-a", "tok-a1", 3*time.Hour)
	stub.tokens["tok-a1"].DeviceLabel = "Chrome 129 · Windows"
	stub.tokens["tok-a1"].DeviceIP = "10.0.0.8"
	stub.tokens["tok-a2"] = liveToken(uid, "sess-a", "tok-a2", 1*time.Hour) // refresh rotation (newer)
	stub.tokens["tok-b"] = liveToken(uid, "sess-b", "tok-b", 2*time.Hour)
	stub.tokens["tok-dead"] = liveToken(uid, "sess-c", "tok-dead", 2*time.Hour)
	stub.tokens["tok-dead"].IsRevoked = true
	stub.tokens["tok-expired"] = liveToken(uid, "sess-d", "tok-expired", 30*time.Hour)
	stub.tokens["tok-expired"].ExpiresAt = time.Now().Add(-time.Hour)

	sessions, err := svc.ListActiveSessions(context.Background(), uid, "sess-a")
	require.NoError(t, err)
	assert.Len(t, sessions, 2, "revoked/expired tokens are not live sessions")

	byID := map[string]types.LoginSession{}
	for _, s := range sessions {
		byID[s.SessionID] = s
	}
	a, ok := byID["sess-a"]
	require.True(t, ok)
	assert.Equal(t, "Chrome 129 · Windows", a.DeviceLabel)
	assert.True(t, a.Current, "sess-a is the caller's current session")
	assert.Equal(t, stub.tokens["tok-a2"].UpdatedAt, a.LastRefreshAt, "newest token supplies last activity")

	b, ok := byID["sess-b"]
	require.True(t, ok)
	assert.False(t, b.Current)
}

func TestListActiveSessions_EarlySessionBucket(t *testing.T) {
	svc, stub := newSessionUserService(t)
	legacy := liveToken("u1", "", "legacy-token", 2*time.Hour) // pre-000088 token (empty session id)
	stub.tokens["legacy-token"] = legacy

	sessions, err := svc.ListActiveSessions(context.Background(), "u1", "sess-x")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Empty(t, sessions[0].SessionID, "early session has no session id")
	assert.Contains(t, sessions[0].DeviceLabel, "早期会话")
	assert.False(t, sessions[0].Current)
}

func TestRevokeSession_KicksTargetButProtectsCurrent(t *testing.T) {
	svc, stub := newSessionUserService(t)
	stub.tokens["tok-a"] = liveToken("u1", "sess-a", "tok-a", time.Hour)
	stub.tokens["tok-b"] = liveToken("u1", "sess-b", "tok-b", time.Hour)

	// 自踢保护：当前会话不可踢
	err := svc.RevokeSession(context.Background(), "u1", "sess-a", "sess-a")
	assert.Error(t, err, "revoking the caller's own session must be rejected")
	assert.False(t, stub.tokens["tok-a"].IsRevoked, "own tokens stay live after rejected revoke")

	// 正常踢除：目标会话全部令牌失效，当前会话不受影响
	require.NoError(t, svc.RevokeSession(context.Background(), "u1", "sess-a", "sess-b"))
	assert.False(t, stub.tokens["tok-a"].IsRevoked, "current session untouched")
	assert.True(t, stub.tokens["tok-b"].IsRevoked, "target session revoked")

	// 空目标 ID 拒绝
	err = svc.RevokeSession(context.Background(), "u1", "sess-a", "")
	assert.Error(t, err)
}

func TestSessionIDInheritanceOnRefresh(t *testing.T) {
	// 会话继承的语义锁定：LoginSessionMeta.SessionID 非空时，令牌铸造
	// 沿用该 session_id（刷新轮换不产生新会话行）。直接验证 meta 传递
	// 合约：LoginSessionMetaFromContext 的往返。
	meta := types.LoginSessionMeta{DeviceLabel: "Chrome 129 · Windows", DeviceIP: "10.0.0.8", SessionID: "sess-keep"}
	ctx := types.WithLoginSessionMeta(context.Background(), meta)
	got := types.LoginSessionMetaFromContext(ctx)
	assert.Equal(t, "sess-keep", got.SessionID)
	assert.Equal(t, "Chrome 129 · Windows", got.DeviceLabel)
	// 缺省路径：无 meta 时空结构（测试/后台铸造兼容）
	assert.Empty(t, types.LoginSessionMetaFromContext(context.Background()).SessionID)
}
