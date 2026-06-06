/*
FILE: margin/types/create-order-request.go

DESCRIPTION:
Place-order request for the KuCoin Margin profile, consumed by
TradingClient.PlaceOrder (POST /api/v3/hf/margin/order).

The struct is a clean, typed surface; the trading sub-client maps it onto
the KuCoin HF-margin wire body, applying the clientOid default when empty.

REQUIRED:
  - Symbol, Side, Type.
  - LIMIT: Price and Size (base currency).
  - MARKET: EXACTLY ONE of Size (base) or Funds (quote).

MARGIN SPECIFICS:
  - IsIsolated selects the isolated account (default cross). It must agree
    with TradeType when both are set; leave TradeType empty to derive it.
  - AutoBorrow lets KuCoin borrow the shortfall at the lowest market rate
    when the account balance is insufficient (sell needs Size; buy needs
    Funds).
  - AutoRepay returns borrowed assets when the position is closed.

SIZING: margin Size/Funds are decimals in base/quote currency (NOT
contracts). Round Size to SymbolInfo.BaseIncrement, Price to
SymbolInfo.PriceIncrement and Funds to SymbolInfo.QuoteIncrement first.
*/

package types

import "github.com/shopspring/decimal"

// CreateOrderRequest — typed margin place-order request.
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

	// IsIsolated — true targets the isolated-margin account; false (default)
	// targets cross margin.
	IsIsolated bool
	// AutoBorrow — borrow the shortfall automatically at the lowest market
	// rate when the balance is insufficient.
	AutoBorrow bool
	// AutoRepay — repay borrowed assets automatically on the filled amount.
	AutoRepay bool

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
