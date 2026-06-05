/*
FILE: futures/types/market-ticker.go

DESCRIPTION:
Market-data payloads for KuCoin Futures: the real-time ticker
(GET /api/v1/ticker and the "/contractMarket/tickerV2" WS topic), the
current mark/index price (GET /api/v1/mark-price/{symbol}/current) and the
current funding rate (GET /api/v1/funding-rate/{symbol}/current).
*/

package types

import "github.com/shopspring/decimal"

// MarketTicker — best bid/ask + last trade for a contract.
type MarketTicker struct {
	Symbol string
	// Sequence — KuCoin ticker sequence (monotonic per symbol).
	Sequence int64
	// LastPrice / LastSize — last trade.
	LastPrice decimal.Decimal
	LastSize  decimal.Decimal
	// Side — last trade aggressor side.
	Side SideType
	// Best bid/ask.
	BestBidPrice decimal.Decimal
	BestBidSize  decimal.Decimal
	BestAskPrice decimal.Decimal
	BestAskSize  decimal.Decimal
	// TradeID — last trade id.
	TradeID string
	// TsMs — ticker timestamp in milliseconds (KuCoin ships ns; SDK
	// converts at the boundary).
	TsMs int64
}

// MarkPrice — current mark + index price for a contract.
type MarkPrice struct {
	Symbol string
	// Value — mark price.
	Value decimal.Decimal
	// IndexPrice — underlying index price.
	IndexPrice decimal.Decimal
	// Granularity — push granularity in ms.
	Granularity int64
	// TimePointMs — reference time (ms).
	TimePointMs int64
}

// FundingRate — current funding rate for a contract.
type FundingRate struct {
	Symbol string
	// Value — current funding rate (fraction, e.g. 0.0001 = 0.01%).
	Value decimal.Decimal
	// PredictedValue — predicted next-period funding rate.
	PredictedValue decimal.Decimal
	// Granularity — funding interval in ms.
	Granularity int64
	// TimePointMs — reference time (ms).
	TimePointMs int64
}
