/*
FILE: futures/types/order-info.go

DESCRIPTION:
Order state for the KuCoin Futures profile, mapped from
GET /api/v1/orders/{orderId}, GET /api/v1/orders (list) and the private WS
"/contractMarket/tradeOrders" channel.

KuCoin reports terminal orders with Status == "done" and IsActive == false;
the filled vs. cancelled distinction is read from FilledSize / CancelExist.
Sizes are in CONTRACTS; values are in the settle currency.
*/

package types

import "github.com/shopspring/decimal"

// OrderInfo — KuCoin Futures order state.
type OrderInfo struct {
	OrderID       string
	ClientOrderID string
	Symbol        string
	Type          OrderType
	Side          SideType

	Price decimal.Decimal
	Size  decimal.Decimal
	Value decimal.Decimal

	// FilledSize / FilledValue — cumulative fills (dealSize / dealValue).
	FilledSize  decimal.Decimal
	FilledValue decimal.Decimal

	Leverage    decimal.Decimal
	TimeInForce TimeInForceType
	MarginMode  MarginMode

	PostOnly    bool
	Hidden      bool
	Iceberg     bool
	VisibleSize decimal.Decimal
	ReduceOnly  bool
	CloseOrder  bool
	ForceHold   bool

	// Stop / conditional fields.
	Stop          StopType
	StopPriceType StopPriceType
	StopPrice     decimal.Decimal
	StopTriggered bool

	// Status — "open"/"done"; IsActive/CancelExist disambiguate.
	Status      OrderStatus
	IsActive    bool
	CancelExist bool

	Remark         string
	SettleCurrency string

	CreatedAtMs int64
	UpdatedAtMs int64
}
