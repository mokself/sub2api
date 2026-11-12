package mok

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// registerAffiliateModeRoutes serves the affiliate rebate mode and the usage rebate records
// (service/affiliate_usage_rebate_mok.go):
//
//	GET  /api/v1/mok/admin/affiliate-rebate-mode    mode, usage window and recent settled days
//	PUT  /api/v1/mok/admin/affiliate-rebate-mode    {"mode": "recharge" | "usage"}
//	GET  /api/v1/mok/admin/affiliate-usage-rebates  every settled record; page, page_size, search
//	GET  /api/v1/mok/affiliate/usage-rebates        the signed-in inviter's records; page, page_size
func registerAffiliateModeRoutes(admin, user *gin.RouterGroup, usageRebate *service.MokAffiliateUsageRebateService) {
	h := &affiliateModeHandler{usageRebate: usageRebate}
	admin.GET("/affiliate-rebate-mode", h.get)
	admin.PUT("/affiliate-rebate-mode", h.put)
	admin.GET("/affiliate-usage-rebates", h.adminRecords)
	user.GET("/affiliate/usage-rebates", h.userRecords)
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

func (h *affiliateModeHandler) adminRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.NormalizeMokAffiliateUsageRebateFilter(service.MokAffiliateUsageRebateFilter{
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	})
	records, total, err := h.usageRebate.ListRecords(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, records, total, filter.Page, filter.PageSize)
}

func (h *affiliateModeHandler) userRecords(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	filter := service.NormalizeMokAffiliateUsageRebateFilter(service.MokAffiliateUsageRebateFilter{Page: page, PageSize: pageSize})
	records, total, err := h.usageRebate.ListRecordsForInviter(c.Request.Context(), subject.UserID, filter.Page, filter.PageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, records, total, filter.Page, filter.PageSize)
}
