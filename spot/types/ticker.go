/*
FILE: spot/types/ticker.go

DESCRIPTION:
Market-data payloads for KuCoin Spot: the level-1 ticker
(GET /api/v1/market/orderbook/level1 and the "/market/ticker:{symbol}" WS
topic) and the 24h stats (GET /api/v1/market/stats).

TIMESTAMPS: spot ticker/level1 carry the time in MILLISECONDS (unlike the
futures ticker, which uses nanoseconds). The trade tape (/market/match,
/api/v1/market/histories) still uses nanoseconds — see the converters in
the spot package.
*/

package types

import "github.com/shopspring/decimal"

// MarketTicker — best bid/ask + last trade for a spot pair.
type MarketTicker struct {
	Symbol string
	// Sequence — KuCoin ticker sequence (monotonic per symbol).
	Sequence int64
	// LastPrice / LastSize — last trade.
	LastPrice decimal.Decimal
	LastSize  decimal.Decimal
	// Best bid/ask.
	BestBidPrice decimal.Decimal
	BestBidSize  decimal.Decimal
	BestAskPrice decimal.Decimal
	BestAskSize  decimal.Decimal
	// TsMs — ticker timestamp in milliseconds.
	TsMs int64
}

// MarketStats — 24-hour rolling statistics for a spot pair, mapped from
// GET /api/v1/market/stats.
type MarketStats struct {
	Symbol string
	// Last — last trade price.
	Last decimal.Decimal
	// ChangeRate / ChangePrice — 24h change (fraction / absolute).
	ChangeRate  decimal.Decimal
	ChangePrice decimal.Decimal
	// High / Low — 24h high / low.
	High decimal.Decimal
	Low  decimal.Decimal
	// Vol / VolValue — 24h volume (base) / turnover (quote).
	Vol      decimal.Decimal
	VolValue decimal.Decimal
	// Best bid/ask at the time of the call.
	Buy  decimal.Decimal
	Sell decimal.Decimal
	// AveragePrice — 24h average price.
	AveragePrice decimal.Decimal
	// TakerFeeRate / MakerFeeRate — current fee rates for the account.
	TakerFeeRate decimal.Decimal
	MakerFeeRate decimal.Decimal
	// TsMs — stats timestamp (ms).
	TsMs int64
}
