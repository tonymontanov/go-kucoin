/*
FILE: futures/trading.go

DESCRIPTION:
Signed trading sub-client for the KuCoin Futures profile. Covers the full
v1.0 order lifecycle on api-futures.kucoin.com:

  - POST   /api/v1/orders                          place (limit/market/stop)
  - POST   /api/v1/orders/multi                     batch place
  - DELETE /api/v1/orders/{orderId}                 cancel by order id
  - DELETE /api/v1/orders/client-order/{clientOid}  cancel by client id
  - DELETE /api/v1/orders?symbol=                     cancel all (limit)
  - DELETE /api/v1/stopOrders?symbol=                 cancel all stop
  - GET    /api/v1/orders/{orderId}                  order detail by id
  - GET    /api/v1/orders/byClientOid?clientOid=     order detail by client id
  - GET    /api/v1/orders?status=&symbol=...          order list (paginated)
  - GET    /api/v1/stopOrders?symbol=                 untriggered stop orders
  - GET    /api/v1/fills                              fills (paginated)
  - GET    /api/v1/recentFills?symbol=                recent fills

SIZING: order Size is an INTEGER NUMBER OF CONTRACTS. Convert base quantity
via SymbolInfo.Multiplier before placing (see futures/types/contract.go).

LEVERAGE: KuCoin classic futures takes leverage PER ORDER (there is no
separate set-leverage REST call in the classic API). PlaceOrder uses the
request's Leverage or the client default; if both are empty it errors.
*/

package futures

import (
	"context"

	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
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
	Side futurestypes.SideType
	// Type — "limit"/"market"; empty for all.
	Type futurestypes.OrderType
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
	Side        futurestypes.SideType
	Type        futurestypes.OrderType
	StartAtMs   int64
	EndAtMs     int64
	CurrentPage int
	PageSize    int
}

// ---------------------------------------------------------------------
// Place.
// ---------------------------------------------------------------------

// PlaceOrder places a single order (limit / market / stop). It fills the
// clientOid, leverage and margin mode defaults, validates the request and
// returns the assigned order id together with the clientOid actually used.
func (t *TradingClient) PlaceOrder(ctx context.Context, req futurestypes.CreateOrderRequest) (*OrderAck, error) {
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
	return &OrderAck{OrderID: ack.OrderID, ClientOrderID: ack.ClientOid}, nil
}

// PlaceBatchOrders places multiple orders in one request
// (POST /api/v1/orders/multi). The returned slice has one row per submitted
// order; inspect Success / Code per row to reconcile a partial batch.
func (t *TradingClient) PlaceBatchOrders(ctx context.Context, reqs []futurestypes.CreateOrderRequest) ([]futurestypes.BatchOrderResult, error) {
	if len(reqs) == 0 {
		return nil, errInvalidRequest("PlaceBatchOrders", "at least one order is required")
	}
	var bodies []placeOrderBody = make([]placeOrderBody, len(reqs))
	var symbols []string = make([]string, 0, len(reqs))
	var i int
	for i = 0; i < len(reqs); i++ {
		var b placeOrderBody
		var err error
		b, err = t.c.buildOrderBody(reqs[i])
		if err != nil {
			return nil, err
		}
		bodies[i] = b
		symbols = append(symbols, reqs[i].Symbol)
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: len(reqs), Symbols: symbols, Category: "place"}
	var rows []batchRowWire
	if err := t.c.doPOST(ctx, "/api/v1/orders/multi", bodies, meta, &rows); err != nil {
		return nil, err
	}
	var out []futurestypes.BatchOrderResult = make([]futurestypes.BatchOrderResult, len(rows))
	for i = 0; i < len(rows); i++ {
		out[i] = futurestypes.BatchOrderResult{
			OrderID:       rows[i].OrderID,
			ClientOrderID: rows[i].ClientOid,
			Symbol:        rows[i].Symbol,
			Success:       rows[i].Code == "" || rows[i].Code == codeOK,
			Code:          rows[i].Code,
			Msg:           rows[i].Msg,
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

// CancelOrderByClientOid cancels a single order by clientOid. KuCoin
// requires the symbol for this path.
func (t *TradingClient) CancelOrderByClientOid(ctx context.Context, symbol, clientOid string) (string, error) {
	if clientOid == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "clientOid is required")
	}
	if symbol == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "symbol is required")
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Symbols: []string{symbol}, Category: "cancel"}
	var res cancelClientOidWire
	if err := t.c.doDELETE(ctx, "/api/v1/orders/client-order/"+clientOid, map[string]string{"symbol": symbol}, meta, &res); err != nil {
		return "", err
	}
	return res.ClientOid, nil
}

