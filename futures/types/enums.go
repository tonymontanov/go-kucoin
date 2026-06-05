/*
FILE: futures/types/enums.go

DESCRIPTION:
Enum surface for the KuCoin Futures profile. Layer-1 enums are re-exported
as type aliases so callers depend on a single package
(github.com/tonymontanov/go-kucoin/v2/futures/types); futures-only enums
that the classic API encodes are defined here.
*/

package types

import roottypes "github.com/tonymontanov/go-kucoin/v2/types"

// ---- Layer-1 alias re-exports --------------------------------------------

// SideType — order direction ("buy"/"sell").
type SideType = roottypes.SideType

// OrderType — execution model ("limit"/"market").
type OrderType = roottypes.OrderType

// TimeInForceType — order expiry ("GTC"/"IOC").
type TimeInForceType = roottypes.TimeInForceType

// OrderStatus — order state.
type OrderStatus = roottypes.OrderStatus

// MarginMode — per-symbol margin mode ("ISOLATED"/"CROSS").
type MarginMode = roottypes.MarginMode

// StopType — stop trigger direction ("up"/"down").
type StopType = roottypes.StopType

// StopPriceType — stop reference price ("TP"/"IP"/"MP").
type StopPriceType = roottypes.StopPriceType

// Timeframe — kline granularity.
type Timeframe = roottypes.Timeframe

// Re-exported constants (so callers need not import the root types pkg).
const (
	// SideBuy — buy / long.
	SideBuy = roottypes.SideTypeBuy
	// SideSell — sell / short.
	SideSell = roottypes.SideTypeSell

	// OrderLimit — limit order.
	OrderLimit = roottypes.OrderTypeLimit
	// OrderMarket — market order.
	OrderMarket = roottypes.OrderTypeMarket

	// GTC — Good Till Cancel.
	GTC = roottypes.TimeInForceGTC
	// IOC — Immediate Or Cancel.
	IOC = roottypes.TimeInForceIOC

	// MarginIsolated — isolated margin.
	MarginIsolated = roottypes.MarginModeIsolated
	// MarginCross — crossed margin.
	MarginCross = roottypes.MarginModeCross

	// StopUp / StopDown — stop trigger directions.
	StopUp   = roottypes.StopTypeUp
	StopDown = roottypes.StopTypeDown

	// StopPriceTrade / StopPriceIndex / StopPriceMark — stop references.
	StopPriceTrade = roottypes.StopPriceTypeTrade
	StopPriceIndex = roottypes.StopPriceTypeIndex
	StopPriceMark  = roottypes.StopPriceTypeMark

	// Timeframe constants (minute-based Wire()).
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

// ---- Futures-only enums ---------------------------------------------------

// PositionSide — position direction for hedge/one-way semantics. KuCoin
// classic uses "BOTH" for one-way mode; "LONG"/"SHORT" address the two legs
// in hedge mode.
type PositionSide string

const (
	// PositionSideBoth — one-way (net) position. Classic default.
	PositionSideBoth PositionSide = "BOTH"
	// PositionSideLong — long leg (hedge mode).
	PositionSideLong PositionSide = "LONG"
	// PositionSideShort — short leg (hedge mode).
	PositionSideShort PositionSide = "SHORT"
)

// TradeType — fill trade context as reported on /api/v1/fills.
type TradeType string

const (
	// TradeTypeTrade — normal matched trade.
	TradeTypeTrade TradeType = "trade"
	// TradeTypeLiquidation — forced-liquidation fill.
	TradeTypeLiquidation TradeType = "liquidation"
	// TradeTypeADL — auto-deleveraging fill.
	TradeTypeADL TradeType = "ADL"
	// TradeTypeSettlement — settlement fill.
	TradeTypeSettlement TradeType = "settlement"
)

// Liquidity — maker/taker flag on a fill.
type Liquidity string

const (
	// LiquidityMaker — the fill added liquidity.
	LiquidityMaker Liquidity = "maker"
	// LiquidityTaker — the fill removed liquidity.
	LiquidityTaker Liquidity = "taker"
)
