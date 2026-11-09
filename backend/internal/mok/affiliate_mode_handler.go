package mok

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// registerAffiliateModeRoutes serves the affiliate rebate mode
// (service/affiliate_usage_rebate_mok.go):
//
//	GET  /api/v1/mok/admin/affiliate-rebate-mode  mode, usage window and recent settled days
//	PUT  /api/v1/mok/admin/affiliate-rebate-mode  {"mode": "recharge" | "usage"}
func registerAffiliateModeRoutes(admin *gin.RouterGroup, usageRebate *service.MokAffiliateUsageRebateService) {
	h := &affiliateModeHandler{usageRebate: usageRebate}
	admin.GET("/affiliate-rebate-mode", h.get)
	admin.PUT("/affiliate-rebate-mode", h.put)
}

type affiliateModeHandler struct {
	usageRebate *service.MokAffiliateUsageRebateService
}

func (h *affiliateModeHandler) get(c *gin.Context) {
	status, err := h.usageRebate.ModeStatus(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *affiliateModeHandler) put(c *gin.Context) {
	var req struct {
		Mode string `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	status, err := h.usageRebate.SetMode(c.Request.Context(), req.Mode)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}
