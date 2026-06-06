/*
FILE: margin/market.go

DESCRIPTION:
Public market-data sub-client for the KuCoin Margin profile. Covers the
margin-SPECIFIC market data on api.kucoin.com and converts the wire JSON into
the SDK's typed structs (decimal-normalised, ms timestamps).

ENDPOINTS:
  - GET /api/v3/margin/symbols                  cross-margin symbol config
  - GET /api/v1/isolated/symbols                isolated-margin pair config
  - GET /api/v1/mark-price/{symbol}/current     single mark price
  - GET /api/v3/mark-price/all-symbols          all mark prices
  - GET /api/v1/margin/config                   global cross-margin config

NB: the live order book, ticker and trade tape for margin pairs are IDENTICAL
to Spot (same matching engine, same WS topics). Use the spot profile for
those — this sub-client only adds the margin-specific data above.

WIRE NOTES: numerics ship as JSON STRINGS (decimal decodes them); the cross
symbols response wraps its rows under {timestamp, items}; mark prices carry
the time in MILLISECONDS (timePoint).
*/

package margin

import (
	"context"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
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

// GetCrossSymbols returns the cross-margin symbol configuration. Pass an empty
// symbol for every pair.
func (m *MarketDataClient) GetCrossSymbols(ctx context.Context, symbol string) ([]margintypes.SymbolInfo, error) {
	var query map[string]string
	if symbol != "" {
		query = map[string]string{"symbol": symbol}
	}
	var wire crossSymbolsWire
	if err := m.c.doGET(ctx, false, "/api/v3/margin/symbols", query, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []margintypes.SymbolInfo = make([]margintypes.SymbolInfo, len(wire.Items))
	var i int
	for i = 0; i < len(wire.Items); i++ {
		out[i] = wire.Items[i].toSymbolInfo()
	}
	return out, nil
}

// GetIsolatedSymbols returns the isolated-margin pair configuration.
func (m *MarketDataClient) GetIsolatedSymbols(ctx context.Context) ([]margintypes.IsolatedSymbol, error) {
	var wire []isolatedSymbolWire
	if err := m.c.doGET(ctx, false, "/api/v1/isolated/symbols", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []margintypes.IsolatedSymbol = make([]margintypes.IsolatedSymbol, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toIsolatedSymbol()
	}
	return out, nil
}

// GetMarkPrice returns the current mark price for a single margin pair.
func (m *MarketDataClient) GetMarkPrice(ctx context.Context, symbol string) (*margintypes.MarkPrice, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetMarkPrice", "symbol is required")
	}
	var wire markPriceWire
	if err := m.c.doGET(ctx, false, "/api/v1/mark-price/"+symbol+"/current", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var mp margintypes.MarkPrice = wire.toMarkPrice()
	return &mp, nil
}

// GetMarkPrices returns the mark price for every margin pair.
func (m *MarketDataClient) GetMarkPrices(ctx context.Context) ([]margintypes.MarkPrice, error) {
	var wire []markPriceWire
	if err := m.c.doGET(ctx, false, "/api/v3/mark-price/all-symbols", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []margintypes.MarkPrice = make([]margintypes.MarkPrice, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toMarkPrice()
	}
	return out, nil
}

// GetMarginConfig returns the global cross-margin configuration.
func (m *MarketDataClient) GetMarginConfig(ctx context.Context) (*margintypes.MarginConfig, error) {
	var wire marginConfigWire
	if err := m.c.doGET(ctx, false, "/api/v1/margin/config", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var cfg margintypes.MarginConfig = wire.toMarginConfig()
	return &cfg, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

// crossSymbolsWire mirrors /api/v3/margin/symbols (rows nested under items).
type crossSymbolsWire struct {
	Timestamp int64             `json:"timestamp"`
	Items     []crossSymbolWire `json:"items"`
}

// crossSymbolWire mirrors one cross-margin symbol. All numerics are strings.
type crossSymbolWire struct {
	Symbol         string          `json:"symbol"`
	Name           string          `json:"name"`
	BaseCurrency   string          `json:"baseCurrency"`
	QuoteCurrency  string          `json:"quoteCurrency"`
	FeeCurrency    string          `json:"feeCurrency"`
	Market         string          `json:"market"`
	BaseMinSize    decimal.Decimal `json:"baseMinSize"`
	QuoteMinSize   decimal.Decimal `json:"quoteMinSize"`
	BaseMaxSize    decimal.Decimal `json:"baseMaxSize"`
	QuoteMaxSize   decimal.Decimal `json:"quoteMaxSize"`
	BaseIncrement  decimal.Decimal `json:"baseIncrement"`
	QuoteIncrement decimal.Decimal `json:"quoteIncrement"`
	PriceIncrement decimal.Decimal `json:"priceIncrement"`
	PriceLimitRate decimal.Decimal `json:"priceLimitRate"`
	MinFunds       decimal.Decimal `json:"minFunds"`
	EnableTrading  bool            `json:"enableTrading"`
}

func (w crossSymbolWire) toSymbolInfo() margintypes.SymbolInfo {
	return margintypes.SymbolInfo{
		Symbol:         w.Symbol,
		Name:           w.Name,
		BaseCurrency:   w.BaseCurrency,
		QuoteCurrency:  w.QuoteCurrency,
		FeeCurrency:    w.FeeCurrency,
		Market:         w.Market,
		BaseMinSize:    w.BaseMinSize,
		BaseMaxSize:    w.BaseMaxSize,
		QuoteMinSize:   w.QuoteMinSize,
		QuoteMaxSize:   w.QuoteMaxSize,
		BaseIncrement:  w.BaseIncrement,
		QuoteIncrement: w.QuoteIncrement,
		PriceIncrement: w.PriceIncrement,
		PriceLimitRate: w.PriceLimitRate,
		MinFunds:       w.MinFunds,
		EnableTrading:  w.EnableTrading,
	}
}

// isolatedSymbolWire mirrors one element of /api/v1/isolated/symbols.
type isolatedSymbolWire struct {
	Symbol                string          `json:"symbol"`
	SymbolName            string          `json:"symbolName"`
	BaseCurrency          string          `json:"baseCurrency"`
	QuoteCurrency         string          `json:"quoteCurrency"`
	MaxLeverage           int             `json:"maxLeverage"`
	FlDebtRatio           decimal.Decimal `json:"flDebtRatio"`
	TradeEnable           bool            `json:"tradeEnable"`
	AutoRenewMaxDebtRatio decimal.Decimal `json:"autoRenewMaxDebtRatio"`
	BaseBorrowEnable      bool            `json:"baseBorrowEnable"`
	QuoteBorrowEnable     bool            `json:"quoteBorrowEnable"`
	BaseTransferInEnable  bool            `json:"baseTransferInEnable"`
	QuoteTransferInEnable bool            `json:"quoteTransferInEnable"`
}

func (w isolatedSymbolWire) toIsolatedSymbol() margintypes.IsolatedSymbol {
	return margintypes.IsolatedSymbol{
		Symbol:                w.Symbol,
		SymbolName:            w.SymbolName,
		BaseCurrency:          w.BaseCurrency,
		QuoteCurrency:         w.QuoteCurrency,
		MaxLeverage:           w.MaxLeverage,
		FlDebtRatio:           w.FlDebtRatio,
		TradeEnable:           w.TradeEnable,
		AutoRenewMaxDebtRatio: w.AutoRenewMaxDebtRatio,
		BaseBorrowEnable:      w.BaseBorrowEnable,
		QuoteBorrowEnable:     w.QuoteBorrowEnable,
		BaseTransferInEnable:  w.BaseTransferInEnable,
		QuoteTransferInEnable: w.QuoteTransferInEnable,
	}
}

// markPriceWire mirrors /api/v1/mark-price/{symbol}/current and one element of
// /api/v3/mark-price/all-symbols. timePoint is ms.
type markPriceWire struct {
	Symbol    string          `json:"symbol"`
	Value     decimal.Decimal `json:"value"`
	TimePoint int64           `json:"timePoint"`
}

func (w markPriceWire) toMarkPrice() margintypes.MarkPrice {
	return margintypes.MarkPrice{
		Symbol: w.Symbol,
		Value:  w.Value,
		TsMs:   w.TimePoint,
	}
}

// marginConfigWire mirrors /api/v1/margin/config.
type marginConfigWire struct {
	CurrencyList     []string        `json:"currencyList"`
	MaxLeverage      int             `json:"maxLeverage"`
	WarningDebtRatio decimal.Decimal `json:"warningDebtRatio"`
	LiqDebtRatio     decimal.Decimal `json:"liqDebtRatio"`
}

func (w marginConfigWire) toMarginConfig() margintypes.MarginConfig {
	return margintypes.MarginConfig{
		CurrencyList:     w.CurrencyList,
		MaxLeverage:      w.MaxLeverage,
		WarningDebtRatio: w.WarningDebtRatio,
		LiqDebtRatio:     w.LiqDebtRatio,
	}
}
