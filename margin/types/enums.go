/*
FILE: margin/types/enums.go

DESCRIPTION:
Enum surface for the KuCoin Margin profile (v2.5). Layer-1 enums are
re-exported as type aliases so callers depend on a single package
(github.com/tonymontanov/go-kucoin/v2/margin/types); margin-only enums are
defined here.

Margin trades on KuCoin's SPOT matching engine, so the order model mirrors
Spot (limit/market, price+size or funds, GTC/GTT/IOC/FOK, postOnly, STP). The
margin-specific axis is the TradeType: cross ("MARGIN_TRADE") vs isolated
("MARGIN_ISOLATED_TRADE").
*/

package types

import roottypes "github.com/tonymontanov/go-kucoin/v2/types"

// ---- Layer-1 alias re-exports --------------------------------------------

// SideType — order direction ("buy"/"sell").
type SideType = roottypes.SideType

// OrderType — execution model ("limit"/"market").
type OrderType = roottypes.OrderType

// TimeInForceType — order expiry ("GTC"/"IOC"/"GTT"/"FOK").
type TimeInForceType = roottypes.TimeInForceType

// OrderStatus — order state.
type OrderStatus = roottypes.OrderStatus

// Re-exported constants (so callers need not import the root types pkg).
const (
	// SideBuy — buy.
	SideBuy = roottypes.SideTypeBuy
	// SideSell — sell.
	SideSell = roottypes.SideTypeSell

	// OrderLimit — limit order.
	OrderLimit = roottypes.OrderTypeLimit
	// OrderMarket — market order.
	OrderMarket = roottypes.OrderTypeMarket

	// GTC — Good Till Cancel.
	GTC = roottypes.TimeInForceGTC
	// IOC — Immediate Or Cancel.
	IOC = roottypes.TimeInForceIOC
)

// ---- Margin time-in-force (extra values) ---------------------------------

// KuCoin Margin supports two extra time-in-force values on top of GTC/IOC.
const (
	// GTT — Good Till Time. Pair with CreateOrderRequest.CancelAfter
	// (seconds) to auto-cancel the resting remainder.
	GTT TimeInForceType = "GTT"
	// FOK — Fill Or Kill. The whole order must fill immediately or it is
	// rejected.
	FOK TimeInForceType = "FOK"
)

// ---- Self-trade prevention ----------------------------------------------

// SelfTradePrevention selects how KuCoin resolves an order that would match
// the account's own resting order. Empty means no STP.
type SelfTradePrevention string

const (
	// STPCancelNewest — "CN": cancel the incoming (taker) order.
	STPCancelNewest SelfTradePrevention = "CN"
	// STPCancelOldest — "CO": cancel the resting (maker) order.
	STPCancelOldest SelfTradePrevention = "CO"
	// STPCancelBoth — "CB": cancel both orders.
	STPCancelBoth SelfTradePrevention = "CB"
	// STPDecreaseAndCancel — "DC": decrease the larger and cancel the
	// smaller. Only valid for limit orders.
	STPDecreaseAndCancel SelfTradePrevention = "DC"
)

// ---- Trade type (cross vs isolated) --------------------------------------

// TradeType selects the margin account an order/query targets. KuCoin Margin
// encodes it as "MARGIN_TRADE" (cross) or "MARGIN_ISOLATED_TRADE" (isolated).
// The HF margin order endpoints require it on every order query/cancel-all.
type TradeType string

const (
	// TradeCross — cross-margin trading account ("MARGIN_TRADE"). Default.
	TradeCross TradeType = "MARGIN_TRADE"
	// TradeIsolated — isolated-margin trading account
	// ("MARGIN_ISOLATED_TRADE").
	TradeIsolated TradeType = "MARGIN_ISOLATED_TRADE"
)

// isIsolated reports whether the trade type targets the isolated account.
func (t TradeType) IsIsolated() bool { return t == TradeIsolated }

// ---- Account query type (cross/isolated, V2 phase-out) -------------------

// QueryType selects the account variant for the margin account endpoints.
// KuCoin's HF migration unified the semantics: "MARGIN" == HF cross,
// "ISOLATED" == HF isolated. The *_V2 forms mean the same thing and are being
// phased out — the SDK sends the preferred non-V2 forms.
type QueryType string

const (
	// QueryCross — HF cross-margin account ("MARGIN").
	QueryCross QueryType = "MARGIN"
	// QueryIsolated — HF isolated-margin account ("ISOLATED").
	QueryIsolated QueryType = "ISOLATED"
)
