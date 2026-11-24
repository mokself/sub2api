// Package mok holds this fork's own features. Nothing in it exists upstream; see
// FORK.md for the conventions that keep upstream merges conflict-free.
package mok

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"

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
	service.NewMokAffiliateUsageRebateService,
	NewSMSStore,
	NewTencentSMSSender,
	NewSMSService,
	NewPaymentQuickAmountsService,
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

// ProvideHTTPServer wraps server.ProvideHTTPServer. Taking the Module makes wire build the
// fork module, and thereby register its routes, before the server. The fork's background
// jobs stop when main shuts the server down gracefully.
func ProvideHTTPServer(cfg *config.Config, router *gin.Engine, m *Module) *http.Server {
	srv := server.ProvideHTTPServer(cfg, router)
	srv.RegisterOnShutdown(m.Stop)
	return srv
}

// Module is the fork's root object. Building it registers the fork's HTTP routes on
// the router upstream has already set up and starts the fork's background jobs.
type Module struct {
	ClaudePlan           *ClaudePlanService
	AffiliateUsageRebate *service.MokAffiliateUsageRebateService
	SMS                  *SMSService
}

// Stop ends the fork's background jobs.
func (m *Module) Stop() {
	if m != nil {
		m.AffiliateUsageRebate.Stop()
		m.SMS.Stop()
	}
}

// The fork's HTTP paths. Admin routes sit under upstream's /api/v1/admin because upstream
// authorizes its admin API keys by path: a full-access key may call /api/v1/admin/... only
// (adminaccess.RouteAllowed). User routes have no admin counterpart to follow.
const (
	adminBasePath = "/api/v1/admin/mok"
	userBasePath  = "/api/v1/mok"
)

// routeDeps are the services the fork's handlers use.
type routeDeps struct {
	settings            *service.SettingService
	affiliates          *service.AffiliateService
	claudePlan          *ClaudePlanService
	usageRebate         *service.MokAffiliateUsageRebateService
	sms                 *SMSService
	paymentQuickAmounts *PaymentQuickAmountsService
}

// registerRoutes registers every fork route on the two groups.
func registerRoutes(admin, user *gin.RouterGroup, d routeDeps) {
	registerClaudePlanRoutes(admin, d.claudePlan)
	registerAffiliateTierRoutes(admin, user, d.settings, d.affiliates)
	registerAffiliateModeRoutes(admin, user, d.usageRebate)
	registerAffiliateScopeRoutes(admin, user, d.affiliates)
	registerSMSRoutes(admin, user, d.sms)
	registerPaymentQuickAmountsRoutes(admin, user, d.paymentQuickAmounts)
}

// NewModule registers the fork routes. Admin routes get the same guards as upstream's
// /api/v1/admin group plus a refusal of restricted admin API keys; user routes get the
// guards of upstream's authenticated user routes.
func NewModule(
	r *gin.Engine,
	adminAuth middleware.AdminAuthMiddleware,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	affiliateService *service.AffiliateService,
	redisClient *redis.Client,
	claudePlan *ClaudePlanService,
	affiliateUsageRebate *service.MokAffiliateUsageRebateService,
	sms *SMSService,
	paymentQuickAmounts *PaymentQuickAmountsService,
) *Module {
	panelRateLimiter := middleware.NewPanelRateLimiter(redisClient, settingService)

	admin := r.Group(adminBasePath)
	admin.Use(
		gin.HandlerFunc(adminAuth),
		panelRateLimiter.Global(),
		gin.HandlerFunc(auditLog),
		middleware.AdminComplianceGuard(settingService),
		rejectRestrictedAdminKeys,
	)

	user := r.Group(userBasePath)
	user.Use(
		gin.HandlerFunc(jwtAuth),
		middleware.BackendModeUserGuard(settingService),
		panelRateLimiter.Global(),
		gin.HandlerFunc(auditLog),
	)

	registerRoutes(admin, user, routeDeps{
		settings:            settingService,
		affiliates:          affiliateService,
		claudePlan:          claudePlan,
		usageRebate:         affiliateUsageRebate,
		sms:                 sms,
		paymentQuickAmounts: paymentQuickAmounts,
	})

	affiliateUsageRebate.Start()
	// The SMS channel of upstream's notifications, see service/sms_notify_mok.go.
	service.MokSetSMSNotifier(sms)
	sms.Start()
	return &Module{ClaudePlan: claudePlan, AffiliateUsageRebate: affiliateUsageRebate, SMS: sms}
}

// rejectRestrictedAdminKeys keeps restricted admin API keys out; full-access keys, the
// legacy admin key and admin logins pass. Upstream's adminAuth already refuses restricted
// keys here, since its permission catalog lists no fork route; this check stays in case
// that catalog ever matches one. Fork routes take raw IDs and do not implement upstream's
// per-account scope checks.
func rejectRestrictedAdminKeys(c *gin.Context) {
	if adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		middleware.AbortWithError(c, http.StatusForbidden, "MOK_RESTRICTED_ADMIN_KEY", "restricted admin API keys cannot use this endpoint")
		return
	}
	c.Next()
}
