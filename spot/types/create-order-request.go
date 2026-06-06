/*
FILE: spot/types/create-order-request.go

DESCRIPTION:
Place-order request for the KuCoin Spot profile, consumed by
TradingClient.PlaceOrder (POST /api/v1/orders) and the batch variant.

The struct is a clean, typed surface; the trading sub-client maps it onto
the KuCoin wire body, applying the clientOid default when left empty.

REQUIRED:
  - Symbol, Side, Type.
  - LIMIT: Price and Size (base currency).
  - MARKET: EXACTLY ONE of Size (base) or Funds (quote).

SIZING: spot Size/Funds are decimals in base/quote currency (NOT contracts).
Round Size to SymbolInfo.BaseIncrement, Price to SymbolInfo.PriceIncrement
and Funds to SymbolInfo.QuoteIncrement before placing.
*/

package types

import "github.com/shopspring/decimal"

// CreateOrderRequest — typed spot place-order request.
type CreateOrderRequest struct {
	Symbol string
	Side   SideType
	Type   OrderType

	// Price — limit price; ignored for market orders.
	Price decimal.Decimal
	// Size — order size in BASE currency. Required for limit orders; for
	// market orders set EITHER Size OR Funds.
	Size decimal.Decimal
	// Funds — market-order value in QUOTE currency. Market only; mutually
	// exclusive with Size.
	Funds decimal.Decimal

	// ClientOrderID — idempotency key; auto-generated when empty.
	ClientOrderID string
	// TimeInForce — GTC/IOC/GTT/FOK; empty lets KuCoin default (GTC).
	TimeInForce TimeInForceType
	// TradeType — TRADE (spot, default) or MARGIN_TRADE.
	TradeType TradeType

	// Execution flags (limit orders).
	PostOnly    bool
	Hidden      bool
	Iceberg     bool
	VisibleSize decimal.Decimal
	// CancelAfter — seconds until auto-cancel; only with TimeInForce GTT.
	CancelAfter int64

	// STP — self-trade prevention; empty for none.
	STP SelfTradePrevention

	// Remark — free-form note (<= 100 chars).
	Remark string
}
