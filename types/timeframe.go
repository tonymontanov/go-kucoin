/*
FILE: types/timeframe.go

DESCRIPTION:
KuCoin Futures kline `granularity` enum — protocol-common across profiles.

Unlike Bitget (named "1m"/"1H" strings), KuCoin Futures expresses the kline
granularity as an INTEGER NUMBER OF MINUTES on both REST
(GET /api/v1/kline/query?granularity=) and the WS candle topic
(/contractMarket/candle:{symbol}_{granularity}). The allowed set is fixed:

	1, 5, 15, 30, 60, 120, 240, 480, 720, 1440, 10080

This enum keeps a human-readable identifier ("1m", "1h", …) while Wire()
returns the minute count as a string and Minutes() the raw integer.
*/

package types

import "strconv"

// Timeframe is a closed enum of kline intervals supported by KuCoin Futures.
type Timeframe string

const (
	// Timeframe1m — 1 minute.
	Timeframe1m Timeframe = "1m"
	// Timeframe5m — 5 minutes.
	Timeframe5m Timeframe = "5m"
	// Timeframe15m — 15 minutes.
	Timeframe15m Timeframe = "15m"
	// Timeframe30m — 30 minutes.
	Timeframe30m Timeframe = "30m"
	// Timeframe1h — 1 hour (60m).
	Timeframe1h Timeframe = "1h"
	// Timeframe2h — 2 hours (120m).
	Timeframe2h Timeframe = "2h"
	// Timeframe4h — 4 hours (240m).
	Timeframe4h Timeframe = "4h"
	// Timeframe8h — 8 hours (480m).
	Timeframe8h Timeframe = "8h"
	// Timeframe12h — 12 hours (720m).
	Timeframe12h Timeframe = "12h"
	// Timeframe1d — 1 day (1440m).
	Timeframe1d Timeframe = "1d"
	// Timeframe1w — 1 week (10080m).
	Timeframe1w Timeframe = "1w"
)

// timeframeMinutes maps each Timeframe to the integer granularity KuCoin
// expects. Unknown timeframes map to 0.
var timeframeMinutes = map[Timeframe]int{
	Timeframe1m:  1,
	Timeframe5m:  5,
	Timeframe15m: 15,
	Timeframe30m: 30,
	Timeframe1h:  60,
	Timeframe2h:  120,
	Timeframe4h:  240,
	Timeframe8h:  480,
	Timeframe12h: 720,
	Timeframe1d:  1440,
	Timeframe1w:  10080,
}

// Minutes returns the integer granularity in minutes (0 for an unknown
// timeframe).
func (t Timeframe) Minutes() int {
	return timeframeMinutes[t]
}

// Wire returns the KuCoin granularity string (minute count). Empty for an
// unknown timeframe so callers can validate before sending.
func (t Timeframe) Wire() string {
	var m int = timeframeMinutes[t]
	if m == 0 {
		return ""
	}
	return strconv.Itoa(m)
}
