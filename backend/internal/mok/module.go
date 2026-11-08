// Package mok holds this fork's own features. Nothing in it exists upstream; see
// FORK.md for the conventions that keep upstream merges conflict-free.
package mok

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ProviderSet builds the fork module.
var ProviderSet = wire.NewSet(
	NewClaudePlanStore,
	NewClaudePlanService,
	NewModule,
)

// ServerProviderSet stands in for server.ProviderSet in cmd/server/wire.go, where the
// [mok] line reads "server.ProvideRouter, mok.ServerProviderSet". It provides the HTTP
// server through ProvideHTTPServer, which needs the fork module, so wire builds the
// module and registers the fork routes without touching upstream's Application struct.
// If upstream adds a provider to server.ProviderSet, add it to that line as well; wire
// reports the missing provider when wire_gen.go is regenerated.
var ServerProviderSet = wire.NewSet(
	ProvideHTTPServer,
	ProviderSet,
)

// ProvideHTTPServer wraps server.ProvideHTTPServer. The Module parameter only makes
// wire build the fork module, and thereby register its routes, before the server.
func ProvideHTTPServer(cfg *config.Config, router *gin.Engine, _ *Module) *http.Server {
	return server.ProvideHTTPServer(cfg, router)
}

// Module is the fork's root object. Building it registers the fork's HTTP routes on
// the router upstream has already set up.
type Module struct {
	ClaudePlan *ClaudePlanService
}

// NewModule registers the fork routes. Admin routes get the same guards as upstream's
// /api/v1/admin group, plus a refusal of restricted admin API keys.
func NewModule(
	r *gin.Engine,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	claudePlan *ClaudePlanService,
) *Module {
	admin := r.Group("/api/v1/mok/admin")
	admin.Use(
		gin.HandlerFunc(adminAuth),
		gin.HandlerFunc(auditLog),
		middleware.AdminComplianceGuard(settingService),
		rejectRestrictedAdminKeys,
	)
	registerClaudePlanRoutes(admin, claudePlan)
	return &Module{ClaudePlan: claudePlan}
}

// rejectRestrictedAdminKeys keeps scoped admin API keys out. Fork routes take raw
// account IDs and do not implement upstream's per-account scope checks.
func rejectRestrictedAdminKeys(c *gin.Context) {
	if adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		middleware.AbortWithError(c, http.StatusForbidden, "MOK_RESTRICTED_ADMIN_KEY", "restricted admin API keys cannot use this endpoint")
		return
	}
	c.Next()
}
