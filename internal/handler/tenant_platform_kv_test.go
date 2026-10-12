package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v2.9 平台统一配置收口：UpdateTenantKV 的平台键（retrieval-config /
// parser-engine-config / storage-engine-config / web-search-config）在
// key 级判定下仅 SystemAdmin 可写；空间 Admin（哪怕通过
// AdminOrSystemAdmin 路由层）一律 403；非平台键 chat-history-config
// 维持空间 Admin 可写不变。

// platformKVEngine mirrors newTenantHandlerTestEngine but exposes the
// platform-level IsSystemAdmin flag as a second, independent identity axis.
func platformKVEngine(t *testing.T, role types.TenantRole, isSystemAdmin bool, tenant *types.Tenant) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &TenantHandler{service: &stubTenantService{tenant: tenant}}

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant.ID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
		if isSystemAdmin {
			ctx = context.WithValue(ctx, types.SystemAdminContextKey, true)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenant.ID)
		c.Next()
	})
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)
	return r
}

func platformKVFixture() *types.Tenant {
	return &types.Tenant{ID: 42, Name: "platform-kv-test"}
}

func TestUpdateTenantKV_PlatformKeysRejectedForSpaceAdmin(t *testing.T) {
	tenant := platformKVFixture()
	engine := platformKVEngine(t, types.TenantRoleAdmin, false, tenant)

	for _, key := range []string{"retrieval-config", "parser-engine-config", "storage-engine-config", "web-search-config"} {
		t.Run(key, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/tenants/kv/"+key, strings.NewReader(`{}`))
			engine.ServeHTTP(rec, req)
			require.Equal(t, http.StatusForbidden, rec.Code, "space admin must NOT write platform key %s", key)
			assert.Contains(t, rec.Body.String(), "system administrator")
		})
	}
}

func TestUpdateTenantKV_PlatformKeysAllowedForSystemAdmin(t *testing.T) {
	tenant := platformKVFixture()
	engine := platformKVEngine(t, types.TenantRoleViewer, true, tenant)

	// 系统管理员（无空间角色亦可）写检索配置：有效负载走完 handler 正常落库。
	body := `{"embedding_top_k":50,"vector_threshold":0.15,"keyword_threshold":0.3,"rerank_top_k":10,"rerank_threshold":0.2,"rerank_model_id":""}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/retrieval-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "system admin must write platform key retrieval-config")

	// parser/storage 两键的正文校验属于各自 handler 的职责；此处断言
	// 系统管理员已通过全部权限门（不再 403）。
	for _, key := range []string{"parser-engine-config", "storage-engine-config"} {
		t.Run(key, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/tenants/kv/"+key, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"system admin must pass both gates for %s", key)
		})
	}
}

func TestUpdateTenantKV_NonPlatformKeysUnchangedForSpaceAdmin(t *testing.T) {
	tenant := platformKVFixture()
	engine := platformKVEngine(t, types.TenantRoleAdmin, false, tenant)

	// 非平台键维持空间 Admin 可写（v2.9 不收紧）：消息管理开关照常。
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/chat-history-config", strings.NewReader(`{"enabled":true}`))
	engine.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusForbidden, rec.Code,
		"chat-history-config must stay writable by space admin (v2.9 scope keeps it)")
}
