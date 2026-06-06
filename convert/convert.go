/*
FILE: convert/convert.go

DESCRIPTION:
KuCoin Convert: public pair/currency directories, market quote + order (+detail
/history), and the limit-order lifecycle (quote/place/detail/list/cancel).

ENDPOINTS:
  - GET    /api/v1/convert/symbol             GetSymbol (public)
  - GET    /api/v1/convert/currencies         GetCurrencies (public)
  - GET    /api/v1/convert/quote              GetQuote
  - POST   /api/v1/convert/order              PlaceMarketOrder
  - GET    /api/v1/convert/order/detail       GetOrder
  - GET    /api/v1/convert/order/history      GetOrderHistory
  - GET    /api/v1/convert/limit/quote        GetLimitQuote
  - POST   /api/v1/convert/limit/order        PlaceLimitOrder
  - GET    /api/v1/convert/limit/order/detail GetLimitOrder
  - GET    /api/v1/convert/limit/orders       GetLimitOrders
  - DELETE /api/v1/convert/limit/order/cancel CancelLimitOrder
*/

package convert

import (
	"context"

	"github.com/shopspring/decimal"

	convtypes "github.com/tonymontanov/go-kucoin/v2/convert/types"
)

// GetSymbol returns the convertible-pair limits for a from→to direction.
// orderType ("MARKET"/"LIMIT") is optional. Public.
func (c *Client) GetSymbol(ctx context.Context, fromCurrency, toCurrency, orderType string) (*convtypes.Symbol, error) {
	if fromCurrency == "" || toCurrency == "" {
		return nil, errInvalidRequest("GetSymbol", "fromCurrency and toCurrency are required")
	}
	var query map[string]string = map[string]string{"fromCurrency": fromCurrency, "toCurrency": toCurrency}
	if orderType != "" {
		query["orderType"] = orderType
	}
	var wire symbolWire
	if err := c.doPublicGET(ctx, "/api/v1/convert/symbol", query, &wire); err != nil {
		return nil, err
	}
	var s convtypes.Symbol = wire.toSymbol()
	return &s, nil
}

// GetCurrencies returns the convertible currency directory. Public.
func (c *Client) GetCurrencies(ctx context.Context) (*convtypes.Currencies, error) {
	var wire currenciesWire
	if err := c.doPublicGET(ctx, "/api/v1/convert/currencies", nil, &wire); err != nil {
		return nil, err
	}
	var out convtypes.Currencies = wire.toCurrencies()
	return &out, nil
}

// GetQuote returns a market convert quote. Supply exactly one of
// FromCurrencySize / ToCurrencySize.
func (c *Client) GetQuote(ctx context.Context, req convtypes.QuoteRequest) (*convtypes.Quote, error) {
	var query, err = quoteQuery("GetQuote", req)
	if err != nil {
		return nil, err
	}
	var wire quoteWire
	if err = c.doGET(ctx, "/api/v1/convert/quote", query, &wire); err != nil {
		return nil, err
	}
	return &convtypes.Quote{
		QuoteID:          wire.QuoteID,
		Price:            wire.Price,
		FromCurrencySize: wire.FromCurrencySize,
		ToCurrencySize:   wire.ToCurrencySize,
		ValidUntil:       wire.ValidUntil,
	}, nil
}

// GetLimitQuote returns the protection-price threshold for a limit order.
func (c *Client) GetLimitQuote(ctx context.Context, req convtypes.QuoteRequest) (*convtypes.LimitQuote, error) {
	var query, err = quoteQuery("GetLimitQuote", req)
	if err != nil {
		return nil, err
	}
	var wire limitQuoteWire
	if err = c.doGET(ctx, "/api/v1/convert/limit/quote", query, &wire); err != nil {
		return nil, err
	}
	return &convtypes.LimitQuote{Price: wire.Price, ValidUntil: wire.ValidUntil}, nil
}

// PlaceMarketOrder places a market convert order against a fresh quote.
func (c *Client) PlaceMarketOrder(ctx context.Context, req convtypes.PlaceMarketRequest) (*convtypes.PlaceResult, error) {
	if req.ClientOrderID == "" || req.QuoteID == "" {
		return nil, errInvalidRequest("PlaceMarketOrder", "clientOrderId and quoteId are required")
	}
	var body map[string]any = map[string]any{
		"clientOrderId": req.ClientOrderID,
		"quoteId":       req.QuoteID,
	}
	if req.AccountType != "" {
		body["accountType"] = req.AccountType
	}
	return c.placeOrder(ctx, "/api/v1/convert/order", body)
}

