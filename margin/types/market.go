/*
FILE: margin/types/market.go

DESCRIPTION:
Public market-data payloads specific to the KuCoin Margin profile:

  - MarkPrice    — index/mark price for a margin pair
                   (GET /api/v1/mark-price/{symbol}/current,
                    GET /api/v3/mark-price/all-symbols).
  - MarginConfig — global cross-margin configuration
                   (GET /api/v1/margin/config).

NB: the live order book / ticker / trade tape for margin pairs are IDENTICAL
to Spot (margin trades on the spot matching engine). Use the spot profile for
those; the margin profile only adds the margin-specific market data above.
*/

package types

import "github.com/shopspring/decimal"

// MarkPrice — mark price for a margin trading pair.
type MarkPrice struct {
	// Symbol — trading pair, e.g. "BTC-USDT".
	Symbol string
	// Value — current mark price.
	Value decimal.Decimal
	// TsMs — quote timestamp (ms).
	TsMs int64
}

// MarginConfig — global cross-margin configuration, mapped from
// GET /api/v1/margin/config.
type MarginConfig struct {
	// CurrencyList — currencies eligible for cross margin.
	CurrencyList []string
	// MaxLeverage — maximum account leverage.
	MaxLeverage int
	// WarningDebtRatio — debt ratio at which KuCoin warns.
	WarningDebtRatio decimal.Decimal
	// LiqDebtRatio — debt ratio at which forced liquidation triggers.
	LiqDebtRatio decimal.Decimal
}
