/*
FILE: types/kline-update.go

DESCRIPTION:
KlineUpdate is one event of the KuCoin Futures
"/contractMarket/candle:{symbol}_{granularity}" WebSocket channel —
protocol-common across profiles. KuCoin pushes the in-progress candle on
each trade and emits the next candle once the interval rolls over. KuCoin
does NOT ship an explicit "closed" flag, so Confirmed is computed by the
stream dispatcher at the boundary (a push whose StartMs advances past the
previous candle marks the previous one confirmed).

FIELDS:
  - Symbol   : KuCoin contract symbol.
  - Interval : Timeframe enum.
  - StartMs  : kline start timestamp (ms).
  - Open / High / Low / Close : OHLC.
  - Volume   : volume in contracts.
  - Confirmed: true once the candle has rolled over; false while forming.
*/

package types

import "github.com/shopspring/decimal"

// KlineUpdate — one event from the candle channel.
type KlineUpdate struct {
	Symbol    string
	Interval  Timeframe
	StartMs   int64
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
	Confirmed bool
}
