package mok

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const affiliateSelectedUsersMaxPageSize = 100

// registerAffiliateScopeRoutes serves the affiliate scope, the feature for selected users
// only (service/affiliate_scope_mok.go):
//
//	GET  /api/v1/mok/admin/affiliate-scope            scope and number of selected users
//	PUT  /api/v1/mok/admin/affiliate-scope            {"scope": "all" | "selected"}
//	GET  /api/v1/mok/admin/affiliate-users            the selected users; page, page_size
//	GET  /api/v1/mok/admin/affiliate-users/:user_id   one user's switch
//	PUT  /api/v1/mok/admin/affiliate-users/:user_id   {"selected": true | false}
//	GET  /api/v1/mok/affiliate/access                 whether the feature is on for the signed-in user,
//	                                                  and whether they still have rebate quota if not
func registerAffiliateScopeRoutes(admin, user *gin.RouterGroup, affiliates *service.AffiliateService) {
	h := &affiliateScopeHandler{affiliates: affiliates}
	admin.GET("/affiliate-scope", h.getScope)
	admin.PUT("/affiliate-scope", h.putScope)
	admin.GET("/affiliate-users", h.listUsers)
	admin.GET("/affiliate-users/:user_id", h.getUser)
	admin.PUT("/affiliate-users/:user_id", h.putUser)
	user.GET("/affiliate/access", h.access)
}

type affiliateScopeHandler struct {
	affiliates *service.AffiliateService
}

func (h *affiliateScopeHandler) getScope(c *gin.Context) {
	status, err := h.affiliates.MokAffiliateScopeStatus(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *affiliateScopeHandler) putScope(c *gin.Context) {
	var req struct {
		Scope string `json:"scope" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	status, err := h.affiliates.MokSetAffiliateScope(c.Request.Context(), req.Scope)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *affiliateScopeHandler) listUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > affiliateSelectedUsersMaxPageSize {
		pageSize = affiliateSelectedUsersMaxPageSize
	}
	users, total, err := h.affiliates.MokListAffiliateSelectedUsers(c.Request.Context(), page, pageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, users, total, page, pageSize)
}

func affiliateUserIDParam(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return 0, false
	}
	return userID, true
}

func (h *affiliateScopeHandler) getUser(c *gin.Context) {
	userID, ok := affiliateUserIDParam(c)
	if !ok {
		return
	}
	access, err := h.affiliates.MokAffiliateUserAccess(c.Request.Context(), userID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, access)
}

func (h *affiliateScopeHandler) putUser(c *gin.Context) {
	userID, ok := affiliateUserIDParam(c)
	if !ok {
		return
	}
	var req struct {
		Selected *bool `json:"selected" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	access, err := h.affiliates.MokSetAffiliateUserSelected(c.Request.Context(), userID, *req.Selected)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, access)
}

func (h *affiliateScopeHandler) access(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	access, err := h.affiliates.MokAffiliateOwnAccess(c.Request.Context(), subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, access)
}