// CancelAllOrders cancels all (limit) orders, optionally scoped to a symbol.
func (t *TradingClient) CancelAllOrders(ctx context.Context, symbol string) ([]string, error) {
	var query map[string]string
	var symbols []string
	if symbol != "" {
		query = map[string]string{"symbol": symbol}
		symbols = []string{symbol}
	}
	var meta rest.RequestMeta = rest.RequestMeta{Symbols: symbols, Category: "cancel"}
	var res cancelIDsWire
	if err := t.c.doDELETE(ctx, "/api/v1/orders", query, meta, &res); err != nil {
		return nil, err
	}
	return res.CancelledOrderIDs, nil
}

// CancelAllStopOrders cancels all untriggered stop orders, optionally scoped
// to a symbol.
func (t *TradingClient) CancelAllStopOrders(ctx context.Context, symbol string) ([]string, error) {
	var query map[string]string
	var symbols []string
	if symbol != "" {
		query = map[string]string{"symbol": symbol}
		symbols = []string{symbol}
	}
	var meta rest.RequestMeta = rest.RequestMeta{Symbols: symbols, Category: "cancel"}
	var res cancelIDsWire
	if err := t.c.doDELETE(ctx, "/api/v1/stopOrders", query, meta, &res); err != nil {
		return nil, err
	}
	return res.CancelledOrderIDs, nil
}

// ---------------------------------------------------------------------
// Queries.
// ---------------------------------------------------------------------

