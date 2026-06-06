/*
FILE: margin/trading.go

DESCRIPTION:
Signed HF-margin trading sub-client for the KuCoin Margin profile. Covers the
high-frequency margin order lifecycle on api.kucoin.com:

  - POST   /api/v3/hf/margin/order                          place (limit/market)
  - POST   /api/v3/hf/margin/order/test                     place (validate-only)
  - DELETE /api/v3/hf/margin/orders/{orderId}?symbol=        cancel by order id
  - DELETE /api/v3/hf/margin/orders/client-order/{coid}?symbol=  cancel by clientOid
  - DELETE /api/v3/hf/margin/orders?symbol=&tradeType=        cancel all by symbol
  - GET    /api/v3/hf/margin/orders/active?symbol=&tradeType= open orders
  - GET    /api/v3/hf/margin/order/active/symbols?tradeType=  symbols w/ open orders
  - GET    /api/v3/hf/margin/orders/done?symbol=&tradeType=…  closed orders
  - GET    /api/v3/hf/margin/orders/{orderId}?symbol=         order detail by id
  - GET    /api/v3/hf/margin/orders/client-order/{coid}?symbol= detail by clientOid
  - GET    /api/v3/hf/margin/fills?symbol=&tradeType=…        fills

HF SPECIFICS: most order operations require BOTH symbol and tradeType
("MARGIN_TRADE" cross / "MARGIN_ISOLATED_TRADE" isolated). The trade type is
derived from the request / client default; the place body instead carries an
isIsolated boolean.

SIZING: order Size is in BASE currency (decimal); market orders may instead
pass Funds (quote currency). There is no contract multiplier.
*/

package margin

import (
	"context"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// TradingClient — signed HF-margin trading sub-client.
type TradingClient struct {
	c *Client
}

// newTradingClient wires the sub-client to its parent.
func newTradingClient(c *Client) *TradingClient {
	return &TradingClient{c: c}
}

// OrderAck — acknowledgement returned by PlaceOrder. BorrowSize / LoanApplyID
// are populated when KuCoin auto-borrowed to fund the order.
type OrderAck struct {
	OrderID       string
	ClientOrderID string
	BorrowSize    decimal.Decimal
	LoanApplyID   string
}

// GetClosedOrdersParams — filter / pagination for GetClosedOrders. HF closed
// orders paginate by lastId + limit (NOT currentPage/pageSize).
type GetClosedOrdersParams struct {
	Symbol    string
	TradeType margintypes.TradeType
	Side      margintypes.SideType
	Type      margintypes.OrderType
	StartAtMs int64
	EndAtMs   int64
	// LastID — id of the last row of the previous page (0 for the first).
	LastID int64
	// Limit — page size (KuCoin default/cap applies when 0).
	Limit int
}

// GetFillsParams — filter / pagination for GetFills.
type GetFillsParams struct {
	OrderID   string
	Symbol    string
	TradeType margintypes.TradeType
	Side      margintypes.SideType
	Type      margintypes.OrderType
	StartAtMs int64
	EndAtMs   int64
	LastID    int64
	Limit     int
}

// queryMeta is the rate-limit metadata for read-only trading queries.
var queryMeta = rest.RequestMeta{Category: "query"}

// resolveTradeType returns the explicit trade type or the client default.
func (t *TradingClient) resolveTradeType(tt margintypes.TradeType) margintypes.TradeType {
	if tt != "" {
		return tt
	}
	return t.c.defaultTradeType
}

// ---------------------------------------------------------------------
// Place.
// ---------------------------------------------------------------------

// PlaceOrder places a single HF-margin order (limit / market). It fills the
// clientOid default, validates the request and returns the assigned order id
// plus any auto-borrow detail.
func (t *TradingClient) PlaceOrder(ctx context.Context, req margintypes.CreateOrderRequest) (*OrderAck, error) {
	return t.place(ctx, req, "/api/v3/hf/margin/order")
}

// PlaceOrderTest validates an order against the matching engine WITHOUT
// submitting it (POST /api/v3/hf/margin/order/test). Useful to pre-flight
// sizing/precision. Returns a zero-value ack on success.
func (t *TradingClient) PlaceOrderTest(ctx context.Context, req margintypes.CreateOrderRequest) (*OrderAck, error) {
	return t.place(ctx, req, "/api/v3/hf/margin/order/test")
}

func (t *TradingClient) place(ctx context.Context, req margintypes.CreateOrderRequest, path string) (*OrderAck, error) {
	var body placeOrderBody
	var err error
	body, err = t.c.buildOrderBody(req)
	if err != nil {
		return nil, err
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Symbols: []string{req.Symbol}, Category: "place"}
	var ack ackWire
	if err = t.c.doPOST(ctx, path, body, meta, &ack); err != nil {
		return nil, err
	}
	return &OrderAck{
		OrderID:       ack.OrderID,
		ClientOrderID: body.ClientOid,
		BorrowSize:    ack.BorrowSize,
		LoanApplyID:   ack.LoanApplyID,
	}, nil
}

// ---------------------------------------------------------------------
// Cancel.
// ---------------------------------------------------------------------

// CancelOrder cancels a single order by KuCoin order id. HF margin requires
// the symbol. Returns the cancelled order id reported by KuCoin.
func (t *TradingClient) CancelOrder(ctx context.Context, symbol, orderID string) (string, error) {
	if symbol == "" {
		return "", errInvalidRequest("CancelOrder", "symbol is required")
	}
	if orderID == "" {
		return "", errInvalidRequest("CancelOrder", "orderID is required")
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Symbols: []string{symbol}, Category: "cancel"}
	var res cancelOrderIDWire
	if err := t.c.doDELETE(ctx, "/api/v3/hf/margin/orders/"+orderID, map[string]string{"symbol": symbol}, meta, &res); err != nil {
		return "", err
	}
	return res.OrderID, nil
}

// CancelOrderByClientOid cancels a single order by clientOid. HF margin
// requires the symbol. Returns the echoed clientOid.
func (t *TradingClient) CancelOrderByClientOid(ctx context.Context, symbol, clientOid string) (string, error) {
	if symbol == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "symbol is required")
	}
	if clientOid == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "clientOid is required")
	}
	var meta rest.RequestMeta = rest.RequestMeta{OrderCount: 1, Symbols: []string{symbol}, Category: "cancel"}
	var res cancelClientOidWire
	if err := t.c.doDELETE(ctx, "/api/v3/hf/margin/orders/client-order/"+clientOid, map[string]string{"symbol": symbol}, meta, &res); err != nil {
		return "", err
	}
	return res.ClientOid, nil
}

