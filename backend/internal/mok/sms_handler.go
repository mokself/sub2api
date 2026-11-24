package mok

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
)

const smsLogsMaxPageSize = 100

// registerSMSRoutes serves the SMS channel (sms_service.go):
//
//	GET    /api/v1/admin/mok/sms/config         the SMS card; the secret key is never returned
//	PUT    /api/v1/admin/mok/sms/config         save the SMS card; an empty secret_key keeps the stored one
//	GET    /api/v1/admin/mok/sms/ops-alert      the SMS part of the ops alert settings
//	PUT    /api/v1/admin/mok/sms/ops-alert      {"enabled", "phones", "min_level"}
//	GET    /api/v1/admin/mok/sms/account-quota  the SMS phones of the account quota alerts
//	PUT    /api/v1/admin/mok/sms/account-quota  {"phones": [{"phone", "disabled"}]}
//	POST   /api/v1/admin/mok/sms/test     {"phone", "event"}: send one template with sample values
//	GET    /api/v1/admin/mok/sms/logs     the send log; page, page_size, search
//	GET    /api/v1/mok/sms/phone          the signed-in user's bound phone and whether SMS is available
//	POST   /api/v1/mok/sms/phone/code     {"phone"}: send a verification code
//	POST   /api/v1/mok/sms/phone          {"phone", "code"}: bind the number
//	PUT    /api/v1/mok/sms/phone          {"enabled"}: pause or resume the reminders
//	DELETE /api/v1/mok/sms/phone          unbind the number
func registerSMSRoutes(admin, user *gin.RouterGroup, sms *SMSService) {
	h := &smsHandler{sms: sms}
	admin.GET("/sms/config", h.getConfig)
	admin.PUT("/sms/config", h.putConfig)
	admin.GET("/sms/ops-alert", h.getOpsAlert)
	admin.PUT("/sms/ops-alert", h.putOpsAlert)
	admin.GET("/sms/account-quota", h.getAccountQuota)
	admin.PUT("/sms/account-quota", h.putAccountQuota)
	admin.POST("/sms/test", h.test)
	admin.GET("/sms/logs", h.logs)
	user.GET("/sms/phone", h.phone)
	user.POST("/sms/phone/code", h.sendCode)
	user.POST("/sms/phone", h.bind)
	user.PUT("/sms/phone", h.setEnabled)
	user.DELETE("/sms/phone", h.remove)
}

type smsHandler struct {
	sms *SMSService
}

func (h *smsHandler) getConfig(c *gin.Context) {
	cfg, err := h.sms.AdminConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, cfg)
}

func (h *smsHandler) putConfig(c *gin.Context) {
	var req SMSConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.sms.SaveConfig(c.Request.Context(), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, cfg)
}

func (h *smsHandler) getOpsAlert(c *gin.Context) {
	ops, err := h.sms.AdminOpsAlert(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, ops)
}

func (h *smsHandler) putOpsAlert(c *gin.Context) {
	var req SMSOpsAlert
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ops, err := h.sms.SaveOpsAlert(c.Request.Context(), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, ops)
}

func (h *smsHandler) getAccountQuota(c *gin.Context) {
	quota, err := h.sms.AdminAccountQuota(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, quota)
}

func (h *smsHandler) putAccountQuota(c *gin.Context) {
	var req SMSAccountQuota
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	quota, err := h.sms.SaveAccountQuota(c.Request.Context(), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, quota)
}

func (h *smsHandler) test(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Event string `json:"event" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.sms.TestSend(c.Request.Context(), req.Phone, req.Event)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *smsHandler) logs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > smsLogsMaxPageSize {
		pageSize = smsLogsMaxPageSize
	}
	search := c.Query("search")
	if len([]rune(search)) > 100 {
		search = string([]rune(search)[:100])
	}
	entries, total, err := h.sms.ListLogs(c.Request.Context(), page, pageSize, search)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, entries, total, page, pageSize)
}

func smsUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func (h *smsHandler) phone(c *gin.Context) {
	userID, ok := smsUserID(c)
	if !ok {
		return
	}
	status, err := h.sms.PhoneStatus(c.Request.Context(), userID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *smsHandler) sendCode(c *gin.Context) {
	userID, ok := smsUserID(c)
	if !ok {
		return
	}
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	sent, err := h.sms.SendBindCode(c.Request.Context(), userID, req.Phone)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, sent)
}

func (h *smsHandler) bind(c *gin.Context) {
	userID, ok := smsUserID(c)
	if !ok {
		return
	}
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	status, err := h.sms.BindPhone(c.Request.Context(), userID, req.Phone, req.Code)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *smsHandler) setEnabled(c *gin.Context) {
	userID, ok := smsUserID(c)
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	status, err := h.sms.SetPhoneEnabled(c.Request.Context(), userID, *req.Enabled)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *smsHandler) remove(c *gin.Context) {
	userID, ok := smsUserID(c)
	if !ok {
		return
	}
	status, err := h.sms.RemovePhone(c.Request.Context(), userID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}
