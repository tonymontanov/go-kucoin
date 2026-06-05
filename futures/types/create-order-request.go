/*
FILE: futures/types/create-order-request.go

DESCRIPTION:
Place-order request for the KuCoin Futures profile, consumed by
TradingClient.PlaceOrder (POST /api/v1/orders) and the batch variant.

The struct is a clean, typed surface; the trading sub-client maps it onto
the KuCoin wire body, applying client defaults (leverage / margin mode /
clientOid) where fields are left empty.

REQUIRED:
  - Symbol, Side, Type.
  - Size (contracts) for both limit and market orders.
  - Price for limit orders.
  - Leverage (here or via the client default).

STOP / CONDITIONAL:
Set Stop ("up"/"down") together with StopPriceType and StopPrice to place a
conditional order on POST /api/v1/orders.
*/

package types

import "github.com/shopspring/decimal"

// CreateOrderRequest — typed place-order request.
type CreateOrderRequest struct {
	Symbol string
	Side   SideType
	Type   OrderType

	// Size — order size in CONTRACTS (integer count).
	Size int64
	// Price — limit price; ignored for market orders.
	Price decimal.Decimal

	// Leverage — per-order leverage as a string (e.g. "5"). Empty falls
	// back to the client default; if both are empty PlaceOrder errors.
	Leverage string
	// ClientOrderID — idempotency key; auto-generated when empty.
	ClientOrderID string
	// TimeInForce — GTC/IOC; empty lets KuCoin default per order type.
	TimeInForce TimeInForceType
	// MarginMode — ISOLATED/CROSS; empty falls back to the client default.
	MarginMode MarginMode

	// Execution flags.
	PostOnly    bool
	Hidden      bool
	Iceberg     bool
	VisibleSize decimal.Decimal
	ReduceOnly  bool
	CloseOrder  bool
	ForceHold   bool

	// Remark — free-form note (<= 100 chars).
	Remark string

	// Stop / conditional.
	Stop          StopType
	StopPriceType StopPriceType
	StopPrice     decimal.Decimal
}