// CancelAllOrders cancels every open order for a symbol on the given trade
// type (cross/isolated; empty uses the client default). KuCoin returns a
// status string ("success").
func (t *TradingClient) CancelAllOrders(ctx context.Context, symbol string, tradeType margintypes.TradeType) (string, error) {
	if symbol == "" {
		return "", errInvalidRequest("CancelAllOrders", "symbol is required")
	}
	var query map[string]string = map[string]string{
		"symbol":    symbol,
		"tradeType": string(t.resolveTradeType(tradeType)),
	}
	var meta rest.RequestMeta = rest.RequestMeta{Symbols: []string{symbol}, Category: "cancel"}
	var res string
	if err := t.c.doDELETE(ctx, "/api/v3/hf/margin/orders", query, meta, &res); err != nil {
		return "", err
	}
	return res, nil
}

// ---------------------------------------------------------------------
// Queries.
// ---------------------------------------------------------------------

// GetOrder returns a single order by KuCoin order id. HF margin requires the
// symbol.
func (t *TradingClient) GetOrder(ctx context.Context, symbol, orderID string) (*margintypes.OrderInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOrder", "symbol is required")
	}
	if orderID == "" {
		return nil, errInvalidRequest("GetOrder", "orderID is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/orders/"+orderID, map[string]string{"symbol": symbol}, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info margintypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOrderByClientOid returns a single order by clientOid. HF margin requires
// the symbol.
func (t *TradingClient) GetOrderByClientOid(ctx context.Context, symbol, clientOid string) (*margintypes.OrderInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOrderByClientOid", "symbol is required")
	}
	if clientOid == "" {
		return nil, errInvalidRequest("GetOrderByClientOid", "clientOid is required")
	}
	var wire orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/orders/client-order/"+clientOid, map[string]string{"symbol": symbol}, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info margintypes.OrderInfo = wire.toOrderInfo()
	return &info, nil
}

// GetOpenOrders returns the active orders for a symbol on the given trade type
// (empty uses the client default).
func (t *TradingClient) GetOpenOrders(ctx context.Context, symbol string, tradeType margintypes.TradeType) ([]margintypes.OrderInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetOpenOrders", "symbol is required")
	}
	var query map[string]string = map[string]string{
		"symbol":    symbol,
		"tradeType": string(t.resolveTradeType(tradeType)),
	}
	var rows []orderInfoWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/orders/active", query, queryMeta, &rows); err != nil {
		return nil, err
	}
	return ordersFromWire(rows), nil
}

