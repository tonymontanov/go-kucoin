/*
FILE: spot/types/enums.go

DESCRIPTION:
Enum surface for the KuCoin Spot profile. Layer-1 enums are re-exported as
type aliases so callers depend on a single package
(github.com/tonymontanov/go-kucoin/v2/spot/types); spot-only enums that the
classic API encodes (extra time-in-force values, self-trade prevention,
trade type) are defined here.
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

// Timeframe — kline granularity.
type Timeframe = roottypes.Timeframe

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

	// Timeframe constants. NB: the spot wire format differs from futures —
	// the spot profile maps these to "1min"/"1hour"/"1day"/… internally.
	Timeframe1m  = roottypes.Timeframe1m
	Timeframe5m  = roottypes.Timeframe5m
	Timeframe15m = roottypes.Timeframe15m
	Timeframe30m = roottypes.Timeframe30m
	Timeframe1h  = roottypes.Timeframe1h
	Timeframe2h  = roottypes.Timeframe2h
	Timeframe4h  = roottypes.Timeframe4h
	Timeframe8h  = roottypes.Timeframe8h
	Timeframe12h = roottypes.Timeframe12h
	Timeframe1d  = roottypes.Timeframe1d
	Timeframe1w  = roottypes.Timeframe1w
)

// ---- Spot-only time-in-force --------------------------------------------

// KuCoin Spot supports two extra time-in-force values on top of GTC/IOC.
// They are values of the layer-1 TimeInForceType so callers use one type.
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

// ---- Trade type ----------------------------------------------------------

// TradeType selects the trading account the order/query targets. KuCoin Spot
// encodes it as "TRADE" (spot) or "MARGIN_TRADE" (margin).
type TradeType string

const (
	// TradeSpot — spot trading account ("TRADE"). Default for this profile.
	TradeSpot TradeType = "TRADE"
	// TradeMargin — cross-margin trading account ("MARGIN_TRADE").
	TradeMargin TradeType = "MARGIN_TRADE"
)
