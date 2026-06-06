/*
FILE: spot/trading.go

DESCRIPTION:
Signed trading sub-client for the KuCoin Spot profile. Covers the v2.0 order
lifecycle on api.kucoin.com:

  - POST   /api/v1/orders                          place (limit/market)
  - POST   /api/v1/orders/multi                     batch place (same symbol)
  - DELETE /api/v1/orders/{orderId}                 cancel by order id
  - DELETE /api/v1/order/client-order/{clientOid}   cancel by client id
  - DELETE /api/v1/orders?symbol=                    cancel all
  - GET    /api/v1/orders/{orderId}                  order detail by id
  - GET    /api/v1/order/client-order/{clientOid}    order detail by client id
  - GET    /api/v1/orders?status=&symbol=...          order list (paginated)
  - GET    /api/v1/fills                              fills (paginated)
  - GET    /api/v1/limit/fills                        recent fills

SIZING: order Size is in BASE currency (decimal); market orders may instead
pass Funds (quote currency). There is no contract multiplier.
*/

package spot

import (
	"context"
	"encoding/json"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

// TradingClient — signed trading sub-client.
type TradingClient struct {
	c *Client
}

// newTradingClient wires the sub-client to its parent.
func newTradingClient(c *Client) *TradingClient {
	return &TradingClient{c: c}
}

// OrderAck — acknowledgement returned by PlaceOrder.
type OrderAck struct {
	OrderID       string
	ClientOrderID string
}

// GetOrdersParams — filter / pagination for GetOrders.
type GetOrdersParams struct {
	// Status — "active" or "done"; empty returns all.
	Status string
	Symbol string
	// Side — "buy"/"sell"; empty for both.
	Side spottypes.SideType
	// Type — "limit"/"market"; empty for all.
	Type spottypes.OrderType
	// TradeType — TRADE (default) / MARGIN_TRADE.
	TradeType spottypes.TradeType
	// StartAtMs / EndAtMs — time window (ms).
	StartAtMs int64
	EndAtMs   int64
	// CurrentPage / PageSize — pagination (1-based; 0 lets KuCoin default).
	CurrentPage int
	PageSize    int
}

// GetFillsParams — filter / pagination for GetFills.
type GetFillsParams struct {
	OrderID     string
	Symbol      string
	Side        spottypes.SideType
	Type        spottypes.OrderType
	TradeType   spottypes.TradeType
	StartAtMs   int64
	EndAtMs     int64
	CurrentPage int
	PageSize    int
}

// queryMeta is the rate-limit metadata for read-only trading queries.
var queryMeta = rest.RequestMeta{Category: "query"}

// ---------------------------------------------------------------------
// Place.
// ---------------------------------------------------------------------

// PlaceOrder places a single order (limit / market). It fills the clientOid
// and trade-type defaults, validates the request and returns the assigned
// order id together with the clientOid actually used.
func (t *TradingClient) PlaceOrder(ctx context.Context, req spottypes.CreateOrderRequest) (*OrderAck, error) {
	var body placeOrderBody
	var err error
	body, err = t.c.buildOrderBody(req)
	if err != nil {
		return nil, err
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Symbols: []string{req.Symbol}, Category: "place"}
	var ack ackWire
	if err = t.c.doPOST(ctx, "/api/v1/orders", body, meta, &ack); err != nil {
		return nil, err
	}
	return &OrderAck{OrderID: ack.OrderID, ClientOrderID: body.ClientOid}, nil
}

// PlaceBatchOrders places multiple orders for ONE symbol in a single request
// (POST /api/v1/orders/multi, max 5 per call). All requests must target the
// same symbol — KuCoin scopes the batch endpoint per symbol. The returned
// slice has one row per submitted order; inspect Success / Status per row to
// reconcile a partial batch.
func (t *TradingClient) PlaceBatchOrders(ctx context.Context, reqs []spottypes.CreateOrderRequest) ([]spottypes.BatchOrderResult, error) {
	if len(reqs) == 0 {
		return nil, errInvalidRequest("PlaceBatchOrders", "at least one order is required")
	}
	var symbol string = reqs[0].Symbol
	var items []placeOrderBody = make([]placeOrderBody, len(reqs))
	var i int
	for i = 0; i < len(reqs); i++ {
		if reqs[i].Symbol != symbol {
			return nil, errInvalidRequest("PlaceBatchOrders", "all orders in a batch must share one symbol")
		}
		var b placeOrderBody
		var err error
		b, err = t.c.buildOrderBody(reqs[i])
		if err != nil {
			return nil, err
		}
		items[i] = b
	}
	var body multiOrderBody = multiOrderBody{Symbol: symbol, OrderList: items}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: len(reqs), Symbols: []string{symbol}, Category: "place"}
	// KuCoin wraps the multi-order result in an EXTRA "data" object:
	// envelope.data = {"data":[{row}...]}. Decode the raw envelope payload
	// and unwrap tolerantly — older/sandbox variants returned a bare array,
	// so accept both shapes (see decodeBatchRows).
	var raw json.RawMessage
	if err := t.c.doPOST(ctx, "/api/v1/orders/multi", body, meta, &raw); err != nil {
		return nil, err
	}
	var rows []batchRowWire
	var err error
	rows, err = decodeBatchRows(raw)
	if err != nil {
		return nil, err
	}
	var out []spottypes.BatchOrderResult = make([]spottypes.BatchOrderResult, len(rows))
	for i = 0; i < len(rows); i++ {
		out[i] = spottypes.BatchOrderResult{
			OrderID:       rows[i].orderID(),
			ClientOrderID: rows[i].ClientOid,
			Symbol:        rows[i].Symbol,
			Success:       rows[i].Status == "" || rows[i].Status == "success",
			Status:        rows[i].Status,
			FailMsg:       rows[i].FailMsg,
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Cancel.
// ---------------------------------------------------------------------

// CancelOrder cancels a single order by KuCoin order id. Returns the ids
// KuCoin reports as cancelled.
func (t *TradingClient) CancelOrder(ctx context.Context, orderID string) ([]string, error) {
	if orderID == "" {
		return nil, errInvalidRequest("CancelOrder", "orderID is required")
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Category: "cancel"}
	var res cancelIDsWire
	if err := t.c.doDELETE(ctx, "/api/v1/orders/"+orderID, nil, meta, &res); err != nil {
		return nil, err
	}
	return res.CancelledOrderIDs, nil
}

// CancelOrderByClientOid cancels a single order by clientOid. Returns the
// cancelled order id reported by KuCoin.
func (t *TradingClient) CancelOrderByClientOid(ctx context.Context, clientOid string) (string, error) {
	if clientOid == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "clientOid is required")
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Category: "cancel"}
	var res cancelClientOidWire
	if err := t.c.doDELETE(ctx, "/api/v1/order/client-order/"+clientOid, nil, meta, &res); err != nil {
		return "", err
	}
	return res.CancelledOrderID, nil
}

// CancelAllOrders cancels all orders, optionally scoped to a symbol. The
// trade type defaults to the client default (TRADE).
func (t *TradingClient) CancelAllOrders(ctx context.Context, symbol string) ([]string, error) {
	var query map[string]string = map[string]string{"tradeType": string(t.c.defaultTradeType)}
	var symbols []string
	if symbol != "" {
		query["symbol"] = symbol
		symbols = []string{symbol}
	}
	var meta rest.RequestMeta = rest.RequestMeta{Symbols: symbols, Category: "cancel"}
	var res cancelIDsWire
	if err := t.c.doDELETE(ctx, "/api/v1/orders", query, meta, &res); err != nil {
		return nil, err
	}
	return res.CancelledOrderIDs, nil
}

// ---------------------------------------------------------------------
// Queries.
// ---------------------------------------------------------------------

// GetOrder returns a single order by KuCoin order id.
func (t *TradingClient) GetOrder(ctx context.Context, orderID string) (*spottypes.OrderInfo, error) {
	if orderID == "" {
		return nil, errInvalidRequest("GetOrder", "orderID is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v1/orders/"+orderID, nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info spottypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOrderByClientOid returns a single order by clientOid.
func (t *TradingClient) GetOrderByClientOid(ctx context.Context, clientOid string) (*spottypes.OrderInfo, error) {
	if clientOid == "" {
		return nil, errInvalidRequest("GetOrderByClientOid", "clientOid is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v1/order/client-order/"+clientOid, nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info spottypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOrders returns a page of orders matching the filter.
func (t *TradingClient) GetOrders(ctx context.Context, p GetOrdersParams) ([]spottypes.OrderInfo, error) {
	var query map[string]string = map[string]string{}
	if p.Status != "" {
		query["status"] = p.Status
	}
	if p.Symbol != "" {
		query["symbol"] = p.Symbol
	}
	if p.Side != "" {
		query["side"] = string(p.Side)
	}
	if p.Type != "" {
		query["type"] = string(p.Type)
	}
	if p.TradeType != "" {
		query["tradeType"] = string(p.TradeType)
	} else {
		query["tradeType"] = string(t.c.defaultTradeType)
	}
	if p.StartAtMs > 0 {
		query["startAt"] = itoa(p.StartAtMs)
	}
	if p.EndAtMs > 0 {
		query["endAt"] = itoa(p.EndAtMs)
	}
	if p.CurrentPage > 0 {
		query["currentPage"] = itoa(int64(p.CurrentPage))
	}
	if p.PageSize > 0 {
		query["pageSize"] = itoa(int64(p.PageSize))
	}
	var page orderPageWire
	if err := t.c.doGET(ctx, true, "/api/v1/orders", query, queryMeta, &page); err != nil {
		return nil, err
	}
	return ordersFromWire(page.Items), nil
}

// GetOpenOrders is a convenience wrapper for GetOrders(status="active").
func (t *TradingClient) GetOpenOrders(ctx context.Context, symbol string) ([]spottypes.OrderInfo, error) {
	return t.GetOrders(ctx, GetOrdersParams{Status: "active", Symbol: symbol})
}

// GetFills returns a page of fills matching the filter.
func (t *TradingClient) GetFills(ctx context.Context, p GetFillsParams) ([]spottypes.Fill, error) {
	var query map[string]string = map[string]string{}
	if p.OrderID != "" {
		query["orderId"] = p.OrderID
	}
	if p.Symbol != "" {
		query["symbol"] = p.Symbol
	}
	if p.Side != "" {
		query["side"] = string(p.Side)
	}
	if p.Type != "" {
		query["type"] = string(p.Type)
	}
	if p.TradeType != "" {
		query["tradeType"] = string(p.TradeType)
	} else {
		query["tradeType"] = string(t.c.defaultTradeType)
	}
	if p.StartAtMs > 0 {
		query["startAt"] = itoa(p.StartAtMs)
	}
	if p.EndAtMs > 0 {
		query["endAt"] = itoa(p.EndAtMs)
	}
	if p.CurrentPage > 0 {
		query["currentPage"] = itoa(int64(p.CurrentPage))
	}
	if p.PageSize > 0 {
		query["pageSize"] = itoa(int64(p.PageSize))
	}
	var page fillPageWire
	if err := t.c.doGET(ctx, true, "/api/v1/fills", query, queryMeta, &page); err != nil {
		return nil, err
	}
	return fillsFromWire(page.Items), nil
}

// GetRecentFills returns the most recent fills (last 24h, up to 1000) via the
// cached /api/v1/limit/fills endpoint.
func (t *TradingClient) GetRecentFills(ctx context.Context) ([]spottypes.Fill, error) {
	var rows []fillWire
	if err := t.c.doGET(ctx, true, "/api/v1/limit/fills", nil, queryMeta, &rows); err != nil {
		return nil, err
	}
	return fillsFromWire(rows), nil
}

// ---------------------------------------------------------------------
// Request body assembly.
// ---------------------------------------------------------------------

// placeOrderBody is the KuCoin /api/v1/orders request body. Zero-value
// booleans and empty strings are omitted so KuCoin applies its own defaults.
type placeOrderBody struct {
	ClientOid   string `json:"clientOid"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Type        string `json:"type,omitempty"`
	TradeType   string `json:"tradeType,omitempty"`
	Price       string `json:"price,omitempty"`
	Size        string `json:"size,omitempty"`
	Funds       string `json:"funds,omitempty"`
	TimeInForce string `json:"timeInForce,omitempty"`
	PostOnly    bool   `json:"postOnly,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	Iceberg     bool   `json:"iceberg,omitempty"`
	VisibleSize string `json:"visibleSize,omitempty"`
	CancelAfter int64  `json:"cancelAfter,omitempty"`
	STP         string `json:"stp,omitempty"`
	Remark      string `json:"remark,omitempty"`
}

// multiOrderBody is the KuCoin /api/v1/orders/multi request body: a symbol
// plus a list of per-order bodies (symbol omitted on each item).
type multiOrderBody struct {
	Symbol    string           `json:"symbol"`
	OrderList []placeOrderBody `json:"orderList"`
}

// buildOrderBody validates the request, applies client defaults and maps it
// onto the KuCoin wire body.
func (c *Client) buildOrderBody(req spottypes.CreateOrderRequest) (placeOrderBody, error) {
	var b placeOrderBody
	if req.Symbol == "" {
		return b, errInvalidRequest("PlaceOrder", "symbol is required")
	}
	if req.Side == "" {
		return b, errInvalidRequest("PlaceOrder", "side is required")
	}
	if req.Type == "" {
		return b, errInvalidRequest("PlaceOrder", "type is required")
	}

	switch req.Type {
	case spottypes.OrderLimit:
		if req.Price.LessThanOrEqual(decimal.Zero) {
			return b, errInvalidRequest("PlaceOrder", "price must be > 0 for limit orders")
		}
		if req.Size.LessThanOrEqual(decimal.Zero) {
			return b, errInvalidRequest("PlaceOrder", "size (base) must be > 0 for limit orders")
		}
	case spottypes.OrderMarket:
		var hasSize bool = req.Size.GreaterThan(decimal.Zero)
		var hasFunds bool = req.Funds.GreaterThan(decimal.Zero)
		if hasSize == hasFunds { // both set or both unset
			return b, errInvalidRequest("PlaceOrder", "market order needs exactly one of size (base) or funds (quote)")
		}
	default:
		return b, errInvalidRequest("PlaceOrder", "unsupported order type")
	}

	var tradeType spottypes.TradeType = req.TradeType
	if tradeType == "" {
		tradeType = c.defaultTradeType
	}

	var clientOid string = req.ClientOrderID
	if clientOid == "" {
		clientOid = generateClientOid()
	}

	b = placeOrderBody{
		ClientOid:   clientOid,
		Symbol:      req.Symbol,
		Side:        string(req.Side),
		Type:        string(req.Type),
		TradeType:   string(tradeType),
		TimeInForce: string(req.TimeInForce),
		PostOnly:    req.PostOnly,
		Hidden:      req.Hidden,
		Iceberg:     req.Iceberg,
		CancelAfter: req.CancelAfter,
		STP:         string(req.STP),
		Remark:      req.Remark,
	}
	if req.Type == spottypes.OrderLimit {
		b.Price = req.Price.String()
		b.Size = req.Size.String()
	} else { // market
		if req.Size.GreaterThan(decimal.Zero) {
			b.Size = req.Size.String()
		}
		if req.Funds.GreaterThan(decimal.Zero) {
			b.Funds = req.Funds.String()
		}
	}
	if !req.VisibleSize.IsZero() {
		b.VisibleSize = req.VisibleSize.String()
	}
	return b, nil
}

// ---------------------------------------------------------------------
// Response wire structs + converters.
// ---------------------------------------------------------------------

// ackWire mirrors the place-order response data.
type ackWire struct {
	OrderID string `json:"orderId"`
}

// batchRowWire mirrors one row of the /api/v1/orders/multi response. KuCoin
// has shipped the assigned id under both "orderId" and "id" across versions;
// orderID() prefers whichever is populated.
type batchRowWire struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	ClientOid string `json:"clientOid"`
	Symbol    string `json:"symbol"`
	Status    string `json:"status"`
	FailMsg   string `json:"failMsg"`
}

// multiOrderResultWire is the current /api/v1/orders/multi success payload:
// the row list is nested under an extra "data" key inside the envelope's
// already-unwrapped data object (envelope.data = {"data":[...]}).
type multiOrderResultWire struct {
	Data []batchRowWire `json:"data"`
}

/*
decodeBatchRows unwraps the /api/v1/orders/multi rows tolerantly. KuCoin
currently nests the list under {"data":[...]}, but older/sandbox builds
returned a bare [ ... ] array. Trimmed leading whitespace decides the shape:
'{' → nested wrapper, '[' → bare array. A decode mismatch here previously
surfaced as a place-leg error even though the orders WERE accepted (HTTP
200), which made the strategy retry and flood the book with duplicate
orders — so this must accept both forms.
*/
func decodeBatchRows(raw json.RawMessage) ([]batchRowWire, error) {
	var trimmed []byte = bytesTrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] == '{' {
		var wrapped multiOrderResultWire
		if err := codecUnmarshal(trimmed, &wrapped); err != nil {
			return nil, err
		}
		return wrapped.Data, nil
	}
	var rows []batchRowWire
	if err := codecUnmarshal(trimmed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// bytesTrimSpace strips leading/trailing ASCII whitespace without importing
// bytes for a single call.
func bytesTrimSpace(b []byte) []byte {
	var start int
	for start < len(b) {
		switch b[start] {
		case ' ', '\t', '\n', '\r':
			start++
			continue
		}
		break
	}
	var end int = len(b)
	for end > start {
		switch b[end-1] {
		case ' ', '\t', '\n', '\r':
			end--
			continue
		}
		break
	}
	return b[start:end]
}

func (w batchRowWire) orderID() string {
	if w.OrderID != "" {
		return w.OrderID
	}
	return w.ID
}

// cancelIDsWire mirrors the cancel responses that return cancelledOrderIds.
type cancelIDsWire struct {
	CancelledOrderIDs []string `json:"cancelledOrderIds"`
}

// cancelClientOidWire mirrors the cancel-by-clientOid response.
type cancelClientOidWire struct {
	CancelledOrderID string `json:"cancelledOrderId"`
	ClientOid        string `json:"clientOid"`
}

// orderPageWire mirrors KuCoin's paginated list envelope for orders.
type orderPageWire struct {
	CurrentPage int             `json:"currentPage"`
	PageSize    int             `json:"pageSize"`
	TotalNum    int             `json:"totalNum"`
	TotalPage   int             `json:"totalPage"`
	Items       []orderInfoWire `json:"items"`
}

// orderInfoWire mirrors a spot order object. Money/price/size fields are
// strings; decimal decodes them. Spot REST orders carry no "status" string —
// isActive / cancelExist are authoritative.
type orderInfoWire struct {
	ID          string          `json:"id"`
	ClientOid   string          `json:"clientOid"`
	Symbol      string          `json:"symbol"`
	Type        string          `json:"type"`
	Side        string          `json:"side"`
	Price       decimal.Decimal `json:"price"`
	Size        decimal.Decimal `json:"size"`
	Funds       decimal.Decimal `json:"funds"`
	DealFunds   decimal.Decimal `json:"dealFunds"`
	DealSize    decimal.Decimal `json:"dealSize"`
	Fee         decimal.Decimal `json:"fee"`
	FeeCurrency string          `json:"feeCurrency"`
	TimeInForce string          `json:"timeInForce"`
	TradeType   string          `json:"tradeType"`
	PostOnly    bool            `json:"postOnly"`
	Hidden      bool            `json:"hidden"`
	Iceberg     bool            `json:"iceberg"`
	VisibleSize decimal.Decimal `json:"visibleSize"`
	CancelAfter int64           `json:"cancelAfter"`
	STP         string          `json:"stp"`
	IsActive    bool            `json:"isActive"`
	CancelExist bool            `json:"cancelExist"`
	Remark      string          `json:"remark"`
	CreatedAt   int64           `json:"createdAt"`
}

func (w orderInfoWire) toOrderInfo() spottypes.OrderInfo {
	return spottypes.OrderInfo{
		OrderID:       w.ID,
		ClientOrderID: w.ClientOid,
		Symbol:        w.Symbol,
		Type:          spottypes.OrderType(w.Type),
		Side:          spottypes.SideType(w.Side),
		Price:         w.Price,
		Size:          w.Size,
		Funds:         w.Funds,
		DealSize:      w.DealSize,
		DealFunds:     w.DealFunds,
		TimeInForce:   spottypes.TimeInForceType(w.TimeInForce),
		TradeType:     spottypes.TradeType(w.TradeType),
		PostOnly:      w.PostOnly,
		Hidden:        w.Hidden,
		Iceberg:       w.Iceberg,
		VisibleSize:   w.VisibleSize,
		CancelAfter:   w.CancelAfter,
		STP:           spottypes.SelfTradePrevention(w.STP),
		Fee:           w.Fee,
		FeeCurrency:   w.FeeCurrency,
		IsActive:      w.IsActive,
		CancelExist:   w.CancelExist,
		Remark:        w.Remark,
		CreatedAtMs:   w.CreatedAt,
	}
}

func ordersFromWire(items []orderInfoWire) []spottypes.OrderInfo {
	var out []spottypes.OrderInfo = make([]spottypes.OrderInfo, len(items))
	var i int
	for i = 0; i < len(items); i++ {
		out[i] = items[i].toOrderInfo()
	}
	return out
}

// fillPageWire mirrors KuCoin's paginated list envelope for fills.
type fillPageWire struct {
	CurrentPage int        `json:"currentPage"`
	PageSize    int        `json:"pageSize"`
	TotalNum    int        `json:"totalNum"`
	TotalPage   int        `json:"totalPage"`
	Items       []fillWire `json:"items"`
}

// fillWire mirrors one spot fill object. createdAt is ms.
type fillWire struct {
	TradeID     string          `json:"tradeId"`
	OrderID     string          `json:"orderId"`
	Symbol      string          `json:"symbol"`
	Side        string          `json:"side"`
	Liquidity   string          `json:"liquidity"`
	ForceTaker  bool            `json:"forceTaker"`
	Price       decimal.Decimal `json:"price"`
	Size        decimal.Decimal `json:"size"`
	Funds       decimal.Decimal `json:"funds"`
	Fee         decimal.Decimal `json:"fee"`
	FeeRate     decimal.Decimal `json:"feeRate"`
	FeeCurrency string          `json:"feeCurrency"`
	Type        string          `json:"type"`
	TradeType   string          `json:"tradeType"`
	CreatedAt   int64           `json:"createdAt"`
}

func (w fillWire) toFill() spottypes.Fill {
	return spottypes.Fill{
		TradeID:     w.TradeID,
		OrderID:     w.OrderID,
		Symbol:      w.Symbol,
		Side:        spottypes.SideType(w.Side),
		Price:       w.Price,
		Size:        w.Size,
		Funds:       w.Funds,
		Liquidity:   w.Liquidity,
		OrderType:   spottypes.OrderType(w.Type),
		TradeType:   spottypes.TradeType(w.TradeType),
		ForceTaker:  w.ForceTaker,
		Fee:         w.Fee,
		FeeRate:     w.FeeRate,
		FeeCurrency: w.FeeCurrency,
		CreatedAtMs: w.CreatedAt,
	}
}

func fillsFromWire(rows []fillWire) []spottypes.Fill {
	var out []spottypes.Fill = make([]spottypes.Fill, len(rows))
	var i int
	for i = 0; i < len(rows); i++ {
		out[i] = rows[i].toFill()
	}
	return out
}
