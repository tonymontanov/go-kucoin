/*
FILE: types/candle.go

DESCRIPTION:
Historical kline (candlestick) — protocol-common across KuCoin profiles.
Mapped from the array returned by GET /api/v1/kline/query (futures):

	[ startMs, open, high, low, close, volume ]

KuCoin Futures ships kline elements as JSON numbers; the SDK normalises
them into decimal.Decimal at the boundary. Klines are returned ASCENDING
by start time (oldest first); the SDK preserves that order.

KuCoin Futures klines do NOT carry a separate quote-volume field (unlike
Bitget), so Candle exposes a single Volume.
*/

package types

import "github.com/shopspring/decimal"

// Candle — one historical kline.
type Candle struct {
	OpenTimeMs int64
	Open       decimal.Decimal
	High       decimal.Decimal
	Low        decimal.Decimal
	Close      decimal.Decimal
	Volume     decimal.Decimal
}

// Candles — slice of candles, ascending by OpenTimeMs (KuCoin order).
type Candles []Candle
