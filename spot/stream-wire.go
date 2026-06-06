/*
FILE: spot/stream-wire.go

DESCRIPTION:
On-the-wire push payloads for the public KuCoin Spot WS topics and their
converters into the SDK's typed structs. Kept separate from stream.go so the
dispatch logic stays readable.

TIMESTAMPS:
  - /market/ticker carries `time` in MILLISECONDS;
  - /market/match (trade tape) carries `time` in NANOSECONDS;
  - /market/candles carries the candle start (array[0]) in SECONDS and the
    push `time` in nanoseconds.

NUMERICS: spot WS ships prices/sizes as JSON STRINGS; decimal decodes them.
The level2 change entries are ["price","size","sequence"] string triples.
*/

package spot

import (
	"github.com/shopspring/decimal"

	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// level2PushWire mirrors a /market/level2 push (subject "trade.l2update").
// Each side change is a ["price","size","sequence"] triple; sequences are
// globally monotonic and contiguous across both sides.
type level2PushWire struct {
	SequenceStart int64  `json:"sequenceStart"`
	SequenceEnd   int64  `json:"sequenceEnd"`
	Symbol        string `json:"symbol"`
	Time          int64  `json:"time"`
	Changes       struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	} `json:"changes"`
}

// timeMs returns the frame timestamp in milliseconds (already ms on this
// channel; 0 when absent).
func (w level2PushWire) timeMs() int64 { return w.Time }

// changes flattens the asks/bids triples into per-side pendingChange records
// sorted ascending by sequence, ready for the contiguous engine. Malformed
// triples are skipped.
func (w level2PushWire) changes() []pendingChange {
	var out []pendingChange = make([]pendingChange, 0, len(w.Changes.Asks)+len(w.Changes.Bids))
	out = appendChanges(out, w.Changes.Asks, orderbook_SideSell)
	out = appendChanges(out, w.Changes.Bids, orderbook_SideBuy)
	sortPendingBySeq(out)
	return out
}

// orderbook side constants mirror internal/kccommon/orderbook to avoid an
// import just for two strings on the hot path.
const (
	orderbook_SideBuy  = "buy"
	orderbook_SideSell = "sell"
)

// appendChanges parses ["price","size","sequence"] triples for one side.
func appendChanges(dst []pendingChange, triples [][]string, side string) []pendingChange {
	var i int
	for i = 0; i < len(triples); i++ {
		var t []string = triples[i]
		if len(t) < 3 {
			continue
		}
		var price, size decimal.Decimal
		var err error
		price, err = decimal.NewFromString(t[0])
		if err != nil {
			continue
		}
		size, err = decimal.NewFromString(t[1])
		if err != nil {
			continue
		}
		var seq int64 = parseInt64(t[2])
		if seq == 0 {
			continue
		}
		dst = append(dst, pendingChange{side: side, price: price, size: size, seq: seq})
	}
	return dst
}

// tickerPushWire mirrors a /market/ticker push. sequence is a string; time
// is in milliseconds.
type tickerPushWire struct {
	Sequence    string          `json:"sequence"`
	Price       decimal.Decimal `json:"price"`
	Size        decimal.Decimal `json:"size"`
	BestBid     decimal.Decimal `json:"bestBid"`
	BestBidSize decimal.Decimal `json:"bestBidSize"`
	BestAsk     decimal.Decimal `json:"bestAsk"`
	BestAskSize decimal.Decimal `json:"bestAskSize"`
	Time        int64           `json:"time"`
}

func (w tickerPushWire) toTicker(symbol string) spottypes.MarketTicker {
	return spottypes.MarketTicker{
		Symbol:       symbol,
		Sequence:     parseInt64(w.Sequence),
		LastPrice:    w.Price,
		LastSize:     w.Size,
		BestBidPrice: w.BestBid,
		BestBidSize:  w.BestBidSize,
		BestAskPrice: w.BestAsk,
		BestAskSize:  w.BestAskSize,
		TsMs:         w.Time,
	}
}

// matchPushWire mirrors a /market/match push. time is in NANOSECONDS.
type matchPushWire struct {
	Sequence string          `json:"sequence"`
	Symbol   string          `json:"symbol"`
	Side     string          `json:"side"`
	Size     decimal.Decimal `json:"size"`
	Price    decimal.Decimal `json:"price"`
	TradeID  string          `json:"tradeId"`
	Time     int64           `json:"time"`
}

func (w matchPushWire) toTradeUpdate(symbol string) roottypes.TradeUpdate {
	return roottypes.TradeUpdate{
		Symbol:  symbol,
		Price:   w.Price,
		Size:    w.Size,
		Side:    roottypes.SideType(w.Side),
		TradeID: w.TradeID,
		TsMs:    nsToMs(w.Time),
	}
}

// candlePushWire mirrors a /market/candles push. The candles array is
// [ startSeconds, open, close, high, low, volume, turnover ] as STRINGS
// (decimal decodes them).
type candlePushWire struct {
	Symbol  string            `json:"symbol"`
	Candles []decimal.Decimal `json:"candles"`
	Time    int64             `json:"time"`
}

// toKlineUpdate maps the candle array into a KlineUpdate. ok is false when
// the array is too short to be a valid candle. NB: spot column order is
// [time, open, CLOSE, high, low, volume, turnover].
func (w candlePushWire) toKlineUpdate(symbol string, tf spottypes.Timeframe) (roottypes.KlineUpdate, bool) {
	if len(w.Candles) < 6 {
		return roottypes.KlineUpdate{}, false
	}
	return roottypes.KlineUpdate{
		Symbol:   symbol,
		Interval: tf,
		StartMs:  w.Candles[0].IntPart() * 1000,
		Open:     w.Candles[1],
		Close:    w.Candles[2],
		High:     w.Candles[3],
		Low:      w.Candles[4],
		Volume:   w.Candles[5],
	}, true
}
