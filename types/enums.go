/*
FILE: types/enums.go

DESCRIPTION:
Closed enums (typed strings) shared across KuCoin profiles. Values match
the wire format KuCoin accepts/returns on the CLASSIC API — keep them
exact, or the exchange rejects the request.

Profile packages re-export these via type aliases (see futures/types).
Profile-specific values encoded as the same enum type are added by the
profile package as constants of the aliased type — no separate enum is
introduced.

WIRE NOTES (KuCoin Futures classic):
  - side / type are lower-case ("buy"/"sell", "limit"/"market");
  - timeInForce is UPPER-case ("GTC"/"IOC"); post-only / hidden / iceberg
    are SEPARATE boolean flags on the order, not timeInForce values, so
    they are NOT part of TimeInForceType;
  - order status is reported as "open"/"done" on REST and additionally as
    lifecycle subjects ("received"/"open"/"match"/"filled"/"canceled") on
    the private WS channel — unknown values are surfaced verbatim.
*/

package types

// SideType — order direction on the wire. KuCoin uses lower-case.
type SideType string

const (
	// SideTypeBuy — buy / long.
	SideTypeBuy SideType = "buy"
	// SideTypeSell — sell / short.
	SideTypeSell SideType = "sell"
)

// OrderType — execution model on the wire. KuCoin uses lower-case.
type OrderType string

const (
	// OrderTypeLimit — limit order.
	OrderTypeLimit OrderType = "limit"
	// OrderTypeMarket — market order.
	OrderTypeMarket OrderType = "market"
)

// TimeInForceType — order expiry / queue behaviour. KuCoin uses UPPER-case
// on the wire. post-only / hidden / iceberg are independent boolean flags
// (see CreateOrderRequest in futures/types), NOT members of this enum.
type TimeInForceType string

const (
	// TimeInForceGTC — Good Till Cancel (default for limit).
	TimeInForceGTC TimeInForceType = "GTC"
	// TimeInForceIOC — Immediate Or Cancel (default for market).
	TimeInForceIOC TimeInForceType = "IOC"
)

// OrderStatus — base order states. KuCoin reports "open"/"done" on REST;
// the private WS channel adds lifecycle subjects. Values outside the
// well-known set are returned verbatim in OrderInfo.Status.
type OrderStatus string

const (
	// OrderStatusOpen — accepted by the matcher and resting in the book.
	OrderStatusOpen OrderStatus = "open"
	// OrderStatusDone — terminal (fully filled or cancelled). Inspect the
	// filled size / cancel fields to disambiguate.
	OrderStatusDone OrderStatus = "done"
	// OrderStatusMatch — a match event (private WS lifecycle subject).
	OrderStatusMatch OrderStatus = "match"
	// OrderStatusFilled — fully filled (private WS lifecycle subject).
	OrderStatusFilled OrderStatus = "filled"
	// OrderStatusCanceled — cancelled (private WS lifecycle subject).
	OrderStatusCanceled OrderStatus = "canceled"
)

// MarginMode — per-symbol margining mode. KuCoin Futures uses these
// lower-case strings on the set-margin-mode and position endpoints.
type MarginMode string

const (
	// MarginModeIsolated — isolated margin (per-symbol).
	MarginModeIsolated MarginMode = "ISOLATED"
	// MarginModeCross — crossed margin.
	MarginModeCross MarginMode = "CROSS"
)

// StopType — trigger condition for stop / conditional orders. KuCoin
// Futures encodes it as the "stop" field ("up"/"down") together with a
// stopPriceType and stopPrice.
type StopType string

const (
	// StopTypeUp — trigger when the price rises to/through stopPrice.
	StopTypeUp StopType = "up"
	// StopTypeDown — trigger when the price falls to/through stopPrice.
	StopTypeDown StopType = "down"
)

// StopPriceType — reference price a stop order triggers against.
type StopPriceType string

const (
	// StopPriceTypeTrade — last trade price ("TP").
	StopPriceTypeTrade StopPriceType = "TP"
	// StopPriceTypeIndex — index price ("IP").
	StopPriceTypeIndex StopPriceType = "IP"
	// StopPriceTypeMark — mark price ("MP").
	StopPriceTypeMark StopPriceType = "MP"
)
