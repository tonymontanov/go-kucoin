/*
FILE: margin/risk-limit.go

DESCRIPTION:
Signed risk-limit sub-client for the KuCoin Margin profile, wrapping
GET /api/v3/margin/currencies. The endpoint returns a cross shape (per
currency) or an isolated shape (per pair) depending on the isIsolated flag, so
this sub-client exposes one method per variant.
*/

package margin

import (
	"context"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// RiskLimitClient — signed risk-limit / borrow-config sub-client.
type RiskLimitClient struct {
	c *Client
}

// newRiskLimitClient wires the sub-client to its parent.
func newRiskLimitClient(c *Client) *RiskLimitClient {
	return &RiskLimitClient{c: c}
}

var riskLimitMeta = rest.RequestMeta{Category: "query"}

// GetCrossRiskLimit returns the cross-margin risk limit + borrow config. Pass
// an empty currency for every currency.
func (r *RiskLimitClient) GetCrossRiskLimit(ctx context.Context, currency string) ([]margintypes.CrossRiskLimit, error) {
	var query map[string]string = map[string]string{"isIsolated": "false"}
	if currency != "" {
		query["currency"] = currency
	}
	var wire []crossRiskLimitWire
	if err := r.c.doGET(ctx, true, "/api/v3/margin/currencies", query, riskLimitMeta, &wire); err != nil {
		return nil, err
	}
	var out []margintypes.CrossRiskLimit = make([]margintypes.CrossRiskLimit, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toCrossRiskLimit()
	}
	return out, nil
}

// GetIsolatedRiskLimit returns the isolated-margin risk limit + borrow config.
// Pass an empty symbol for every pair.
func (r *RiskLimitClient) GetIsolatedRiskLimit(ctx context.Context, symbol string) ([]margintypes.IsolatedRiskLimit, error) {
	var query map[string]string = map[string]string{"isIsolated": "true"}
	if symbol != "" {
		query["symbol"] = symbol
	}
	var wire []isolatedRiskLimitWire
	if err := r.c.doGET(ctx, true, "/api/v3/margin/currencies", query, riskLimitMeta, &wire); err != nil {
		return nil, err
	}
	var out []margintypes.IsolatedRiskLimit = make([]margintypes.IsolatedRiskLimit, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toIsolatedRiskLimit()
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type crossRiskLimitWire struct {
	Timestamp         int64           `json:"timestamp"`
	Currency          string          `json:"currency"`
	BorrowMaxAmount   decimal.Decimal `json:"borrowMaxAmount"`
	BuyMaxAmount      decimal.Decimal `json:"buyMaxAmount"`
	HoldMaxAmount     decimal.Decimal `json:"holdMaxAmount"`
	BorrowCoefficient decimal.Decimal `json:"borrowCoefficient"`
	MarginCoefficient decimal.Decimal `json:"marginCoefficient"`
	Precision         int             `json:"precision"`
	BorrowMinAmount   decimal.Decimal `json:"borrowMinAmount"`
	BorrowMinUnit     decimal.Decimal `json:"borrowMinUnit"`
	BorrowEnabled     bool            `json:"borrowEnabled"`
}

func (w crossRiskLimitWire) toCrossRiskLimit() margintypes.CrossRiskLimit {
	return margintypes.CrossRiskLimit{
		Currency:          w.Currency,
		BorrowMaxAmount:   w.BorrowMaxAmount,
		BuyMaxAmount:      w.BuyMaxAmount,
		HoldMaxAmount:     w.HoldMaxAmount,
		BorrowCoefficient: w.BorrowCoefficient,
		MarginCoefficient: w.MarginCoefficient,
		Precision:         w.Precision,
		BorrowMinAmount:   w.BorrowMinAmount,
		BorrowMinUnit:     w.BorrowMinUnit,
		BorrowEnabled:     w.BorrowEnabled,
		TsMs:              w.Timestamp,
	}
}

type isolatedRiskLimitWire struct {
	Timestamp              int64           `json:"timestamp"`
	Symbol                 string          `json:"symbol"`
	BaseMaxBorrowAmount    decimal.Decimal `json:"baseMaxBorrowAmount"`
	QuoteMaxBorrowAmount   decimal.Decimal `json:"quoteMaxBorrowAmount"`
	BaseMaxBuyAmount       decimal.Decimal `json:"baseMaxBuyAmount"`
	QuoteMaxBuyAmount      decimal.Decimal `json:"quoteMaxBuyAmount"`
	BaseMaxHoldAmount      decimal.Decimal `json:"baseMaxHoldAmount"`
	QuoteMaxHoldAmount     decimal.Decimal `json:"quoteMaxHoldAmount"`
	BasePrecision          int             `json:"basePrecision"`
	QuotePrecision         int             `json:"quotePrecision"`
	BaseBorrowCoefficient  decimal.Decimal `json:"baseBorrowCoefficient"`
	QuoteBorrowCoefficient decimal.Decimal `json:"quoteBorrowCoefficient"`
	BaseMarginCoefficient  decimal.Decimal `json:"baseMarginCoefficient"`
	QuoteMarginCoefficient decimal.Decimal `json:"quoteMarginCoefficient"`
	BaseBorrowMinAmount    decimal.Decimal `json:"baseBorrowMinAmount"`
	BaseBorrowMinUnit      decimal.Decimal `json:"baseBorrowMinUnit"`
	QuoteBorrowMinAmount   decimal.Decimal `json:"quoteBorrowMinAmount"`
	QuoteBorrowMinUnit     decimal.Decimal `json:"quoteBorrowMinUnit"`
	BaseBorrowEnabled      bool            `json:"baseBorrowEnabled"`
	QuoteBorrowEnabled     bool            `json:"quoteBorrowEnabled"`
}

func (w isolatedRiskLimitWire) toIsolatedRiskLimit() margintypes.IsolatedRiskLimit {
	return margintypes.IsolatedRiskLimit{
		Symbol:                 w.Symbol,
		BaseMaxBorrowAmount:    w.BaseMaxBorrowAmount,
		QuoteMaxBorrowAmount:   w.QuoteMaxBorrowAmount,
		BaseMaxBuyAmount:       w.BaseMaxBuyAmount,
		QuoteMaxBuyAmount:      w.QuoteMaxBuyAmount,
		BaseMaxHoldAmount:      w.BaseMaxHoldAmount,
		QuoteMaxHoldAmount:     w.QuoteMaxHoldAmount,
		BasePrecision:          w.BasePrecision,
		QuotePrecision:         w.QuotePrecision,
		BaseBorrowCoefficient:  w.BaseBorrowCoefficient,
		QuoteBorrowCoefficient: w.QuoteBorrowCoefficient,
		BaseMarginCoefficient:  w.BaseMarginCoefficient,
		QuoteMarginCoefficient: w.QuoteMarginCoefficient,
		BaseBorrowMinAmount:    w.BaseBorrowMinAmount,
		BaseBorrowMinUnit:      w.BaseBorrowMinUnit,
		QuoteBorrowMinAmount:   w.QuoteBorrowMinAmount,
		QuoteBorrowMinUnit:     w.QuoteBorrowMinUnit,
		BaseBorrowEnabled:      w.BaseBorrowEnabled,
		QuoteBorrowEnabled:     w.QuoteBorrowEnabled,
		TsMs:                   w.Timestamp,
	}
}