// PlaceLimitOrder places a limit convert order. The implied price must be ≥ the
// GetLimitQuote protection price.
func (c *Client) PlaceLimitOrder(ctx context.Context, req convtypes.PlaceLimitRequest) (*convtypes.PlaceResult, error) {
	if req.ClientOrderID == "" || req.FromCurrency == "" || req.ToCurrency == "" {
		return nil, errInvalidRequest("PlaceLimitOrder", "clientOrderId, fromCurrency and toCurrency are required")
	}
	if req.FromCurrencySize == "" || req.ToCurrencySize == "" {
		return nil, errInvalidRequest("PlaceLimitOrder", "fromCurrencySize and toCurrencySize are required")
	}
	var body map[string]any = map[string]any{
		"clientOrderId":    req.ClientOrderID,
		"fromCurrency":     req.FromCurrency,
		"toCurrency":       req.ToCurrency,
		"fromCurrencySize": req.FromCurrencySize,
		"toCurrencySize":   req.ToCurrencySize,
	}
	if req.AccountType != "" {
		body["accountType"] = req.AccountType
	}
	return c.placeOrder(ctx, "/api/v1/convert/limit/order", body)
}

// CancelLimitOrder cancels an open limit convert order by clientOrderId.
func (c *Client) CancelLimitOrder(ctx context.Context, clientOrderID string) error {
	if clientOrderID == "" {
		return errInvalidRequest("CancelLimitOrder", "clientOrderId is required")
	}
	return c.doDELETE(ctx, "/api/v1/convert/limit/order/cancel", map[string]any{"clientOrderId": clientOrderID}, nil)
}

// GetOrder returns a market convert order's detail. Supply orderID or
// clientOrderID (at least one).
func (c *Client) GetOrder(ctx context.Context, orderID, clientOrderID string) (*convtypes.Order, error) {
	var query, err = idQuery("GetOrder", orderID, clientOrderID)
	if err != nil {
		return nil, err
	}
	var wire orderWire
	if err = c.doGET(ctx, "/api/v1/convert/order/detail", query, &wire); err != nil {
		return nil, err
	}
	var o convtypes.Order = wire.toOrder()
	return &o, nil
}

// GetLimitOrder returns a limit convert order's detail. Supply orderID or
// clientOrderID (at least one).
func (c *Client) GetLimitOrder(ctx context.Context, orderID, clientOrderID string) (*convtypes.LimitOrder, error) {
	var query, err = idQuery("GetLimitOrder", orderID, clientOrderID)
	if err != nil {
		return nil, err
	}
	var wire limitOrderWire
	if err = c.doGET(ctx, "/api/v1/convert/limit/order/detail", query, &wire); err != nil {
		return nil, err
	}
	var o convtypes.LimitOrder = wire.toLimitOrder()
	return &o, nil
}

// GetOrderHistory returns paginated market convert order history.
func (c *Client) GetOrderHistory(ctx context.Context, q convtypes.HistoryQuery) (*convtypes.OrderPage, error) {
	var wire orderPageWire
	if err := c.doGET(ctx, "/api/v1/convert/order/history", historyQuery(q), &wire); err != nil {
		return nil, err
	}
	var items []convtypes.Order = make([]convtypes.Order, len(wire.Items))
	var i int
	for i = 0; i < len(wire.Items); i++ {
		items[i] = wire.Items[i].toOrder()
	}
	return &convtypes.OrderPage{
		CurrentPage: wire.CurrentPage, PageSize: wire.PageSize,
		TotalNum: wire.TotalNum, TotalPage: wire.TotalPage, Items: items,
	}, nil
}

// GetLimitOrders returns paginated (active + historical) limit convert orders.
func (c *Client) GetLimitOrders(ctx context.Context, q convtypes.HistoryQuery) (*convtypes.LimitOrderPage, error) {
	var wire limitOrderPageWire
	if err := c.doGET(ctx, "/api/v1/convert/limit/orders", historyQuery(q), &wire); err != nil {
		return nil, err
	}
	var items []convtypes.LimitOrder = make([]convtypes.LimitOrder, len(wire.Items))
	var i int
	for i = 0; i < len(wire.Items); i++ {
		items[i] = wire.Items[i].toLimitOrder()
	}
	return &convtypes.LimitOrderPage{
		CurrentPage: wire.CurrentPage, PageSize: wire.PageSize,
		TotalNum: wire.TotalNum, TotalPage: wire.TotalPage, Items: items,
	}, nil
}

