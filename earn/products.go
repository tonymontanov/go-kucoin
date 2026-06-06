/*
FILE: earn/products.go

DESCRIPTION:
Earn product-catalogue reads. The five product-list endpoints share the same
query (optional currency) and the same array row shape, so they funnel through
one private helper and one wire→type converter.

ENDPOINTS:
  - GET /api/v1/earn/saving/products
  - GET /api/v1/earn/promotion/products
  - GET /api/v1/earn/staking/products
  - GET /api/v1/earn/kcs-staking/products
  - GET /api/v1/earn/eth-staking/products
*/

package earn

import (
	"context"

	"github.com/shopspring/decimal"

	earntypes "github.com/tonymontanov/go-kucoin/v2/earn/types"
)

// GetSavingsProducts returns the available savings (demand) products. Pass an
// empty currency for all.
func (c *Client) GetSavingsProducts(ctx context.Context, currency string) ([]earntypes.Product, error) {
	return c.getProducts(ctx, "/api/v1/earn/saving/products", currency)
}

// GetPromotionProducts returns the available promotion products.
func (c *Client) GetPromotionProducts(ctx context.Context, currency string) ([]earntypes.Product, error) {
	return c.getProducts(ctx, "/api/v1/earn/promotion/products", currency)
}

// GetStakingProducts returns the available staking products.
func (c *Client) GetStakingProducts(ctx context.Context, currency string) ([]earntypes.Product, error) {
	return c.getProducts(ctx, "/api/v1/earn/staking/products", currency)
}

// GetKCSStakingProducts returns the available KCS-staking products.
func (c *Client) GetKCSStakingProducts(ctx context.Context, currency string) ([]earntypes.Product, error) {
	return c.getProducts(ctx, "/api/v1/earn/kcs-staking/products", currency)
}

// GetETHStakingProducts returns the available ETH-staking products.
func (c *Client) GetETHStakingProducts(ctx context.Context, currency string) ([]earntypes.Product, error) {
	return c.getProducts(ctx, "/api/v1/earn/eth-staking/products", currency)
}

// getProducts is the shared product-list fetch + convert.
func (c *Client) getProducts(ctx context.Context, path, currency string) ([]earntypes.Product, error) {
	var query map[string]string
	if currency != "" {
		query = map[string]string{"currency": currency}
	}
	var wire []productWire
	if err := c.doGET(ctx, path, query, &wire); err != nil {
		return nil, err
	}
	var out []earntypes.Product = make([]earntypes.Product, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toProduct()
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Wire struct + converter.
// ---------------------------------------------------------------------

// productWire mirrors one product row. lockEndTime / applyEndTime ship as null
// for open-ended products — int64 decodes null to 0.
type productWire struct {
	ID                   string          `json:"id"`
	Currency             string          `json:"currency"`
	Category             string          `json:"category"`
	Type                 string          `json:"type"`
	Precision            int             `json:"precision"`
	ProductUpperLimit    decimal.Decimal `json:"productUpperLimit"`
	ProductRemainAmount  decimal.Decimal `json:"productRemainAmount"`
	UserUpperLimit       decimal.Decimal `json:"userUpperLimit"`
	UserLowerLimit       decimal.Decimal `json:"userLowerLimit"`
	ReturnRate           decimal.Decimal `json:"returnRate"`
	IncomeCurrency       string          `json:"incomeCurrency"`
	RedeemPeriod         int             `json:"redeemPeriod"`
	LockStartTime        int64           `json:"lockStartTime"`
	LockEndTime          int64           `json:"lockEndTime"`
	ApplyStartTime       int64           `json:"applyStartTime"`
	ApplyEndTime         int64           `json:"applyEndTime"`
	Duration             int             `json:"duration"`
	EarlyRedeemSupported int             `json:"earlyRedeemSupported"`
	Status               string          `json:"status"`
	RedeemType           string          `json:"redeemType"`
	IncomeReleaseType    string          `json:"incomeReleaseType"`
	InterestDate         int64           `json:"interestDate"`
	NewUserOnly          int             `json:"newUserOnly"`
}

func (w productWire) toProduct() earntypes.Product {
	return earntypes.Product{
		ID:                   w.ID,
		Currency:             w.Currency,
		Category:             w.Category,
		Type:                 w.Type,
		Precision:            w.Precision,
		ProductUpperLimit:    w.ProductUpperLimit,
		ProductRemainAmount:  w.ProductRemainAmount,
		UserUpperLimit:       w.UserUpperLimit,
		UserLowerLimit:       w.UserLowerLimit,
		ReturnRate:           w.ReturnRate,
		IncomeCurrency:       w.IncomeCurrency,
		RedeemPeriod:         w.RedeemPeriod,
		LockStartTime:        w.LockStartTime,
		LockEndTime:          w.LockEndTime,
		ApplyStartTime:       w.ApplyStartTime,
		ApplyEndTime:         w.ApplyEndTime,
		Duration:             w.Duration,
		EarlyRedeemSupported: w.EarlyRedeemSupported,
		Status:               w.Status,
		RedeemType:           w.RedeemType,
		IncomeReleaseType:    w.IncomeReleaseType,
		InterestDate:         w.InterestDate,
		NewUserOnly:          w.NewUserOnly,
	}
}
