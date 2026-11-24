package mok

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The quick amount buttons of the recharge page. Upstream hard-codes them in
// frontend/src/views/user/PaymentView.vue (10, 20, 50, 100, 200, 500, 1000, 2000, 5000);
// a [mok] seam there asks the fork for the admin's list instead and keeps upstream's
// buttons while none is set. The page still hides amounts outside the payment methods'
// minimum and maximum, as upstream does.

// paymentQuickAmountsKey is the settings key the list is stored under, as a JSON array.
const paymentQuickAmountsKey = "mok_payment_quick_amounts"

const (
	paymentQuickAmountsMax      = 12
	paymentQuickAmountMaxValue  = 1_000_000
	paymentQuickAmountsDecimals = 2
)

func errPaymentQuickAmountsInvalid(message string) error {
	return infraerrors.BadRequest("MOK_PAYMENT_QUICK_AMOUNTS_INVALID", message)
}

// PaymentQuickAmounts is the stored list; empty means upstream's buttons.
type PaymentQuickAmounts struct {
	Amounts []float64 `json:"amounts"`
}

// PaymentQuickAmountsService reads and writes the list.
type PaymentQuickAmountsService struct {
	settings service.SettingRepository
}

func NewPaymentQuickAmountsService(settings service.SettingRepository) *PaymentQuickAmountsService {
	return &PaymentQuickAmountsService{settings: settings}
}

// Get returns the admin's list, or an empty one when upstream's buttons apply. A stored
// value that no longer validates counts as unset.
func (s *PaymentQuickAmountsService) Get(ctx context.Context) (PaymentQuickAmounts, error) {
	raw, err := s.settings.GetValue(ctx, paymentQuickAmountsKey)
	if errors.Is(err, service.ErrSettingNotFound) || (err == nil && strings.TrimSpace(raw) == "") {
		return PaymentQuickAmounts{Amounts: []float64{}}, nil
	}
	if err != nil {
		return PaymentQuickAmounts{}, err
	}
	var stored []float64
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return PaymentQuickAmounts{Amounts: []float64{}}, nil
	}
	amounts, err := normalizePaymentQuickAmounts(stored)
	if err != nil {
		return PaymentQuickAmounts{Amounts: []float64{}}, nil
	}
	return PaymentQuickAmounts{Amounts: amounts}, nil
}

// Set validates and stores the list; an empty list restores upstream's buttons.
func (s *PaymentQuickAmountsService) Set(ctx context.Context, amounts []float64) (PaymentQuickAmounts, error) {
	normalized, err := normalizePaymentQuickAmounts(amounts)
	if err != nil {
		return PaymentQuickAmounts{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return PaymentQuickAmounts{}, err
	}
	if err := s.settings.Set(ctx, paymentQuickAmountsKey, string(raw)); err != nil {
		return PaymentQuickAmounts{}, err
	}
	return PaymentQuickAmounts{Amounts: normalized}, nil
}

// normalizePaymentQuickAmounts checks every amount and returns them deduplicated and in
// ascending order, the order the page shows them in.
func normalizePaymentQuickAmounts(amounts []float64) ([]float64, error) {
	out := make([]float64, 0, len(amounts))
	seen := map[float64]bool{}
	scale := math.Pow10(paymentQuickAmountsDecimals)
	for _, amount := range amounts {
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount > paymentQuickAmountMaxValue {
			return nil, errPaymentQuickAmountsInvalid("each amount must be greater than 0 and at most 1000000")
		}
		rounded := math.Round(amount*scale) / scale
		if math.Abs(rounded-amount) > 1e-9 {
			return nil, errPaymentQuickAmountsInvalid("amounts have at most 2 decimal places")
		}
		if !seen[rounded] {
			seen[rounded] = true
			out = append(out, rounded)
		}
	}
	if len(out) > paymentQuickAmountsMax {
		return nil, errPaymentQuickAmountsInvalid("at most 12 amounts")
	}
	sort.Float64s(out)
	return out, nil
}

// registerPaymentQuickAmountsRoutes serves the list:
//
//	GET /api/v1/admin/mok/payment/quick-amounts   the stored list
//	PUT /api/v1/admin/mok/payment/quick-amounts   {"amounts": [30, 50, 100]}; [] restores upstream's buttons
//	GET /api/v1/mok/payment/quick-amounts         the list for the recharge page
func registerPaymentQuickAmountsRoutes(admin, user *gin.RouterGroup, quickAmounts *PaymentQuickAmountsService) {
	get := func(c *gin.Context) {
		result, err := quickAmounts.Get(c.Request.Context())
		if response.ErrorFrom(c, err) {
			return
		}
		response.Success(c, result)
	}
	admin.GET("/payment/quick-amounts", get)
	user.GET("/payment/quick-amounts", get)
	admin.PUT("/payment/quick-amounts", func(c *gin.Context) {
		var req struct {
			Amounts *[]float64 `json:"amounts" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
		result, err := quickAmounts.Set(c.Request.Context(), *req.Amounts)
		if response.ErrorFrom(c, err) {
			return
		}
		response.Success(c, result)
	})
}
