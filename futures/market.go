/*
FILE: futures/market.go

DESCRIPTION:
Public market-data sub-client for the KuCoin Futures profile. Wraps the
unsigned REST endpoints on api-futures.kucoin.com and converts the wire
JSON into the SDK's typed structs (decimal-normalised, ms timestamps).

ENDPOINTS:
  - GET /api/v1/timestamp                         server time (ms)
  - GET /api/v1/contracts/active                  all contracts
  - GET /api/v1/contracts/{symbol}                one contract
  - GET /api/v1/ticker?symbol=                     real-time ticker
  - GET /api/v1/level2/snapshot?symbol=            full order book snapshot
  - GET /api/v1/kline/query?symbol=&granularity=&from=&to=  klines
  - GET /api/v1/mark-price/{symbol}/current        mark + index price
  - GET /api/v1/funding-rate/{symbol}/current      current funding rate
  - GET /api/v1/trade/history?symbol=              recent public trades

KuCoin ships market-data numerics as JSON numbers (occasionally strings);
decimal.Decimal decodes both. Trade/ticker timestamps arrive in
NANOSECONDS and are converted to milliseconds at the boundary.
*/

package futures

import (
	"context"

	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
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

// GetServerTime returns the KuCoin Futures server time in milliseconds.
func (m *MarketDataClient) GetServerTime(ctx context.Context) (int64, error) {
	var ts int64
	if err := m.c.doGET(ctx, false, "/api/v1/timestamp", nil, marketMeta, &ts); err != nil {
		return 0, err
	}
	return ts, nil
}

// GetContracts returns the specification of every active contract.
func (m *MarketDataClient) GetContracts(ctx context.Context) ([]futurestypes.SymbolInfo, error) {
	var wire []contractWire
	if err := m.c.doGET(ctx, false, "/api/v1/contracts/active", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []futurestypes.SymbolInfo = make([]futurestypes.SymbolInfo, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toSymbolInfo()
	}
	return out, nil
}

// GetContract returns the specification of a single contract.
func (m *MarketDataClient) GetContract(ctx context.Context, symbol string) (*futurestypes.SymbolInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetContract", "symbol is required")
	}
	var wire contractWire
	if err := m.c.doGET(ctx, false, "/api/v1/contracts/"+symbol, nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var info futurestypes.SymbolInfo = wire.toSymbolInfo()
	return &info, nil
}

// GetTicker returns the real-time ticker for a contract.
func (m *MarketDataClient) GetTicker(ctx context.Context, symbol string) (*futurestypes.MarketTicker, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetTicker", "symbol is required")
	}
	var wire tickerWire
	if err := m.c.doGET(ctx, false, "/api/v1/ticker", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var t futurestypes.MarketTicker = wire.toTicker()
	return &t, nil
}

// GetOrderBook returns the full level-2 order book snapshot, seeded with the
// KuCoin sequence so the local engine can reconcile the WS change stream.
func (m *MarketDataClient) GetOrderBook(ctx context.Context, symbol string) (*roottypes.OrderBookSnapshot, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOrderBook", "symbol is required")
	}
	var wire level2Wire
	if err := m.c.doGET(ctx, false, "/api/v1/level2/snapshot", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
		return nil, err
	}
	var snap roottypes.OrderBookSnapshot = wire.toSnapshot(symbol)
	return &snap, nil
}

// GetKlines returns historical candles for a contract in [fromMs, toMs].
// Pass 0 for fromMs/toMs to let KuCoin apply its defaults.
func (m *MarketDataClient) GetKlines(ctx context.Context, symbol string, tf futurestypes.Timeframe, fromMs, toMs int64) (roottypes.Candles, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetKlines", "symbol is required")
	}
	var gran string = tf.Wire()
	if gran == "" {
		return nil, errInvalidRequest("GetKlines", "unsupported timeframe")
	}
	var query map[string]string = map[string]string{"symbol": symbol, "granularity": gran}
	if fromMs > 0 {
		query["from"] = itoa(fromMs)
	}
	if toMs > 0 {
		query["to"] = itoa(toMs)
	}
	var rows [][]decimal.Decimal
	if err := m.c.doGET(ctx, false, "/api/v1/kline/query", query, marketMeta, &rows); err != nil {
		return nil, err
	}
	return klinesFromRows(rows), nil
}