// ---------------------------------------------------------------------
// shared internals
// ---------------------------------------------------------------------

func (c *Client) placeOrder(ctx context.Context, path string, body map[string]any) (*convtypes.PlaceResult, error) {
	var wire placeResultWire
	if err := c.doPOST(ctx, path, body, &wire); err != nil {
		return nil, err
	}
	return &convtypes.PlaceResult{ClientOrderID: wire.ClientOrderID, OrderID: string(wire.OrderID)}, nil
}

func quoteQuery(method string, req convtypes.QuoteRequest) (map[string]string, error) {
	if req.FromCurrency == "" || req.ToCurrency == "" {
		return nil, errInvalidRequest(method, "fromCurrency and toCurrency are required")
	}
	if req.FromCurrencySize == "" && req.ToCurrencySize == "" {
		return nil, errInvalidRequest(method, "one of fromCurrencySize / toCurrencySize is required")
	}
	var query map[string]string = map[string]string{"fromCurrency": req.FromCurrency, "toCurrency": req.ToCurrency}
	if req.FromCurrencySize != "" {
		query["fromCurrencySize"] = req.FromCurrencySize
	}
	if req.ToCurrencySize != "" {
		query["toCurrencySize"] = req.ToCurrencySize
	}
	return query, nil
}

func idQuery(method, orderID, clientOrderID string) (map[string]string, error) {
	if orderID == "" && clientOrderID == "" {
		return nil, errInvalidRequest(method, "orderId or clientOrderId is required")
	}
	var query map[string]string = map[string]string{}
	if orderID != "" {
		query["orderId"] = orderID
	}
	if clientOrderID != "" {
		query["clientOrderId"] = clientOrderID
	}
	return query, nil
}

func historyQuery(q convtypes.HistoryQuery) map[string]string {
	var query map[string]string = map[string]string{}
	if q.StartAt > 0 {
		query["startAt"] = itoa64(q.StartAt)
	}
	if q.EndAt > 0 {
		query["endAt"] = itoa64(q.EndAt)
	}
	if q.Page > 0 {
		query["page"] = itoa(q.Page)
	}
	if q.PageSize > 0 {
		query["pageSize"] = itoa(q.PageSize)
	}
	if q.Status != "" {
		query["status"] = q.Status
	}
	return query
}

// ---------------------------------------------------------------------
// wire structs + converters
// ---------------------------------------------------------------------

type symbolWire struct {
	FromCurrency        string          `json:"fromCurrency"`
	ToCurrency          string          `json:"toCurrency"`
	FromCurrencyMaxSize decimal.Decimal `json:"fromCurrencyMaxSize"`
	FromCurrencyMinSize decimal.Decimal `json:"fromCurrencyMinSize"`
	FromCurrencyStep    decimal.Decimal `json:"fromCurrencyStep"`
	ToCurrencyMaxSize   decimal.Decimal `json:"toCurrencyMaxSize"`
	ToCurrencyMinSize   decimal.Decimal `json:"toCurrencyMinSize"`
	ToCurrencyStep      decimal.Decimal `json:"toCurrencyStep"`
}

func (w symbolWire) toSymbol() convtypes.Symbol {
	return convtypes.Symbol{
		FromCurrency:        w.FromCurrency,
		ToCurrency:          w.ToCurrency,
		FromCurrencyMaxSize: w.FromCurrencyMaxSize,
		FromCurrencyMinSize: w.FromCurrencyMinSize,
		FromCurrencyStep:    w.FromCurrencyStep,
		ToCurrencyMaxSize:   w.ToCurrencyMaxSize,
		ToCurrencyMinSize:   w.ToCurrencyMinSize,
		ToCurrencyStep:      w.ToCurrencyStep,
	}
}

type currencyLimitWire struct {
	Currency       string          `json:"currency"`
	MaxSize        decimal.Decimal `json:"maxSize"`
	MinSize        decimal.Decimal `json:"minSize"`
	Step           decimal.Decimal `json:"step"`
	TradeDirection string          `json:"tradeDirection"`
}

func (w currencyLimitWire) toLimit() convtypes.CurrencyLimit {
	return convtypes.CurrencyLimit{
		Currency:       w.Currency,
		MaxSize:        w.MaxSize,
		MinSize:        w.MinSize,
		Step:           w.Step,
		TradeDirection: w.TradeDirection,
	}
}

type currenciesWire struct {
	Currencies        []currencyLimitWire `json:"currencies"`
	USDTCurrencyLimit []currencyLimitWire `json:"usdtCurrencyLimit"`
}

