/*
FILE: futures/stream-wire.go

DESCRIPTION:
On-the-wire push payloads for the public KuCoin Futures WS topics and their
converters into the SDK's typed structs. Kept separate from stream.go so the
dispatch logic stays readable.

TIMESTAMPS: ticker / execution pushes carry `ts` in NANOSECONDS; the
converters normalise to milliseconds. The candle push carries the candle
start time in SECONDS as the first array element.
*/

package futures

import (
	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// level2PushWire mirrors a /contractMarket/level2 push: one change per frame.
type level2PushWire struct {
	Sequence  int64  `json:"sequence"`
	Change    string `json:"change"`
	Timestamp int64  `json:"timestamp"`
}

// tickerPushWire mirrors a /contractMarket/ticker push. ts is in nanoseconds.
type tickerPushWire struct {
	Sequence     int64           `json:"sequence"`
	Side         string          `json:"side"`
	Size         decimal.Decimal `json:"size"`
	Price        decimal.Decimal `json:"price"`
	BestBidSize  decimal.Decimal `json:"bestBidSize"`
	BestBidPrice decimal.Decimal `json:"bestBidPrice"`
	BestAskPrice decimal.Decimal `json:"bestAskPrice"`
	BestAskSize  decimal.Decimal `json:"bestAskSize"`
	TradeID      string          `json:"tradeId"`
	Ts           int64           `json:"ts"`
}

func (w tickerPushWire) toTicker(symbol string) futurestypes.MarketTicker {
	return futurestypes.MarketTicker{
		Symbol:       symbol,
		Sequence:     w.Sequence,
		LastPrice:    w.Price,
		LastSize:     w.Size,
		Side:         futurestypes.SideType(w.Side),
		BestBidPrice: w.BestBidPrice,
		BestBidSize:  w.BestBidSize,
		BestAskPrice: w.BestAskPrice,
		BestAskSize:  w.BestAskSize,
		TradeID:      w.TradeID,
		TsMs:         nsToMs(w.Ts),
	}
}

// executionPushWire mirrors a /contractMarket/execution push. ts is ns.
type executionPushWire struct {
	Sequence int64           `json:"sequence"`
	Side     string          `json:"side"`
	Size     decimal.Decimal `json:"size"`
	Price    decimal.Decimal `json:"price"`
	TradeID  string          `json:"tradeId"`
	Ts       int64           `json:"ts"`
}

func (w executionPushWire) toTradeUpdate(symbol string) roottypes.TradeUpdate {
	return roottypes.TradeUpdate{
		Symbol:  symbol,
		Price:   w.Price,
		Size:    w.Size,
		Side:    roottypes.SideType(w.Side),
		TradeID: w.TradeID,
		TsMs:    nsToMs(w.Ts),
	}
}

// instrumentPushWire mirrors a /contract/instrument:{symbol} push with the
// "mark.index.price" subject. KuCoin ships the timestamp in MILLISECONDS on
// this channel (unlike ticker/execution which use nanoseconds) and the
// granularity as the mark-price push interval in ms.
type instrumentPushWire struct {
	Granularity int64           `json:"granularity"`
	IndexPrice  decimal.Decimal `json:"indexPrice"`
	MarkPrice   decimal.Decimal `json:"markPrice"`
	Timestamp   int64           `json:"timestamp"`
}

// toMarkPrice maps the instrument push into the typed MarkPrice. ok is false
// when neither price is present (e.g. a funding-rate frame on the same topic
// decoded into this struct).
func (w instrumentPushWire) toMarkPrice(symbol string) (futurestypes.MarkPrice, bool) {
	if w.MarkPrice.IsZero() && w.IndexPrice.IsZero() {
		return futurestypes.MarkPrice{}, false
	}
	return futurestypes.MarkPrice{
		Symbol:      symbol,
		Value:       w.MarkPrice,
		IndexPrice:  w.IndexPrice,
		Granularity: w.Granularity,
		TimePointMs: w.Timestamp,
	}, true
}

// candlePushWire mirrors a /contractMarket/candle push. The candles array is
// [ startSeconds, open, high, low, close, volume, turnover ] as numbers.
type candlePushWire struct {
	Symbol  string            `json:"symbol"`
	Candles []decimal.Decimal `json:"candles"`
	Time    int64             `json:"time"`
}

// toKlineUpdate maps the candle array into a KlineUpdate. ok is false when
// the array is too short to be a valid candle.
func (w candlePushWire) toKlineUpdate(symbol string, tf futurestypes.Timeframe) (roottypes.KlineUpdate, bool) {
	if len(w.Candles) < 6 {
		return roottypes.KlineUpdate{}, false
	}
	return roottypes.KlineUpdate{
		Symbol:   symbol,
		Interval: tf,
		StartMs:  w.Candles[0].IntPart() * 1000,
		Open:     w.Candles[1],
		High:     w.Candles[2],
		Low:      w.Candles[3],
		Close:    w.Candles[4],
		Volume:   w.Candles[5],
	}, true
}