// GetMarkPrice returns the current mark + index price.
func (m *MarketDataClient) GetMarkPrice(ctx context.Context, symbol string) (*futurestypes.MarkPrice, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetMarkPrice", "symbol is required")
	}
	var wire markPriceWire
	if err := m.c.doGET(ctx, false, "/api/v1/mark-price/"+symbol+"/current", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	return &futurestypes.MarkPrice{
		Symbol:      wire.Symbol,
		Value:       wire.Value,
		IndexPrice:  wire.IndexPrice,
		Granularity: wire.Granularity,
		TimePointMs: wire.TimePoint,
	}, nil
}

// GetFundingRate returns the current funding rate.
func (m *MarketDataClient) GetFundingRate(ctx context.Context, symbol string) (*futurestypes.FundingRate, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetFundingRate", "symbol is required")
	}
	var wire fundingRateWire
	if err := m.c.doGET(ctx, false, "/api/v1/funding-rate/"+symbol+"/current", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	return &futurestypes.FundingRate{
		Symbol:         wire.Symbol,
		Value:          wire.Value,
		PredictedValue: wire.PredictedValue,
		Granularity:    wire.Granularity,
		TimePointMs:    wire.TimePoint,
	}, nil
}

// GetRecentTrades returns the recent public trade tape for a contract.
func (m *MarketDataClient) GetRecentTrades(ctx context.Context, symbol string) ([]roottypes.TradeUpdate, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetRecentTrades", "symbol is required")
	}
	var wire []tradeWire
	if err := m.c.doGET(ctx, false, "/api/v1/trade/history", map[string]string{"symbol": symbol}, marketMeta, &wire); err != nil {
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

// contractWire mirrors one element of /api/v1/contracts/active. KuCoin ships
// most numerics as JSON numbers and openInterest as a string; decimal
// decodes both, and JSON null leaves the zero value.
type contractWire struct {
	Symbol             string          `json:"symbol"`
	RootSymbol         string          `json:"rootSymbol"`
	Type               string          `json:"type"`
	BaseCurrency       string          `json:"baseCurrency"`
	QuoteCurrency      string          `json:"quoteCurrency"`
	SettleCurrency     string          `json:"settleCurrency"`
	Status             string          `json:"status"`
	IsInverse          bool            `json:"isInverse"`
	Multiplier         decimal.Decimal `json:"multiplier"`
	LotSize            decimal.Decimal `json:"lotSize"`
	TickSize           decimal.Decimal `json:"tickSize"`
	IndexPriceTickSize decimal.Decimal `json:"indexPriceTickSize"`
	MaxOrderQty        decimal.Decimal `json:"maxOrderQty"`
	MaxPrice           decimal.Decimal `json:"maxPrice"`
	MaxLeverage        decimal.Decimal `json:"maxLeverage"`
	InitialMargin      decimal.Decimal `json:"initialMargin"`
	MaintainMargin     decimal.Decimal `json:"maintainMargin"`
	MakerFeeRate       decimal.Decimal `json:"makerFeeRate"`
	TakerFeeRate       decimal.Decimal `json:"takerFeeRate"`
	MarkPrice          decimal.Decimal `json:"markPrice"`
	IndexPrice         decimal.Decimal `json:"indexPrice"`
	LastTradePrice     decimal.Decimal `json:"lastTradePrice"`
	FundingFeeRate     decimal.Decimal `json:"fundingFeeRate"`
	OpenInterest       decimal.Decimal `json:"openInterest"`
	VolumeOf24h        decimal.Decimal `json:"volumeOf24h"`
	TurnoverOf24h      decimal.Decimal `json:"turnoverOf24h"`
}

func (w contractWire) toSymbolInfo() futurestypes.SymbolInfo {
	return futurestypes.SymbolInfo{
		Symbol:             w.Symbol,
		RootSymbol:         w.RootSymbol,
		Type:               w.Type,
		BaseCurrency:       w.BaseCurrency,
		QuoteCurrency:      w.QuoteCurrency,
		SettleCurrency:     w.SettleCurrency,
		Status:             w.Status,
		IsInverse:          w.IsInverse,
		Multiplier:         w.Multiplier,
		LotSize:            w.LotSize,
		TickSize:           w.TickSize,
		IndexPriceTickSize: w.IndexPriceTickSize,
		MaxOrderQty:        w.MaxOrderQty,
		MaxPrice:           w.MaxPrice,
		MaxLeverage:        w.MaxLeverage,
		InitialMargin:      w.InitialMargin,
		MaintainMargin:     w.MaintainMargin,
		MakerFeeRate:       w.MakerFeeRate,
		TakerFeeRate:       w.TakerFeeRate,
		MarkPrice:          w.MarkPrice,
		IndexPrice:         w.IndexPrice,
		LastTradePrice:     w.LastTradePrice,
		FundingFeeRate:     w.FundingFeeRate,
		OpenInterest:       w.OpenInterest,
		VolumeOf24h:        w.VolumeOf24h,
		TurnoverOf24h:      w.TurnoverOf24h,
	}
}

// tickerWire mirrors /api/v1/ticker. ts is in nanoseconds.
type tickerWire struct {
	Sequence     int64           `json:"sequence"`
	Symbol       string          `json:"symbol"`
	Side         string          `json:"side"`
	Size         decimal.Decimal `json:"size"`
	Price        decimal.Decimal `json:"price"`
	BestBidPrice decimal.Decimal `json:"bestBidPrice"`
	BestBidSize  decimal.Decimal `json:"bestBidSize"`
	BestAskPrice decimal.Decimal `json:"bestAskPrice"`
	BestAskSize  decimal.Decimal `json:"bestAskSize"`
	TradeID      string          `json:"tradeId"`
	Ts           int64           `json:"ts"`
}

func (w tickerWire) toTicker() futurestypes.MarketTicker {
	return futurestypes.MarketTicker{
		Symbol:       w.Symbol,
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

// level2Wire mirrors /api/v1/level2/snapshot. bids/asks are [price, size]
// number pairs.
type level2Wire struct {
	Sequence int64               `json:"sequence"`
	Symbol   string              `json:"symbol"`
	Bids     [][]decimal.Decimal `json:"bids"`
	Asks     [][]decimal.Decimal `json:"asks"`
	Ts       int64               `json:"ts"`
}

func (w level2Wire) toSnapshot(symbol string) roottypes.OrderBookSnapshot {
	return roottypes.OrderBookSnapshot{
		Symbol:   symbol,
		Bids:     pairsToLevels(w.Bids),
		Asks:     pairsToLevels(w.Asks),
		TsMs:     w.Ts,
		Sequence: w.Sequence,
	}
}

// markPriceWire mirrors /api/v1/mark-price/{symbol}/current.
type markPriceWire struct {
	Symbol      string          `json:"symbol"`
	Granularity int64           `json:"granularity"`
	TimePoint   int64           `json:"timePoint"`
	Value       decimal.Decimal `json:"value"`
	IndexPrice  decimal.Decimal `json:"indexPrice"`
}

// fundingRateWire mirrors /api/v1/funding-rate/{symbol}/current.
type fundingRateWire struct {
	Symbol         string          `json:"symbol"`
	Granularity    int64           `json:"granularity"`
	TimePoint      int64           `json:"timePoint"`
	Value          decimal.Decimal `json:"value"`
	PredictedValue decimal.Decimal `json:"predictedValue"`
}

// tradeWire mirrors one element of /api/v1/trade/history. ts is in
// nanoseconds; size is in contracts.
type tradeWire struct {
	TradeID string          `json:"tradeId"`
	Price   decimal.Decimal `json:"price"`
	Size    decimal.Decimal `json:"size"`
	Side    string          `json:"side"`
	Ts      int64           `json:"ts"`
}

func (w tradeWire) toTradeUpdate(symbol string) roottypes.TradeUpdate {
	return roottypes.TradeUpdate{
		Symbol:  symbol,
		Price:   w.Price,
		Size:    w.Size,
		Side:    roottypes.SideType(w.Side),
		TradeID: w.TradeID,
		TsMs:    nsToMs(w.Ts),
	}
}

// pairsToLevels converts [price, size] decimal pairs into book levels,
// skipping malformed rows.
func pairsToLevels(pairs [][]decimal.Decimal) []roottypes.OrderBookLevel {
	if len(pairs) == 0 {
		return nil
	}
	var out []roottypes.OrderBookLevel = make([]roottypes.OrderBookLevel, 0, len(pairs))
	var i int
	for i = 0; i < len(pairs); i++ {
		if len(pairs[i]) < 2 {
			continue
		}
		out = append(out, roottypes.OrderBookLevel{Price: pairs[i][0], Size: pairs[i][1]})
	}
	return out
}

// klinesFromRows maps [time, open, high, low, close, volume] rows (numbers)
// into the typed Candles slice. time is in milliseconds.
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
			OpenTimeMs: r[0].IntPart(),
			Open:       r[1],
			High:       r[2],
			Low:        r[3],
			Close:      r[4],
			Volume:     r[5],
		})
	}
	return out
}
