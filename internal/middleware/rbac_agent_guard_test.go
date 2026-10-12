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

// v2.13 业务资产治理收口：智能体管理（创建/编辑/删除/复制）收归空间
// Admin+，普通成员（contributor/viewer）只读+使用。本文件锁定 Admin 守卫
// 对智能体写路由的语义：contributor 被拒、admin/owner 放行。
// （路由侧 routes_agent.go 的 agentsWrite 组已从 Contributor/OwnedAgentOrAdmin
// 改为 g.Admin()；OwnedAgentOrAdmin 的 creator 分支不再覆盖智能体写路径。）

func agentGuardHarness(role types.TenantRole, mw gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, role)
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

func TestAgentAdminGuard_Matrix(t *testing.T) {
	mw := RequireRole(types.TenantRoleAdmin, cfgRBAC(true))

	cases := []struct {
		name     string
		role     types.TenantRole
		wantCode int
	}{
		{"space viewer cannot manage agents", types.TenantRoleViewer, http.StatusForbidden},
		{"space contributor cannot manage agents", types.TenantRoleContributor, http.StatusForbidden},
		{"space admin manages agents", types.TenantRoleAdmin, http.StatusOK},
		{"space owner manages agents", types.TenantRoleOwner, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := agentGuardHarness(tc.role, mw)
			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}
