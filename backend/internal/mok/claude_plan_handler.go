package mok

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
)

func registerClaudePlanRoutes(admin *gin.RouterGroup, svc *ClaudePlanService) {
	h := &claudePlanHandler{svc: svc}
	admin.GET("/claude-plan-tiers", h.list)
}

type claudePlanHandler struct {
	svc *ClaudePlanService
}

// list handles GET /api/v1/mok/admin/claude-plan-tiers?ids=1,2,3[&refresh=true].
func (h *claudePlanHandler) list(c *gin.Context) {
	ids, err := parseClaudePlanIDs(c.Query("ids"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	refresh, _ := strconv.ParseBool(c.Query("refresh"))
	items, err := h.svc.Get(c.Request.Context(), ids, refresh)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items})
}

// parseClaudePlanIDs parses a comma-separated list of account IDs, dropping duplicates.
func parseClaudePlanIDs(raw string) ([]int64, error) {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid account id %q", part)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("ids is required")
	}
	if len(ids) > claudePlanMaxIDs {
		return nil, fmt.Errorf("at most %d ids per request", claudePlanMaxIDs)
	}
	return ids, nil
}
