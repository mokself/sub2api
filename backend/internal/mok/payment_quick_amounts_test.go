//go:build unit

package mok

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizePaymentQuickAmounts(t *testing.T) {
	got, err := normalizePaymentQuickAmounts([]float64{100, 30, 9.9, 30, 0.01, 1000000})
	require.NoError(t, err)
	require.Equal(t, []float64{0.01, 9.9, 30, 100, 1000000}, got, "deduplicated, ascending")

	got, err = normalizePaymentQuickAmounts(nil)
	require.NoError(t, err)
	require.Equal(t, []float64{}, got)

	for name, amounts := range map[string][]float64{
		"zero":      {0},
		"negative":  {-5},
		"too big":   {1000000.01},
		"decimals":  {9.999},
		"not a num": {math.NaN()},
		"infinite":  {math.Inf(1)},
		"too many":  {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
	} {
		_, err := normalizePaymentQuickAmounts(amounts)
		require.Error(t, err, name)
		require.Contains(t, err.Error(), "MOK_PAYMENT_QUICK_AMOUNTS_INVALID", name)
	}
	_, err = normalizePaymentQuickAmounts([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 12})
	require.NoError(t, err, "duplicates do not count towards the limit")
}

func TestPaymentQuickAmountsService(t *testing.T) {
	ctx := context.Background()
	settings := newMemorySettings()
	svc := NewPaymentQuickAmountsService(settings)

	got, err := svc.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, []float64{}, got.Amounts, "unset: upstream's buttons")

	saved, err := svc.Set(ctx, []float64{200, 50, 88})
	require.NoError(t, err)
	require.Equal(t, []float64{50, 88, 200}, saved.Amounts)
	require.Equal(t, "[50,88,200]", settings.data[paymentQuickAmountsKey])
	got, err = svc.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, []float64{50, 88, 200}, got.Amounts)

	_, err = svc.Set(ctx, []float64{-1})
	require.Error(t, err)
	got, _ = svc.Get(ctx)
	require.Equal(t, []float64{50, 88, 200}, got.Amounts, "a rejected list leaves the stored one")

	_, err = svc.Set(ctx, []float64{})
	require.NoError(t, err)
	got, _ = svc.Get(ctx)
	require.Equal(t, []float64{}, got.Amounts, "an empty list restores upstream's buttons")

	settings.data[paymentQuickAmountsKey] = "not json"
	got, err = svc.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, []float64{}, got.Amounts)
}

func TestPaymentQuickAmountsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := NewPaymentQuickAmountsService(newMemorySettings())
	registerPaymentQuickAmountsRoutes(r.Group("/admin"), r.Group("/user"), svc)
	do := func(method, path, body string) (int, map[string]any) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	code, _ := do(http.MethodPut, "/admin/payment/quick-amounts", `{}`)
	require.Equal(t, http.StatusBadRequest, code, "amounts is required; [] restores the defaults")
	code, _ = do(http.MethodPut, "/admin/payment/quick-amounts", `{"amounts":[0]}`)
	require.Equal(t, http.StatusBadRequest, code)

	code, body := do(http.MethodPut, "/admin/payment/quick-amounts", `{"amounts":[100,30]}`)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []any{30.0, 100.0}, body["data"].(map[string]any)["amounts"])

	code, body = do(http.MethodGet, "/user/payment/quick-amounts", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []any{30.0, 100.0}, body["data"].(map[string]any)["amounts"])
}