// GetActiveSymbols returns the symbols that currently have open orders on the
// given trade type (empty uses the client default).
func (t *TradingClient) GetActiveSymbols(ctx context.Context, tradeType margintypes.TradeType) ([]string, error) {
	var query map[string]string = map[string]string{"tradeType": string(t.resolveTradeType(tradeType))}
	var res activeSymbolsWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/order/active/symbols", query, queryMeta, &res); err != nil {
		return nil, err
	}
	return res.Symbols, nil
}

// GetClosedOrders returns a page of closed (done) orders matching the filter.
func (t *TradingClient) GetClosedOrders(ctx context.Context, p GetClosedOrdersParams) ([]margintypes.OrderInfo, error) {
	if p.Symbol == "" {
		return nil, errInvalidRequest("GetClosedOrders", "symbol is required")
	}
	var query map[string]string = map[string]string{
		"symbol":    p.Symbol,
		"tradeType": string(t.resolveTradeType(p.TradeType)),
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
	if p.LastID > 0 {
		query["lastId"] = itoa(p.LastID)
	}
	if p.Limit > 0 {
		query["limit"] = itoa(int64(p.Limit))
	}
	var page orderListWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/orders/done", query, queryMeta, &page); err != nil {
		return nil, err
	}
	return ordersFromWire(page.Items), nil
}