func (w currenciesWire) toCurrencies() convtypes.Currencies {
	var conv = func(in []currencyLimitWire) []convtypes.CurrencyLimit {
		var out []convtypes.CurrencyLimit = make([]convtypes.CurrencyLimit, len(in))
		var i int
		for i = 0; i < len(in); i++ {
			out[i] = in[i].toLimit()
		}
		return out
	}
	return convtypes.Currencies{
		Currencies:        conv(w.Currencies),
		USDTCurrencyLimit: conv(w.USDTCurrencyLimit),
	}
}

type quoteWire struct {
	QuoteID          string          `json:"quoteId"`
	Price            decimal.Decimal `json:"price"`
	FromCurrencySize decimal.Decimal `json:"fromCurrencySize"`
	ToCurrencySize   decimal.Decimal `json:"toCurrencySize"`
	ValidUntil       int64           `json:"validUntill"`
}

type limitQuoteWire struct {
	Price      decimal.Decimal `json:"price"`
	ValidUntil int64           `json:"validUntill"`
}

type placeResultWire struct {
	ClientOrderID string  `json:"clientOrderId"`
	OrderID       flexStr `json:"orderId"`
}

type orderWire struct {
	ClientOrderID    string          `json:"clientOrderId"`
	OrderID          flexStr         `json:"orderId"`
	Price            decimal.Decimal `json:"price"`
	FromCurrency     string          `json:"fromCurrency"`
	ToCurrency       string          `json:"toCurrency"`
	FromCurrencySize decimal.Decimal `json:"fromCurrencySize"`
	ToCurrencySize   decimal.Decimal `json:"toCurrencySize"`
	AccountType      string          `json:"accountType"`
	OrderTime        int64           `json:"orderTime"`
	Status           string          `json:"status"`
}

func (w orderWire) toOrder() convtypes.Order {
	return convtypes.Order{
		ClientOrderID:    w.ClientOrderID,
		OrderID:          string(w.OrderID),
		Price:            w.Price,
		FromCurrency:     w.FromCurrency,
		ToCurrency:       w.ToCurrency,
		FromCurrencySize: w.FromCurrencySize,
		ToCurrencySize:   w.ToCurrencySize,
		AccountType:      w.AccountType,
		OrderTime:        w.OrderTime,
		Status:           w.Status,
	}
}

type limitOrderWire struct {
	ClientOrderID    string          `json:"clientOrderId"`
	OrderID          flexStr         `json:"orderId"`
	Price            decimal.Decimal `json:"price"`
	FromCurrency     string          `json:"fromCurrency"`
	ToCurrency       string          `json:"toCurrency"`
	FromCurrencySize decimal.Decimal `json:"fromCurrencySize"`
	ToCurrencySize   decimal.Decimal `json:"toCurrencySize"`
	AccountType      string          `json:"accountType"`
	OrderTime        int64           `json:"orderTime"`
	Status           string          `json:"status"`
	ExpiryTime       int64           `json:"expiryTime"`
	CancelTime       int64           `json:"cancelTime"`
	FilledTime       int64           `json:"filledTime"`
	CancelType       int             `json:"cancelType"`
}

func (w limitOrderWire) toLimitOrder() convtypes.LimitOrder {
	return convtypes.LimitOrder{
		ClientOrderID:    w.ClientOrderID,
		OrderID:          string(w.OrderID),
		Price:            w.Price,
		FromCurrency:     w.FromCurrency,
		ToCurrency:       w.ToCurrency,
		FromCurrencySize: w.FromCurrencySize,
		ToCurrencySize:   w.ToCurrencySize,
		AccountType:      w.AccountType,
		OrderTime:        w.OrderTime,
		Status:           w.Status,
		ExpiryTime:       w.ExpiryTime,
		CancelTime:       w.CancelTime,
		FilledTime:       w.FilledTime,
		CancelType:       w.CancelType,
	}
}

type orderPageWire struct {
	CurrentPage int         `json:"currentPage"`
	PageSize    int         `json:"pageSize"`
	TotalNum    int         `json:"totalNum"`
	TotalPage   int         `json:"totalPage"`
	Items       []orderWire `json:"items"`
}

type limitOrderPageWire struct {
	CurrentPage int              `json:"currentPage"`
	PageSize    int              `json:"pageSize"`
	TotalNum    int              `json:"totalNum"`
	TotalPage   int              `json:"totalPage"`
	Items       []limitOrderWire `json:"items"`
}
