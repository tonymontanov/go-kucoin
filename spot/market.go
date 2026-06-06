/*
FILE: spot/market.go

DESCRIPTION:
Public market-data sub-client for the KuCoin Spot profile. Wraps the
unsigned REST endpoints on api.kucoin.com and converts the wire JSON into the
SDK's typed structs (decimal-normalised, ms timestamps).

ENDPOINTS:
  - GET /api/v1/timestamp                              server time (ms)
  - GET /api/v2/symbols                                all symbols
  - GET /api/v2/symbols/{symbol}                       one symbol
  - GET /api/v1/market/orderbook/level1?symbol=        level1 ticker
  - GET /api/v1/market/stats?symbol=                   24h stats
  - GET /api/v1/market/orderbook/level2_100?symbol=    order-book snapshot
  - GET /api/v1/market/candles?symbol=&type=&...       klines
  - GET /api/v1/market/histories?symbol=               recent trades

WIRE NOTES (KuCoin Spot vs Futures):
  - numerics ship as JSON STRINGS (decimal decodes them);
  - sequence fields are STRINGS (parsed to int64);
  - level1 / stats timestamps are MILLISECONDS; the trade tape uses
    NANOSECONDS;
  - the candle row order is [time, open, CLOSE, high, low, volume, turnover]
    with time in SECONDS (note: open then CLOSE, unlike the futures rows).
*/

package spot