// GetFills returns a page of fills matching the filter.
func (t *TradingClient) GetFills(ctx context.Context, p GetFillsParams) ([]margintypes.Fill, error) {
	if p.Symbol == "" {
		return nil, errInvalidRequest("GetFills", "symbol is required")
	}
	var query map[string]string = map[string]string{
		"symbol":    p.Symbol,
		"tradeType": string(t.resolveTradeType(p.TradeType)),
	}
	if p.OrderID != "" {
		query["orderId"] = p.OrderID
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
	if p.LastID > 0 {
		query["lastId"] = itoa(p.LastID)
	}
	if p.Limit > 0 {
		query["limit"] = itoa(int64(p.Limit))
	}
	var page fillListWire
	if err := t.c.doGET(ctx, true, "/api/v3/hf/margin/fills", query, queryMeta, &page); err != nil {
		return nil, err
	}
	return fillsFromWire(page.Items), nil
}

// ---------------------------------------------------------------------
// Request body assembly.
// ---------------------------------------------------------------------

// placeOrderBody is the KuCoin /api/v3/hf/margin/order request body.
// Zero-value booleans and empty strings are omitted so KuCoin applies its own
// defaults. Cross vs isolated is selected by isIsolated (no tradeType field).
type placeOrderBody struct {
	ClientOid   string `json:"clientOid"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Type        string `json:"type,omitempty"`
	IsIsolated  bool   `json:"isIsolated,omitempty"`
	AutoBorrow  bool   `json:"autoBorrow,omitempty"`
	AutoRepay   bool   `json:"autoRepay,omitempty"`
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

// buildOrderBody validates the request, applies client defaults and maps it
// onto the KuCoin HF-margin wire body.
func (c *Client) buildOrderBody(req margintypes.CreateOrderRequest) (placeOrderBody, error) {
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
	case margintypes.OrderLimit:
		if req.Price.LessThanOrEqual(decimal.Zero) {
			return b, errInvalidRequest("PlaceOrder", "price must be > 0 for limit orders")
		}
		if req.Size.LessThanOrEqual(decimal.Zero) {
			return b, errInvalidRequest("PlaceOrder", "size (base) must be > 0 for limit orders")
		}
	case margintypes.OrderMarket:
		var hasSize bool = req.Size.GreaterThan(decimal.Zero)
		var hasFunds bool = req.Funds.GreaterThan(decimal.Zero)
		if hasSize == hasFunds { // both set or both unset
			return b, errInvalidRequest("PlaceOrder", "market order needs exactly one of size (base) or funds (quote)")
		}
	default:
		return b, errInvalidRequest("PlaceOrder", "unsupported order type")
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
		IsIsolated:  c.resolveIsolated(req),
		AutoBorrow:  req.AutoBorrow,
		AutoRepay:   req.AutoRepay,
		TimeInForce: string(req.TimeInForce),
		PostOnly:    req.PostOnly,
		Hidden:      req.Hidden,
		Iceberg:     req.Iceberg,
		CancelAfter: req.CancelAfter,
		STP:         string(req.STP),
		Remark:      req.Remark,
	}
	if req.Type == margintypes.OrderLimit {
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

// resolveIsolated decides cross vs isolated for a place request: an explicit
// IsIsolated flag wins; otherwise the client default trade type applies.
func (c *Client) resolveIsolated(req margintypes.CreateOrderRequest) bool {
	if req.IsIsolated {
		return true
	}
	return c.defaultTradeType.IsIsolated()
}

// ---------------------------------------------------------------------
// Response wire structs + converters.
// ---------------------------------------------------------------------

// ackWire mirrors the place-order response data.
type ackWire struct {
	OrderID     string          `json:"orderId"`
	ClientOid   string          `json:"clientOid"`
	BorrowSize  decimal.Decimal `json:"borrowSize"`
	LoanApplyID string          `json:"loanApplyId"`
}

// cancelOrderIDWire mirrors the cancel-by-orderId response.
type cancelOrderIDWire struct {
	OrderID string `json:"orderId"`
}

// cancelClientOidWire mirrors the cancel-by-clientOid response.
type cancelClientOidWire struct {
	ClientOid string `json:"clientOid"`
}

// activeSymbolsWire mirrors the symbols-with-open-order response.
type activeSymbolsWire struct {
	Symbols []string `json:"symbols"`
}

// orderListWire mirrors the HF closed-orders list (rows under items).
type orderListWire struct {
	LastID int64           `json:"lastId"`
	Items  []orderInfoWire `json:"items"`
}

// fillListWire mirrors the HF fills list (rows under items).
type fillListWire struct {
	LastID int64      `json:"lastId"`
	Items  []fillWire `json:"items"`
}

// orderInfoWire mirrors an HF-margin order object. Money/price/size fields are
// strings; decimal decodes them. HF reports liveness via `active`.
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
	Active      bool            `json:"active"`
	CancelExist bool            `json:"cancelExist"`
	Remark      string          `json:"remark"`
	CreatedAt   int64           `json:"createdAt"`
	LastUpdated int64           `json:"lastUpdatedAt"`
}

func (w orderInfoWire) toOrderInfo() margintypes.OrderInfo {
	return margintypes.OrderInfo{
		OrderID:       w.ID,
		ClientOrderID: w.ClientOid,
		Symbol:        w.Symbol,
		Type:          margintypes.OrderType(w.Type),
		Side:          margintypes.SideType(w.Side),
		Price:         w.Price,
		Size:          w.Size,
		Funds:         w.Funds,
		DealSize:      w.DealSize,
		DealFunds:     w.DealFunds,
		TimeInForce:   margintypes.TimeInForceType(w.TimeInForce),
		TradeType:     margintypes.TradeType(w.TradeType),
		PostOnly:      w.PostOnly,
		Hidden:        w.Hidden,
		Iceberg:       w.Iceberg,
		VisibleSize:   w.VisibleSize,
		CancelAfter:   w.CancelAfter,
		STP:           margintypes.SelfTradePrevention(w.STP),
		Fee:           w.Fee,
		FeeCurrency:   w.FeeCurrency,
		IsActive:      w.Active,
		CancelExist:   w.CancelExist,
		Remark:        w.Remark,
		CreatedAtMs:   w.CreatedAt,
		UpdatedAtMs:   w.LastUpdated,
	}
}

func ordersFromWire(items []orderInfoWire) []margintypes.OrderInfo {
	var out []margintypes.OrderInfo = make([]margintypes.OrderInfo, len(items))
	var i int
	for i = 0; i < len(items); i++ {
		out[i] = items[i].toOrderInfo()
	}
	return out
}

// fillWire mirrors one HF-margin fill object. createdAt is ms.
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

func (w fillWire) toFill() margintypes.Fill {
	return margintypes.Fill{
		TradeID:     w.TradeID,
		OrderID:     w.OrderID,
		Symbol:      w.Symbol,
		Side:        margintypes.SideType(w.Side),
		Price:       w.Price,
		Size:        w.Size,
		Funds:       w.Funds,
		Liquidity:   w.Liquidity,
		OrderType:   margintypes.OrderType(w.Type),
		TradeType:   margintypes.TradeType(w.TradeType),
		ForceTaker:  w.ForceTaker,
		Fee:         w.Fee,
		FeeRate:     w.FeeRate,
		FeeCurrency: w.FeeCurrency,
		CreatedAtMs: w.CreatedAt,
	}
}

func fillsFromWire(rows []fillWire) []margintypes.Fill {
	var out []margintypes.Fill = make([]margintypes.Fill, len(rows))
	var i int
	for i = 0; i < len(rows); i++ {
		out[i] = rows[i].toFill()
	}
	return out
}
