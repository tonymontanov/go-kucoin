/*
FILE: copytrading/types/types.go

DESCRIPTION:
Types for the KuCoin futures Copy-Trading (lead-trader) profile, mapped from the
/api/v1/copy-trade/futures/* endpoints on the FUTURES host.

Sizes/prices/leverage are carried as strings (the SDK convention); KuCoin
accepts quoted numerics. PositionSide is "LONG" / "SHORT" / "BOTH".
*/

package types

import "github.com/shopspring/decimal"

// OrderRequest — body for a copy-trade futures order.
type OrderRequest struct {
	// ClientOid — unique client id. Required.
	ClientOid string
	// Symbol — contract, e.g. "XBTUSDTM". Required.
	Symbol string
	// MarginMode — "ISOLATED" / "CROSS". Copy-trading currently supports
	// ISOLATED only (CROSS is rejected with 180204).
	MarginMode string
	// Leverage — e.g. "12" (max 20x for copy-trading).
	Leverage string
	// PositionSide — "LONG" / "SHORT" / "BOTH".
	PositionSide string
	// Side — "buy" / "sell". Required.
	Side string
	// Type — "limit" / "market". Required.
	Type string
	// Size — order size in contracts. Required.
	Size string
	// Price — required for limit orders.
	Price string
	// TimeInForce — "GTC" / "IOC" (optional).
	TimeInForce string
	// ReduceOnly — close-only flag (optional).
	ReduceOnly bool
	// Remark — free-form note (optional).
	Remark string
}

// TPSLOrderRequest — body for a copy-trade order with take-profit / stop-loss.
type TPSLOrderRequest struct {
	OrderRequest
	// StopPriceType — "TP" / "IP" / "MP" (mark/index/last price basis).
	StopPriceType string
	// TriggerStopUpPrice — take-profit trigger price.
	TriggerStopUpPrice string
	// TriggerStopDownPrice — stop-loss trigger price.
	TriggerStopDownPrice string
}

// OrderResult — acknowledgement of a placed copy-trade order.
type OrderResult struct {
	OrderID   string
	ClientOid string
}

// CancelResult — result of cancelling by orderId.
type CancelResult struct {
	CancelledOrderIDs []string
}

// MaxOpenSize — maximum open size for a symbol at a price/leverage.
type MaxOpenSize struct {
	Symbol          string
	MaxBuyOpenSize  decimal.Decimal
	MaxSellOpenSize decimal.Decimal
}

// AddMarginRequest — body for adding isolated margin to a copy-trade position.
type AddMarginRequest struct {
	// Symbol — contract. Required.
	Symbol string
	// Margin — margin to add (units of settle currency). Required.
	Margin string
	// BizNo — idempotency key for the deposit. Required.
	BizNo string
	// PositionSide — "LONG" / "SHORT" / "BOTH" (optional).
	PositionSide string
}

// Position — a copy-trade futures position (returned by AddIsolatedMargin).
type Position struct {
	ID                string
	Symbol            string
	AutoDeposit       bool
	MaintMarginReq    decimal.Decimal
	RiskLimit         int64
	RealLeverage      decimal.Decimal
	CrossMode         bool
	MarginMode        string
	PositionSide      string
	Leverage          decimal.Decimal
	DelevPercentage   decimal.Decimal
	OpeningTimestamp  int64
	CurrentTimestamp  int64
	CurrentQty        decimal.Decimal
	CurrentCost       decimal.Decimal
	CurrentComm       decimal.Decimal
	UnrealisedCost    decimal.Decimal
	IsOpen            bool
	MarkPrice         decimal.Decimal
	MarkValue         decimal.Decimal
	PosCost           decimal.Decimal
	PosMargin         decimal.Decimal
	PosMaint          decimal.Decimal
	MaintMargin       decimal.Decimal
	RealisedPnl       decimal.Decimal
	UnrealisedPnl     decimal.Decimal
	UnrealisedPnlPcnt decimal.Decimal
	UnrealisedRoePcnt decimal.Decimal
	AvgEntryPrice     decimal.Decimal
	LiquidationPrice  decimal.Decimal
	BankruptPrice     decimal.Decimal
	SettleCurrency    string
}
