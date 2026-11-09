package mok

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// registerAffiliateTierRoutes serves the tiered affiliate rebate settings
// (service/affiliate_service_mok.go):
//
//	GET  /api/v1/mok/admin/affiliate-rebate-tiers  current tiers
//	PUT  /api/v1/mok/admin/affiliate-rebate-tiers  replace tiers; an empty list disables them
//	GET  /api/v1/mok/affiliate/rebate-tiers        the signed-in user's progress through them
func registerAffiliateTierRoutes(admin, user *gin.RouterGroup, settings *service.SettingService, affiliates *service.AffiliateService) {
	h := &affiliateTierHandler{settings: settings, affiliates: affiliates}
	admin.GET("/affiliate-rebate-tiers", h.adminGet)
	admin.PUT("/affiliate-rebate-tiers", h.adminPut)
	user.GET("/affiliate/rebate-tiers", h.userStatus)
}

type affiliateTierHandler struct {
	settings   *service.SettingService
	affiliates *service.AffiliateService
}

type affiliateTiersPayload struct {
	Tiers []service.MokAffiliateRebateTier `json:"tiers"`
}

func (h *affiliateTierHandler) adminGet(c *gin.Context) {
	tiers, err := h.settings.MokGetAffiliateRebateTiers(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	if tiers == nil {
		tiers = []service.MokAffiliateRebateTier{}
	}
	response.Success(c, affiliateTiersPayload{Tiers: tiers})
}

func (h *affiliateTierHandler) adminPut(c *gin.Context) {
	var req affiliateTiersPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tiers, err := h.settings.MokSetAffiliateRebateTiers(c.Request.Context(), req.Tiers)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, affiliateTiersPayload{Tiers: tiers})
}

func (h *affiliateTierHandler) userStatus(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.affiliates.MokAffiliateTierStatus(c.Request.Context(), subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}
