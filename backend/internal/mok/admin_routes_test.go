//go:build unit

package mok

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// adminKeyStore holds the one scoped admin key a test creates.
type adminKeyStore struct {
	service.ScopedAdminKeyRepository
	item *service.ScopedAdminKey
}

func (s *adminKeyStore) InsertScopedAdminKey(_ context.Context, item *service.ScopedAdminKey) error {
	s.item = item
	return nil
}

func (s *adminKeyStore) GetScopedAdminKey(context.Context, string) (*service.ScopedAdminKey, error) {
	return s.item, nil
}

func (s *adminKeyStore) TouchScopedAdminKey(context.Context, string, string, time.Time, string, time.Time) error {
	return nil
}

// adminKeyUsers knows the admin who created the keys.
type adminKeyUsers struct{ service.UserRepository }

func (*adminKeyUsers) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 9, Role: service.RoleAdmin, Status: service.StatusActive}, nil
}

func (*adminKeyUsers) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

// newAdminKeyAuth returns upstream's admin auth middleware and a scoped admin key it accepts.
func newAdminKeyAuth(t *testing.T, policy adminaccess.Policy) (middleware.AdminAuthMiddleware, string) {
	t.Helper()
	settings := service.NewSettingService(nil, nil)
	settings.SetScopedAdminKeyRepository(&adminKeyStore{})
	key, _, err := settings.CreateScopedAdminKey(context.Background(), service.ScopedAdminKeyInput{
		Name: "test", Enabled: true, ConfirmFullAccess: policy.FullAccess, Policy: policy,
	}, 9)
	require.NoError(t, err)
	return middleware.NewAdminAuthMiddleware(nil, service.NewUserService(&adminKeyUsers{}, nil, nil, nil), settings, nil), key
}

// forkRouter registers upstream's admin routes and the fork's routes on one engine, as the
// server does. Fork admin routes answer 204 once the guards pass, so no handler runs.
func forkRouter(t *testing.T, auth middleware.AdminAuthMiddleware) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	routes.RegisterAdminRoutes(r.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}}, auth,
		middleware.AuditLogMiddleware(pass), middleware.StepUpAuthMiddleware(pass), nil, nil)
	admin := r.Group(adminBasePath)
	admin.Use(gin.HandlerFunc(auth), rejectRestrictedAdminKeys, func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) })
	registerRoutes(admin, r.Group(userBasePath), routeDeps{})
	return r
}

func forkAdminRoutes(r *gin.Engine) []gin.RouteInfo {
	var out []gin.RouteInfo
	for _, route := range r.Routes() {
		if strings.HasPrefix(route.Path, adminBasePath+"/") {
			out = append(out, route)
		}
	}
	return out
}

func callWithKey(r *gin.Engine, method, path, key string) int {
	path = strings.ReplaceAll(path, ":user_id", "7")
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("x-api-key", key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestForkAdminRoutesAcceptFullAccessAdminKeys(t *testing.T) {
	auth, key := newAdminKeyAuth(t, adminaccess.Policy{FullAccess: true})
	r := forkRouter(t, auth)

	admin := forkAdminRoutes(r)
	require.NotEmpty(t, admin)
	for _, route := range admin {
		require.True(t, adminaccess.RouteAllowed(adminaccess.Policy{FullAccess: true}, route.Method, route.Path), route.Path)
		require.Equal(t, http.StatusNoContent, callWithKey(r, route.Method, route.Path, key), "%s %s", route.Method, route.Path)
		require.Equal(t, http.StatusUnauthorized, callWithKey(r, route.Method, route.Path, ""), "%s %s without a key", route.Method, route.Path)
	}
}

func TestForkAdminRoutesRefuseRestrictedAdminKeys(t *testing.T) {
	auth, key := newAdminKeyAuth(t, adminaccess.Policy{GlobalPermissions: []string{"openai.tokens.refresh"}})
	r := forkRouter(t, auth)

	for _, route := range forkAdminRoutes(r) {
		require.Equal(t, http.StatusForbidden, callWithKey(r, route.Method, route.Path, key), "%s %s", route.Method, route.Path)
	}
}

func TestForkRoutesLayout(t *testing.T) {
	auth, _ := newAdminKeyAuth(t, adminaccess.Policy{FullAccess: true})
	r := forkRouter(t, auth)

	var user int
	for _, route := range r.Routes() {
		require.False(t, strings.HasPrefix(route.Path, "/api/v1/mok/admin"), "the old admin prefix: %s", route.Path)
		if strings.HasPrefix(route.Path, userBasePath+"/") {
			user++
		}
	}
	require.NotZero(t, user)
}
