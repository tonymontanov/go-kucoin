/*
FILE: margin/types/fill.go

DESCRIPTION:
Execution / fill row for the KuCoin Margin profile, mapped from
GET /api/v3/hf/margin/fills.

Size is in BASE currency; funds and fee are in QUOTE / fee currency.
*/

package types

import "github.com/shopspring/decimal"

// Fill — one margin execution row.
type Fill struct {
	TradeID string
	OrderID string
	Symbol  string
	Side    SideType

	Price decimal.Decimal
	// Size — filled size (base currency).
	Size decimal.Decimal
	// Funds — filled value (quote currency).
	Funds decimal.Decimal

	// Liquidity — "maker"/"taker".
	Liquidity string
	// OrderType — order type that produced the fill.
	OrderType OrderType
	// TradeType — MARGIN_TRADE / MARGIN_ISOLATED_TRADE.
	TradeType TradeType
	// ForceTaker — true when KuCoin charged taker fees regardless of flags.
	ForceTaker bool

	// Fee / FeeRate / FeeCurrency.
	Fee         decimal.Decimal
	FeeRate     decimal.Decimal
	FeeCurrency string

	CreatedAtMs int64
}