import (
	"context"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// marketMeta is the rate-limit metadata stamped on every market-data call.
var marketMeta = rest.RequestMeta{Category: "market"}

// MarketDataClient — public market-data sub-client.
type MarketDataClient struct {
	c *Client
}

// newMarketDataClient wires the sub-client to its parent.
func newMarketDataClient(c *Client) *MarketDataClient {
	return &MarketDataClient{c: c}
}

// GetServerTime returns the KuCoin Spot server time in milliseconds.
func (m *MarketDataClient) GetServerTime(ctx context.Context) (int64, error) {
	var ts int64
	if err := m.c.doGET(ctx, false, "/api/v1/timestamp", nil, marketMeta, &ts); err != nil {
		return 0, err
	}
	return ts, nil
}

// GetSymbols returns the specification of every spot trading pair.
func (m *MarketDataClient) GetSymbols(ctx context.Context) ([]spottypes.SymbolInfo, error) {
	var wire []symbolWire
	if err := m.c.doGET(ctx, false, "/api/v2/symbols", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []spottypes.SymbolInfo = make([]spottypes.SymbolInfo, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toSymbolInfo()
	}
	return out, nil
}

// GetSymbol returns the specification of a single spot trading pair.
func (m *MarketDataClient) GetSymbol(ctx context.Context, symbol string) (*spottypes.SymbolInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetSymbol", "symbol is required")
	}
	var wire symbolWire
	if err := m.c.doGET(ctx, false, "/api/v2/symbols/"+symbol, nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var info spottypes.SymbolInfo = wire.toSymbolInfo()
	return &info, nil
}

// GetTicker returns the level1 ticker (best bid/ask + last trade).
func (m *MarketDataClient) GetTicker(ctx context.Context, symbol string) (*spottypes.MarketTicker, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetTicker", "symbol is required")
	}
	var wire level1Wire
	if err := m.c.doGET(ctx, false, "/api/v1/market/orderbook/level1", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var t spottypes.MarketTicker = wire.toTicker(symbol)
	return &t, nil
}

// GetStats returns the 24-hour rolling statistics for a pair.
func (m *MarketDataClient) GetStats(ctx context.Context, symbol string) (*spottypes.MarketStats, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetStats", "symbol is required")
	}
	var wire statsWire
	if err := m.c.doGET(ctx, false, "/api/v1/market/stats", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var s spottypes.MarketStats = wire.toStats()
	return &s, nil
}

// GetOrderBook returns an order-book snapshot (best 100 levels per side),
// seeded with the KuCoin sequence so the local engine can reconcile the WS
// change stream. Uses the PUBLIC level2_100 endpoint (no credentials). For
// market-making depth use GetOrderBookFull.
func (m *MarketDataClient) GetOrderBook(ctx context.Context, symbol string) (*roottypes.OrderBookSnapshot, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOrderBook", "symbol is required")
	}
	var wire level2Wire
	if err := m.c.doGET(ctx, false, "/api/v1/market/orderbook/level2_100", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var snap roottypes.OrderBookSnapshot = wire.toSnapshot(symbol)
	return &snap, nil
}

// GetOrderBookFull returns the FULL-depth aggregated order-book snapshot via
// the SIGNED /api/v3/market/orderbook/level2 endpoint. KuCoin gates the
// full book behind API credentials (it is heavier than level2_100), so this
// call requires a configured key. The response shape matches level2_100, so
// it seeds the same sequence engine — WatchOrderBook prefers this source when
// credentials are present (see the book manager's reseed).
func (m *MarketDataClient) GetOrderBookFull(ctx context.Context, symbol string) (*roottypes.OrderBookSnapshot, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOrderBookFull", "symbol is required")
	}
	var wire level2Wire
	if err := m.c.doGET(ctx, true, "/api/v3/market/orderbook/level2", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var snap roottypes.OrderBookSnapshot = wire.toSnapshot(symbol)
	return &snap, nil
}

// GetKlines returns historical candles for a pair in [fromSec, toSec]. The
// window is in SECONDS (KuCoin Spot convention); pass 0 to let KuCoin apply
// its defaults. Rows arrive newest-first.
func (m *MarketDataClient) GetKlines(ctx context.Context, symbol string, tf spottypes.Timeframe, fromSec, toSec int64) (roottypes.Candles, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetKlines", "symbol is required")
	}
	var gran string = spotGranularity[tf]
	if gran == "" {
		return nil, errInvalidRequest("GetKlines", "unsupported timeframe")
	}
	var query map[string]string = map[string]string{"symbol": symbol, "type": gran}
	if fromSec > 0 {
		query["startAt"] = itoa(fromSec)
	}
	if toSec > 0 {
		query["endAt"] = itoa(toSec)
	}
	var rows [][]decimal.Decimal
	if err := m.c.doGET(ctx, false, "/api/v1/market/candles", query, marketMeta, &rows); err != nil {
		return nil, err
	}
	return klinesFromRows(rows), nil
}

// GetRecentTrades returns the recent public trade tape for a pair.
func (m *MarketDataClient) GetRecentTrades(ctx context.Context, symbol string) ([]roottypes.TradeUpdate, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetRecentTrades", "symbol is required")
	}
	var wire []tradeWire
	if err := m.c.doGET(ctx, false, "/api/v1/market/histories", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []roottypes.TradeUpdate = make([]roottypes.TradeUpdate, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toTradeUpdate(symbol)
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

// symbolWire mirrors one element of /api/v2/symbols. All numerics are JSON
// strings; decimal decodes them.
type symbolWire struct {
	Symbol          string          `json:"symbol"`
	Name            string          `json:"name"`
	BaseCurrency    string          `json:"baseCurrency"`
	QuoteCurrency   string          `json:"quoteCurrency"`
	FeeCurrency     string          `json:"feeCurrency"`
	Market          string          `json:"market"`
	BaseMinSize     decimal.Decimal `json:"baseMinSize"`
	QuoteMinSize    decimal.Decimal `json:"quoteMinSize"`
	BaseMaxSize     decimal.Decimal `json:"baseMaxSize"`
	QuoteMaxSize    decimal.Decimal `json:"quoteMaxSize"`
	BaseIncrement   decimal.Decimal `json:"baseIncrement"`
	QuoteIncrement  decimal.Decimal `json:"quoteIncrement"`
	PriceIncrement  decimal.Decimal `json:"priceIncrement"`
	PriceLimitRate  decimal.Decimal `json:"priceLimitRate"`
	MinFunds        decimal.Decimal `json:"minFunds"`
	IsMarginEnabled bool            `json:"isMarginEnabled"`
	EnableTrading   bool            `json:"enableTrading"`
}

func (w symbolWire) toSymbolInfo() spottypes.SymbolInfo {
	return spottypes.SymbolInfo{
		Symbol:          w.Symbol,
		Name:            w.Name,
		BaseCurrency:    w.BaseCurrency,
		QuoteCurrency:   w.QuoteCurrency,
		FeeCurrency:     w.FeeCurrency,
		Market:          w.Market,
		BaseMinSize:     w.BaseMinSize,
		BaseMaxSize:     w.BaseMaxSize,
		QuoteMinSize:    w.QuoteMinSize,
		QuoteMaxSize:    w.QuoteMaxSize,
		BaseIncrement:   w.BaseIncrement,
		QuoteIncrement:  w.QuoteIncrement,
		PriceIncrement:  w.PriceIncrement,
		PriceLimitRate:  w.PriceLimitRate,
		MinFunds:        w.MinFunds,
		IsMarginEnabled: w.IsMarginEnabled,
		EnableTrading:   w.EnableTrading,
	}
}

// level1Wire mirrors /api/v1/market/orderbook/level1. sequence is a string;
// time is in milliseconds.
type level1Wire struct {
	Sequence    string          `json:"sequence"`
	Price       decimal.Decimal `json:"price"`
	Size        decimal.Decimal `json:"size"`
	BestBid     decimal.Decimal `json:"bestBid"`
	BestBidSize decimal.Decimal `json:"bestBidSize"`
	BestAsk     decimal.Decimal `json:"bestAsk"`
	BestAskSize decimal.Decimal `json:"bestAskSize"`
	Time        int64           `json:"time"`
}

func (w level1Wire) toTicker(symbol string) spottypes.MarketTicker {
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

// statsWire mirrors /api/v1/market/stats. time is in milliseconds.
type statsWire struct {
	Time         int64           `json:"time"`
	Symbol       string          `json:"symbol"`
	Buy          decimal.Decimal `json:"buy"`
	Sell         decimal.Decimal `json:"sell"`
	ChangeRate   decimal.Decimal `json:"changeRate"`
	ChangePrice  decimal.Decimal `json:"changePrice"`
	High         decimal.Decimal `json:"high"`
	Low          decimal.Decimal `json:"low"`
	Vol          decimal.Decimal `json:"vol"`
	VolValue     decimal.Decimal `json:"volValue"`
	Last         decimal.Decimal `json:"last"`
	AveragePrice decimal.Decimal `json:"averagePrice"`
	TakerFeeRate decimal.Decimal `json:"takerFeeRate"`
	MakerFeeRate decimal.Decimal `json:"makerFeeRate"`
}

func (w statsWire) toStats() spottypes.MarketStats {
	return spottypes.MarketStats{
		Symbol:       w.Symbol,
		Last:         w.Last,
		ChangeRate:   w.ChangeRate,
		ChangePrice:  w.ChangePrice,
		High:         w.High,
		Low:          w.Low,
		Vol:          w.Vol,
		VolValue:     w.VolValue,
		Buy:          w.Buy,
		Sell:         w.Sell,
		AveragePrice: w.AveragePrice,
		TakerFeeRate: w.TakerFeeRate,
		MakerFeeRate: w.MakerFeeRate,
		TsMs:         w.Time,
	}
}

// level2Wire mirrors /api/v1/market/orderbook/level2_100. bids/asks are
// [price, size] STRING pairs; sequence is a string; time is ms.
type level2Wire struct {
	Sequence string     `json:"sequence"`
	Time     int64      `json:"time"`
	Bids     [][]string `json:"bids"`
	Asks     [][]string `json:"asks"`
}

func (w level2Wire) toSnapshot(symbol string) roottypes.OrderBookSnapshot {
	return roottypes.OrderBookSnapshot{
		Symbol:   symbol,
		Bids:     strPairsToLevels(w.Bids),
		Asks:     strPairsToLevels(w.Asks),
		TsMs:     w.Time,
		Sequence: parseInt64(w.Sequence),
	}
}

// tradeWire mirrors one element of /api/v1/market/histories. time is in
// NANOSECONDS; size/price are strings.
type tradeWire struct {
	Sequence string          `json:"sequence"`
	Price    decimal.Decimal `json:"price"`
	Size     decimal.Decimal `json:"size"`
	Side     string          `json:"side"`
	Time     int64           `json:"time"`
}

func (w tradeWire) toTradeUpdate(symbol string) roottypes.TradeUpdate {
	return roottypes.TradeUpdate{
		Symbol:  symbol,
		Price:   w.Price,
		Size:    w.Size,
		Side:    roottypes.SideType(w.Side),
		TradeID: w.Sequence,
		TsMs:    nsToMs(w.Time),
	}
}

// ---------------------------------------------------------------------
// Shared converters.
// ---------------------------------------------------------------------

// strPairsToLevels converts ["price","size"] string pairs into book levels,
// skipping malformed rows.
func strPairsToLevels(pairs [][]string) []roottypes.OrderBookLevel {
	if len(pairs) == 0 {
		return nil
	}
	var out []roottypes.OrderBookLevel = make([]roottypes.OrderBookLevel, 0, len(pairs))
	var i int
	for i = 0; i < len(pairs); i++ {
		if len(pairs[i]) < 2 {
			continue
		}
		var price, size decimal.Decimal
		var err error
		price, err = decimal.NewFromString(pairs[i][0])
		if err != nil {
			continue
		}
		size, err = decimal.NewFromString(pairs[i][1])
		if err != nil {
			continue
		}
		out = append(out, roottypes.OrderBookLevel{Price: price, Size: size})
	}
	return out
}

// klinesFromRows maps [time(sec), open, close, high, low, volume, turnover]
// rows into the typed Candles slice. time is converted to milliseconds. Note
// the spot column order places CLOSE before HIGH/LOW.
func klinesFromRows(rows [][]decimal.Decimal) roottypes.Candles {
	if len(rows) == 0 {
		return nil
	}
	var out roottypes.Candles = make(roottypes.Candles, 0, len(rows))
	var i int
	for i = 0; i < len(rows); i++ {
		var r []decimal.Decimal = rows[i]
		if len(r) < 6 {
			continue
		}
		out = append(out, roottypes.Candle{
			OpenTimeMs: r[0].IntPart() * 1000,
			Open:       r[1],
			Close:      r[2],
			High:       r[3],
			Low:        r[4],
			Volume:     r[5],
		})
	}
	return out
}

// parseInt64 parses a base-10 string into int64, returning 0 on error (the
// spot API ships sequence numbers as strings).
func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	var v int64
	var err error
	v, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
