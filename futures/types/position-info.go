/*
FILE: futures/types/position-info.go

DESCRIPTION:
Position state for the KuCoin Futures profile, mapped from
GET /api/v1/position?symbol= and GET /api/v1/positions, and pushed on the
private WS "/contract/position:{symbol}" channel.

KuCoin reports a net position per contract (CurrentQty signed: positive =
long, negative = short). Money fields are in the settle currency.
*/

package types

import "github.com/shopspring/decimal"

// PositionInfo — KuCoin Futures position state.
type PositionInfo struct {
	Symbol         string
	SettleCurrency string

	// IsOpen — true while the position holds a non-zero quantity.
	IsOpen bool
	// CrossMode — true for cross margin, false for isolated.
	CrossMode bool
	// MarginMode — derived enum form of CrossMode.
	MarginMode MarginMode

	// CurrentQty — signed position size in contracts (long > 0, short < 0).
	CurrentQty decimal.Decimal
	// CurrentQtyKnown — true when the source frame actually carried
	// currentQty. The WS /contract/position channel emits position.change
	// frames driven by MARK PRICE (and settlement) that do NOT include
	// currentQty; for those CurrentQtyKnown is false and CurrentQty stays at
	// its zero value. Consumers MUST check this before treating CurrentQty
	// (and IsOpen) as authoritative — otherwise a mark-price tick looks like
	// a flat position. REST snapshots always set it true.
	CurrentQtyKnown bool
	// AvgEntryPrice — average entry price.
	AvgEntryPrice decimal.Decimal
	// MarkPrice / MarkValue — current mark price and position mark value.
	MarkPrice decimal.Decimal
	MarkValue decimal.Decimal
	// LiquidationPrice / BankruptPrice.
	LiquidationPrice decimal.Decimal
	BankruptPrice    decimal.Decimal

	// RealLeverage — effective leverage of the position.
	RealLeverage decimal.Decimal
	// PosMargin / PosCost — position margin and cost.
	PosMargin decimal.Decimal
	PosCost   decimal.Decimal
	// MaintMarginReq — maintenance margin requirement (rate).
	MaintMarginReq decimal.Decimal
	// RiskLimit — current risk-limit level.
	RiskLimit decimal.Decimal

	// UnrealizedPnL / UnrealizedPnLPct.
	UnrealizedPnL    decimal.Decimal
	UnrealizedPnLPct decimal.Decimal
	// RealizedPnL — cumulative realised PnL.
	RealizedPnL decimal.Decimal

	OpeningTimestampMs int64
}
