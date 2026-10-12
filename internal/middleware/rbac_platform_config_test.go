package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// v2.9 平台统一配置收口（PRD：模型 / 数据与扩展 / 检索配置写权限收归
// 系统管理员）。本文件锁定两件事：
//  1. RequireSystemAdmin 对「空间角色再高」的普通用户一律 403，仅
//     IsSystemAdmin 放行（AdminOrSystemAdmin 语义不再适用于这些路由）；
//  2. 平台 KV 键（retrieval-config 等）在 UpdateTenantKV 的 key 级判定
//     下，空间 Admin 被拒、系统管理员放行（handler 侧矩阵见
//     internal/handler 的平台键测试）。

// sysAdminHarness mirrors rbacTestHarness but lets the caller decide both
// the tenant role and the platform-level IsSystemAdmin flag — the two
// identity axes are independent (see stores/auth.ts 双身份轴).
func sysAdminHarness(role types.TenantRole, isSystemAdmin bool, mw gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, role)
		if isSystemAdmin {
			ctx = context.WithValue(ctx, types.SystemAdminContextKey, true)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/protected", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	r.ServeHTTP(w, req)
	return w
}

// TestRequireSystemAdmin_PlatformConfigMatrix locks the v2.9 guard choice
// for every platform-managed config write route family: 空间 Owner/Admin
// 一律 403（哪怕角色比 Viewer 高），SystemAdmin 放行。
func TestRequireSystemAdmin_PlatformConfigMatrix(t *testing.T) {
	mw := RequireSystemAdmin(cfgRBAC(true))

	cases := []struct {
		name          string
		role          types.TenantRole
		isSystemAdmin bool
		wantCode      int
	}{
		{"space owner is rejected", types.TenantRoleOwner, false, http.StatusForbidden},
		{"space admin is rejected", types.TenantRoleAdmin, false, http.StatusForbidden},
		{"space viewer is rejected", types.TenantRoleViewer, false, http.StatusForbidden},
		{"system admin passes regardless of tenant role", types.TenantRoleViewer, true, http.StatusOK},
		{"system admin with owner role passes", types.TenantRoleOwner, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := sysAdminHarness(tc.role, tc.isSystemAdmin, mw)
			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

// TestRequireRoleOrSystemAdmin_AdminStillPasses documents the contrast:
// the AdminOrSystemAdmin guard (kept for KV PUT route entry) still admits
// space Admins — the platform-key restriction happens at handler level.
func TestRequireRoleOrSystemAdmin_AdminStillPasses(t *testing.T) {
	mw := RequireRoleOrSystemAdmin(types.TenantRoleAdmin, cfgRBAC(true))
	w := sysAdminHarness(types.TenantRoleAdmin, false, mw)
	assert.Equal(t, http.StatusOK, w.Code)
}
