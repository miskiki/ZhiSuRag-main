package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSystemUserAdminRouteTree_NoConflicts guards the v2.19 账号管理 route
// family: under /system/admin/users* the same HTTP method tree mixes static
// segments (POST /users/create, POST /users/reset-password) with param
// segments (POST /users/:user_id/force-logout; GET /users vs GET
// /users/:user_id). gin v1.12 supports this coexistence, but a gin
// downgrade or a future path reshuffle must fail HERE as a test instead of
// panicking at server startup (gin panics on tree conflicts at registration
// time — exactly what this test reproduces).
func TestSystemUserAdminRouteTree_NoConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/system/admin")
	noop := func(c *gin.Context) {}

	// Exact registration order/shape of routes_auth_tenant.go's
	// RegisterSystemAdminRoutes for the users family.
	admin.POST("/users/create", noop)
	admin.POST("/users/reset-password", noop)
	admin.GET("/users", noop)
	admin.GET("/users/:user_id", noop)
	admin.PUT("/users/:user_id", noop)
	admin.PUT("/users/:user_id/status", noop)
	admin.POST("/users/:user_id/force-logout", noop)
	admin.GET("/users/:user_id/sessions", noop)
	admin.DELETE("/users/:user_id/sessions/:session_id", noop)
	admin.PUT("/users/:user_id/memberships/:tenant_id", noop)
	admin.DELETE("/users/:user_id/memberships/:tenant_id", noop)

	// Routing sanity: every declared path resolves on the engine.
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	want := []string{
		"POST /system/admin/users/create",
		"POST /system/admin/users/reset-password",
		"GET /system/admin/users",
		"GET /system/admin/users/:user_id",
		"PUT /system/admin/users/:user_id",
		"PUT /system/admin/users/:user_id/status",
		"POST /system/admin/users/:user_id/force-logout",
		"GET /system/admin/users/:user_id/sessions",
		"DELETE /system/admin/users/:user_id/sessions/:session_id",
		"PUT /system/admin/users/:user_id/memberships/:tenant_id",
		"DELETE /system/admin/users/:user_id/memberships/:tenant_id",
	}
	for _, w := range want {
		if !registered[w] {
			t.Fatalf("route missing after registration: %s", w)
		}
	}
}