// GetOrder returns a single order by KuCoin order id.
func (t *TradingClient) GetOrder(ctx context.Context, orderID string) (*futurestypes.OrderInfo, error) {
	if orderID == "" {
		return nil, errInvalidRequest("GetOrder", "orderID is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v1/orders/"+orderID, nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info futurestypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOrderByClientOid returns a single order by clientOid.
func (t *TradingClient) GetOrderByClientOid(ctx context.Context, clientOid string) (*futurestypes.OrderInfo, error) {
	if clientOid == "" {
		return nil, errInvalidRequest("GetOrderByClientOid", "clientOid is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v1/orders/byClientOid", map[string]string{"clientOid": clientOid}, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info futurestypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOrders returns a page of orders matching the filter.
func (t *TradingClient) GetOrders(ctx context.Context, p GetOrdersParams) ([]futurestypes.OrderInfo, error) {
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
func (t *TradingClient) GetOpenOrders(ctx context.Context, symbol string) ([]futurestypes.OrderInfo, error) {
	return t.GetOrders(ctx, GetOrdersParams{Status: "active", Symbol: symbol})
}

// GetStopOrders returns untriggered stop orders, optionally scoped to a
// symbol.
func (t *TradingClient) GetStopOrders(ctx context.Context, symbol string) ([]futurestypes.OrderInfo, error) {
	var query map[string]string
	if symbol != "" {
		query = map[string]string{"symbol": symbol}
	}
	var page orderPageWire
	if err := t.c.doGET(ctx, true, "/api/v1/stopOrders", query, queryMeta, &page); err != nil {
		return nil, err
	}
	return ordersFromWire(page.Items), nil
}

// GetFills returns a page of fills matching the filter.
func (t *TradingClient) GetFills(ctx context.Context, p GetFillsParams) ([]futurestypes.Fill, error) {
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

// GetRecentFills returns the most recent fills (last 24h), optionally scoped
// to a symbol.
func (t *TradingClient) GetRecentFills(ctx context.Context, symbol string) ([]futurestypes.Fill, error) {
	var query map[string]string
	if symbol != "" {
		query = map[string]string{"symbol": symbol}
	}
	var rows []fillWire
	if err := t.c.doGET(ctx, true, "/api/v1/recentFills", query, queryMeta, &rows); err != nil {
		return nil, err
	}
	return fillsFromWire(rows), nil
}

// queryMeta is the rate-limit metadata for read-only trading queries.
var queryMeta = rest.RequestMeta{Category: "query"}

// codeOK mirrors the KuCoin success envelope code for per-row batch checks.
const codeOK = "200000"

// ---------------------------------------------------------------------
// Request body assembly.
// ---------------------------------------------------------------------

// placeOrderBody is the KuCoin /api/v1/orders request body. Zero-value
// booleans and empty strings are omitted so KuCoin applies its own defaults.
type placeOrderBody struct {
	ClientOid     string `json:"clientOid"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type,omitempty"`
	Leverage      string `json:"leverage,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Price         string `json:"price,omitempty"`
	TimeInForce   string `json:"timeInForce,omitempty"`
	PostOnly      bool   `json:"postOnly,omitempty"`
	Hidden        bool   `json:"hidden,omitempty"`
	Iceberg       bool   `json:"iceberg,omitempty"`
	VisibleSize   string `json:"visibleSize,omitempty"`
	ReduceOnly    bool   `json:"reduceOnly,omitempty"`
	CloseOrder    bool   `json:"closeOrder,omitempty"`
	ForceHold     bool   `json:"forceHold,omitempty"`
	MarginMode    string `json:"marginMode,omitempty"`
	Remark        string `json:"remark,omitempty"`
	Stop          string `json:"stop,omitempty"`
	StopPriceType string `json:"stopPriceType,omitempty"`
	StopPrice     string `json:"stopPrice,omitempty"`
}

// buildOrderBody validates the request, applies client defaults and maps it
// onto the KuCoin wire body.
func (c *Client) buildOrderBody(req futurestypes.CreateOrderRequest) (placeOrderBody, error) {
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
	if req.Size <= 0 {
		return b, errInvalidRequest("PlaceOrder", "size (contracts) must be > 0")
	}
	if req.Type == futurestypes.OrderLimit && req.Price.IsZero() {
		return b, errInvalidRequest("PlaceOrder", "price is required for limit orders")
	}

	var leverage string = req.Leverage
	if leverage == "" {
		leverage = c.defaultLeverage
	}
	if leverage == "" {
		return b, errInvalidRequest("PlaceOrder", "leverage is required (set per order or via the client default)")
	}

	var marginMode futurestypes.MarginMode = req.MarginMode
	if marginMode == "" {
		marginMode = c.defaultMarginMode
	}

	var clientOid string = req.ClientOrderID
	if clientOid == "" {
		clientOid = generateClientOid()
	}

	b = placeOrderBody{
		ClientOid:     clientOid,
		Symbol:        req.Symbol,
		Side:          string(req.Side),
		Type:          string(req.Type),
		Leverage:      leverage,
		Size:          req.Size,
		TimeInForce:   string(req.TimeInForce),
		PostOnly:      req.PostOnly,
		Hidden:        req.Hidden,
		Iceberg:       req.Iceberg,
		ReduceOnly:    req.ReduceOnly,
		CloseOrder:    req.CloseOrder,
		ForceHold:     req.ForceHold,
		MarginMode:    string(marginMode),
		Remark:        req.Remark,
		Stop:          string(req.Stop),
		StopPriceType: string(req.StopPriceType),
	}
	if req.Type == futurestypes.OrderLimit {
		b.Price = req.Price.String()
	}
	if !req.VisibleSize.IsZero() {
		b.VisibleSize = req.VisibleSize.String()
	}
	if !req.StopPrice.IsZero() {
		b.StopPrice = req.StopPrice.String()
	}
	return b, nil
}

// ---------------------------------------------------------------------
// Response wire structs + converters.
// ---------------------------------------------------------------------

// ackWire mirrors the place-order response data.
type ackWire struct {
	OrderID   string `json:"orderId"`
	ClientOid string `json:"clientOid"`
}

// batchRowWire mirrors one row of the /api/v1/orders/multi response.
type batchRowWire struct {
	OrderID   string `json:"orderId"`
	ClientOid string `json:"clientOid"`
	Symbol    string `json:"symbol"`
	Code      string `json:"code"`
	Msg       string `json:"msg"`
}

// cancelIDsWire mirrors the cancel responses that return cancelledOrderIds.
type cancelIDsWire struct {
	CancelledOrderIDs []string `json:"cancelledOrderIds"`
}

// cancelClientOidWire mirrors the cancel-by-clientOid response.
type cancelClientOidWire struct {
	ClientOid string `json:"clientOid"`
}

// orderPageWire mirrors KuCoin's paginated list envelope for orders.
type orderPageWire struct {
	CurrentPage int             `json:"currentPage"`
	PageSize    int             `json:"pageSize"`
	TotalNum    int             `json:"totalNum"`
	TotalPage   int             `json:"totalPage"`
	Items       []orderInfoWire `json:"items"`
}

// orderInfoWire mirrors an order object. KuCoin ships money/price fields as
// strings and size/dealSize as numbers; decimal decodes both.
type orderInfoWire struct {
	ID            string          `json:"id"`
	ClientOid     string          `json:"clientOid"`
	Symbol        string          `json:"symbol"`
	Type          string          `json:"type"`
	Side          string          `json:"side"`
	Price         decimal.Decimal `json:"price"`
	Size          decimal.Decimal `json:"size"`
	Value         decimal.Decimal `json:"value"`
	DealValue     decimal.Decimal `json:"dealValue"`
	DealSize      decimal.Decimal `json:"dealSize"`
	Leverage      decimal.Decimal `json:"leverage"`
	TimeInForce   string          `json:"timeInForce"`
	MarginMode    string          `json:"marginMode"`
	PostOnly      bool            `json:"postOnly"`
	Hidden        bool            `json:"hidden"`
	Iceberg       bool            `json:"iceberg"`
	VisibleSize   decimal.Decimal `json:"visibleSize"`
	ReduceOnly    bool            `json:"reduceOnly"`
	CloseOrder    bool            `json:"closeOrder"`
	ForceHold     bool            `json:"forceHold"`
	Stop          string          `json:"stop"`
	StopPriceType string          `json:"stopPriceType"`
	StopPrice     decimal.Decimal `json:"stopPrice"`
	StopTriggered bool            `json:"stopTriggered"`
	Status        string          `json:"status"`
	IsActive      bool            `json:"isActive"`
	CancelExist   bool            `json:"cancelExist"`
	Remark        string          `json:"remark"`
	Settle        string          `json:"settleCurrency"`
	CreatedAt     int64           `json:"createdAt"`
	UpdatedAt     int64           `json:"updatedAt"`
}

func (w orderInfoWire) toOrderInfo() futurestypes.OrderInfo {
	return futurestypes.OrderInfo{
		OrderID:        w.ID,
		ClientOrderID:  w.ClientOid,
		Symbol:         w.Symbol,
		Type:           futurestypes.OrderType(w.Type),
		Side:           futurestypes.SideType(w.Side),
		Price:          w.Price,
		Size:           w.Size,
		Value:          w.Value,
		FilledSize:     w.DealSize,
		FilledValue:    w.DealValue,
		Leverage:       w.Leverage,
		TimeInForce:    futurestypes.TimeInForceType(w.TimeInForce),
		MarginMode:     futurestypes.MarginMode(w.MarginMode),
		PostOnly:       w.PostOnly,
		Hidden:         w.Hidden,
		Iceberg:        w.Iceberg,
		VisibleSize:    w.VisibleSize,
		ReduceOnly:     w.ReduceOnly,
		CloseOrder:     w.CloseOrder,
		ForceHold:      w.ForceHold,
		Stop:           futurestypes.StopType(w.Stop),
		StopPriceType:  futurestypes.StopPriceType(w.StopPriceType),
		StopPrice:      w.StopPrice,
		StopTriggered:  w.StopTriggered,
		Status:         futurestypes.OrderStatus(w.Status),
		IsActive:       w.IsActive,
		CancelExist:    w.CancelExist,
		Remark:         w.Remark,
		SettleCurrency: w.Settle,
		CreatedAtMs:    w.CreatedAt,
		UpdatedAtMs:    w.UpdatedAt,
	}
}

func ordersFromWire(items []orderInfoWire) []futurestypes.OrderInfo {
	var out []futurestypes.OrderInfo = make([]futurestypes.OrderInfo, len(items))
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

// fillWire mirrors one fill object. tradeTime is ns; createdAt is ms.
type fillWire struct {
	TradeID     string          `json:"tradeId"`
	OrderID     string          `json:"orderId"`
	Symbol      string          `json:"symbol"`
	Side        string          `json:"side"`
	Liquidity   string          `json:"liquidity"`
	Price       decimal.Decimal `json:"price"`
	Size        decimal.Decimal `json:"size"`
	Value       decimal.Decimal `json:"value"`
	FeeRate     decimal.Decimal `json:"feeRate"`
	FixFee      decimal.Decimal `json:"fixFee"`
	FeeCurrency string          `json:"feeCurrency"`
	Fee         decimal.Decimal `json:"fee"`
	OrderType   string          `json:"orderType"`
	TradeType   string          `json:"tradeType"`
	Settle      string          `json:"settleCurrency"`
	CreatedAt   int64           `json:"createdAt"`
	TradeTime   int64           `json:"tradeTime"`
}

func (w fillWire) toFill() futurestypes.Fill {
	return futurestypes.Fill{
		TradeID:        w.TradeID,
		OrderID:        w.OrderID,
		Symbol:         w.Symbol,
		Side:           futurestypes.SideType(w.Side),
		Price:          w.Price,
		Size:           w.Size,
		Value:          w.Value,
		Liquidity:      futurestypes.Liquidity(w.Liquidity),
		OrderType:      futurestypes.OrderType(w.OrderType),
		TradeType:      futurestypes.TradeType(w.TradeType),
		Fee:            w.Fee,
		FeeRate:        w.FeeRate,
		FixFee:         w.FixFee,
		FeeCurrency:    w.FeeCurrency,
		SettleCurrency: w.Settle,
		TradeTimeMs:    nsToMs(w.TradeTime),
		CreatedAtMs:    w.CreatedAt,
	}
}

func fillsFromWire(rows []fillWire) []futurestypes.Fill {
	var out []futurestypes.Fill = make([]futurestypes.Fill, len(rows))
	var i int
	for i = 0; i < len(rows); i++ {
		out[i] = rows[i].toFill()
	}
	return out
}
