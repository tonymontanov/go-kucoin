/*
FILE: futures/types/fill.go

DESCRIPTION:
Execution / fill row for the KuCoin Futures profile, mapped from
GET /api/v1/fills and GET /api/v1/recentFills.

Sizes are in CONTRACTS; value and fee are in the settle currency.
*/

package types

import "github.com/shopspring/decimal"

// Fill — one execution row.
type Fill struct {
	TradeID string
	OrderID string
	Symbol  string
	Side    SideType

	Price decimal.Decimal
	Size  decimal.Decimal
	Value decimal.Decimal

	// Liquidity — maker/taker flag.
	Liquidity Liquidity
	// OrderType — order type that produced the fill.
	OrderType OrderType
	// TradeType — trade context (trade / liquidation / ADL / settlement).
	TradeType TradeType

	// Fee / FeeRate / FeeCurrency.
	Fee         decimal.Decimal
	FeeRate     decimal.Decimal
	FixFee      decimal.Decimal
	FeeCurrency string

	SettleCurrency string
	TradeTimeMs    int64
	CreatedAtMs    int64
}
