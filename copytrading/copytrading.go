/*
FILE: copytrading/copytrading.go

DESCRIPTION:
Lead-trader futures copy-trading: order placement (+TP/SL), cancellation, max
open size and isolated-margin / risk-limit management.

ENDPOINTS (FUTURES host):
  - POST   /api/v1/copy-trade/futures/orders                          PlaceOrder
  - POST   /api/v1/copy-trade/futures/orders/test                     PlaceOrderTest
  - POST   /api/v1/copy-trade/futures/st-orders                       PlaceTPSLOrder
  - DELETE /api/v1/copy-trade/futures/orders                          CancelOrder
  - DELETE /api/v1/copy-trade/futures/orders/client-order             CancelOrderByClientOid
  - GET    /api/v1/copy-trade/futures/get-max-open-size               GetMaxOpenSize
  - GET    /api/v1/copy-trade/futures/position/margin/max-withdraw-margin  GetMaxWithdrawMargin
  - POST   /api/v1/copy-trade/futures/position/margin/deposit-margin  AddIsolatedMargin
  - POST   /api/v1/copy-trade/futures/position/margin/withdraw-margin RemoveIsolatedMargin
  - POST   /api/v1/copy-trade/futures/position/risk-limit-level/change ModifyRiskLimitLevel
  - POST   /api/v1/copy-trade/futures/position/margin/auto-deposit-status SetAutoDepositStatus
*/

package copytrading

import (
	"context"

	"github.com/shopspring/decimal"

	cttypes "github.com/tonymontanov/go-kucoin/v2/copytrading/types"
)

// PlaceOrder places a copy-trade futures order.
func (c *Client) PlaceOrder(ctx context.Context, req cttypes.OrderRequest) (*cttypes.OrderResult, error) {
	return c.placeOrder(ctx, "/api/v1/copy-trade/futures/orders", req, nil)
}

// PlaceOrderTest validates a copy-trade order without matching it.
func (c *Client) PlaceOrderTest(ctx context.Context, req cttypes.OrderRequest) (*cttypes.OrderResult, error) {
	return c.placeOrder(ctx, "/api/v1/copy-trade/futures/orders/test", req, nil)
}

// PlaceTPSLOrder places a copy-trade order with take-profit / stop-loss.
func (c *Client) PlaceTPSLOrder(ctx context.Context, req cttypes.TPSLOrderRequest) (*cttypes.OrderResult, error) {
	if req.StopPriceType == "" {
		return nil, errInvalidRequest("PlaceTPSLOrder", "stopPriceType is required")
	}
	var extra map[string]any = map[string]any{"stopPriceType": req.StopPriceType}
	if req.TriggerStopUpPrice != "" {
		extra["triggerStopUpPrice"] = req.TriggerStopUpPrice
	}
	if req.TriggerStopDownPrice != "" {
		extra["triggerStopDownPrice"] = req.TriggerStopDownPrice
	}
	return c.placeOrder(ctx, "/api/v1/copy-trade/futures/st-orders", req.OrderRequest, extra)
}

// CancelOrder cancels a copy-trade order by orderId.
func (c *Client) CancelOrder(ctx context.Context, orderID string) (*cttypes.CancelResult, error) {
	if orderID == "" {
		return nil, errInvalidRequest("CancelOrder", "orderId is required")
	}
	var wire struct {
		CancelledOrderIDs []string `json:"cancelledOrderIds"`
	}
	if err := c.doDELETE(ctx, "/api/v1/copy-trade/futures/orders", map[string]string{"orderId": orderID}, &wire); err != nil {
		return nil, err
	}
	return &cttypes.CancelResult{CancelledOrderIDs: wire.CancelledOrderIDs}, nil
}

// CancelOrderByClientOid cancels a copy-trade order by clientOid (symbol
// required).
func (c *Client) CancelOrderByClientOid(ctx context.Context, clientOid, symbol string) (string, error) {
	if clientOid == "" || symbol == "" {
		return "", errInvalidRequest("CancelOrderByClientOid", "clientOid and symbol are required")
	}
	var wire struct {
		ClientOid string `json:"clientOid"`
	}
	if err := c.doDELETE(ctx, "/api/v1/copy-trade/futures/orders/client-order", map[string]string{"clientOid": clientOid, "symbol": symbol}, &wire); err != nil {
		return "", err
	}
	return wire.ClientOid, nil
}

// GetMaxOpenSize returns the maximum open size for a symbol at a price /
// leverage.
func (c *Client) GetMaxOpenSize(ctx context.Context, symbol, price, leverage string) (*cttypes.MaxOpenSize, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetMaxOpenSize", "symbol is required")
	}
	var query map[string]string = map[string]string{"symbol": symbol}
	if price != "" {
		query["price"] = price
	}
	if leverage != "" {
		query["leverage"] = leverage
	}
	var wire struct {
		Symbol          string          `json:"symbol"`
		MaxBuyOpenSize  decimal.Decimal `json:"maxBuyOpenSize"`
		MaxSellOpenSize decimal.Decimal `json:"maxSellOpenSize"`
	}
	if err := c.doGET(ctx, "/api/v1/copy-trade/futures/get-max-open-size", query, &wire); err != nil {
		return nil, err
	}
	return &cttypes.MaxOpenSize{Symbol: wire.Symbol, MaxBuyOpenSize: wire.MaxBuyOpenSize, MaxSellOpenSize: wire.MaxSellOpenSize}, nil
}

// GetMaxWithdrawMargin returns the maximum isolated margin withdrawable from a
// position.
func (c *Client) GetMaxWithdrawMargin(ctx context.Context, symbol, positionSide string) (decimal.Decimal, error) {
	if symbol == "" {
		return decimal.Decimal{}, errInvalidRequest("GetMaxWithdrawMargin", "symbol is required")
	}
	var query map[string]string = map[string]string{"symbol": symbol}
	if positionSide != "" {
		query["positionSide"] = positionSide
	}
	var out decimal.Decimal
	if err := c.doGET(ctx, "/api/v1/copy-trade/futures/position/margin/max-withdraw-margin", query, &out); err != nil {
		return decimal.Decimal{}, err
	}
	return out, nil
}

// AddIsolatedMargin manually adds isolated margin to a copy-trade position.
func (c *Client) AddIsolatedMargin(ctx context.Context, req cttypes.AddMarginRequest) (*cttypes.Position, error) {
	if req.Symbol == "" || req.Margin == "" || req.BizNo == "" {
		return nil, errInvalidRequest("AddIsolatedMargin", "symbol, margin and bizNo are required")
	}
	var body map[string]any = map[string]any{"symbol": req.Symbol, "margin": req.Margin, "bizNo": req.BizNo}
	if req.PositionSide != "" {
		body["positionSide"] = req.PositionSide
	}
	var wire positionWire
	if err := c.doPOST(ctx, "/api/v1/copy-trade/futures/position/margin/deposit-margin", body, &wire); err != nil {
		return nil, err
	}
	var p cttypes.Position = wire.toPosition()
	return &p, nil
}

// RemoveIsolatedMargin manually removes isolated margin from a copy-trade
// position; returns the withdrawn amount.
func (c *Client) RemoveIsolatedMargin(ctx context.Context, symbol, withdrawAmount, positionSide string) (decimal.Decimal, error) {
	if symbol == "" || withdrawAmount == "" {
		return decimal.Decimal{}, errInvalidRequest("RemoveIsolatedMargin", "symbol and withdrawAmount are required")
	}
	var body map[string]any = map[string]any{"symbol": symbol, "withdrawAmount": withdrawAmount}
	if positionSide != "" {
		body["positionSide"] = positionSide
	}
	var out decimal.Decimal
	if err := c.doPOST(ctx, "/api/v1/copy-trade/futures/position/margin/withdraw-margin", body, &out); err != nil {
		return decimal.Decimal{}, err
	}
	return out, nil
}

// ModifyRiskLimitLevel adjusts the isolated-margin risk-limit level (cancels
// open orders). Returns whether the request was accepted.
func (c *Client) ModifyRiskLimitLevel(ctx context.Context, symbol string, level int) (bool, error) {
	if symbol == "" {
		return false, errInvalidRequest("ModifyRiskLimitLevel", "symbol is required")
	}
	var out bool
	if err := c.doPOST(ctx, "/api/v1/copy-trade/futures/position/risk-limit-level/change", map[string]any{"symbol": symbol, "level": level}, &out); err != nil {
		return false, err
	}
	return out, nil
}

// SetAutoDepositStatus toggles isolated-margin auto-deposit for a position.
// DEPRECATED by KuCoin in favour of cross margin.
func (c *Client) SetAutoDepositStatus(ctx context.Context, symbol string, status bool, positionSide string) (bool, error) {
	if symbol == "" {
		return false, errInvalidRequest("SetAutoDepositStatus", "symbol is required")
	}
	var body map[string]any = map[string]any{"symbol": symbol, "status": status}
	if positionSide != "" {
		body["positionSide"] = positionSide
	}
	var out bool
	if err := c.doPOST(ctx, "/api/v1/copy-trade/futures/position/margin/auto-deposit-status", body, &out); err != nil {
		return false, err
	}
	return out, nil
}

// ---------------------------------------------------------------------
// shared internals
// ---------------------------------------------------------------------

func (c *Client) placeOrder(ctx context.Context, path string, req cttypes.OrderRequest, extra map[string]any) (*cttypes.OrderResult, error) {
	if req.ClientOid == "" || req.Symbol == "" || req.Side == "" || req.Type == "" || req.Size == "" {
		return nil, errInvalidRequest("PlaceOrder", "clientOid, symbol, side, type and size are required")
	}
	if req.Type == "limit" && req.Price == "" {
		return nil, errInvalidRequest("PlaceOrder", "price is required for limit orders")
	}
	var body map[string]any = map[string]any{
		"clientOid": req.ClientOid,
		"symbol":    req.Symbol,
		"side":      req.Side,
		"type":      req.Type,
		"size":      req.Size,
	}
	if req.MarginMode != "" {
		body["marginMode"] = req.MarginMode
	}
	if req.Leverage != "" {
		body["leverage"] = req.Leverage
	}
	if req.PositionSide != "" {
		body["positionSide"] = req.PositionSide
	}
	if req.Price != "" {
		body["price"] = req.Price
	}
	if req.TimeInForce != "" {
		body["timeInForce"] = req.TimeInForce
	}
	if req.ReduceOnly {
		body["reduceOnly"] = true
	}
	if req.Remark != "" {
		body["remark"] = req.Remark
	}
	var k string
	var v any
	for k, v = range extra {
		body[k] = v
	}
	var wire struct {
		OrderID   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}
	if err := c.doPOST(ctx, path, body, &wire); err != nil {
		return nil, err
	}
	return &cttypes.OrderResult{OrderID: wire.OrderID, ClientOid: wire.ClientOid}, nil
}

type positionWire struct {
	ID                string          `json:"id"`
	Symbol            string          `json:"symbol"`
	AutoDeposit       bool            `json:"autoDeposit"`
	MaintMarginReq    decimal.Decimal `json:"maintMarginReq"`
	RiskLimit         int64           `json:"riskLimit"`
	RealLeverage      decimal.Decimal `json:"realLeverage"`
	CrossMode         bool            `json:"crossMode"`
	MarginMode        string          `json:"marginMode"`
	PositionSide      string          `json:"positionSide"`
	Leverage          decimal.Decimal `json:"leverage"`
	DelevPercentage   decimal.Decimal `json:"delevPercentage"`
	OpeningTimestamp  int64           `json:"openingTimestamp"`
	CurrentTimestamp  int64           `json:"currentTimestamp"`
	CurrentQty        decimal.Decimal `json:"currentQty"`
	CurrentCost       decimal.Decimal `json:"currentCost"`
	CurrentComm       decimal.Decimal `json:"currentComm"`
	UnrealisedCost    decimal.Decimal `json:"unrealisedCost"`
	IsOpen            bool            `json:"isOpen"`
	MarkPrice         decimal.Decimal `json:"markPrice"`
	MarkValue         decimal.Decimal `json:"markValue"`
	PosCost           decimal.Decimal `json:"posCost"`
	PosMargin         decimal.Decimal `json:"posMargin"`
	PosMaint          decimal.Decimal `json:"posMaint"`
	MaintMargin       decimal.Decimal `json:"maintMargin"`
	RealisedPnl       decimal.Decimal `json:"realisedPnl"`
	UnrealisedPnl     decimal.Decimal `json:"unrealisedPnl"`
	UnrealisedPnlPcnt decimal.Decimal `json:"unrealisedPnlPcnt"`
	UnrealisedRoePcnt decimal.Decimal `json:"unrealisedRoePcnt"`
	AvgEntryPrice     decimal.Decimal `json:"avgEntryPrice"`
	LiquidationPrice  decimal.Decimal `json:"liquidationPrice"`
	BankruptPrice     decimal.Decimal `json:"bankruptPrice"`
	SettleCurrency    string          `json:"settleCurrency"`
}

func (w positionWire) toPosition() cttypes.Position {
	return cttypes.Position{
		ID:                w.ID,
		Symbol:            w.Symbol,
		AutoDeposit:       w.AutoDeposit,
		MaintMarginReq:    w.MaintMarginReq,
		RiskLimit:         w.RiskLimit,
		RealLeverage:      w.RealLeverage,
		CrossMode:         w.CrossMode,
		MarginMode:        w.MarginMode,
		PositionSide:      w.PositionSide,
		Leverage:          w.Leverage,
		DelevPercentage:   w.DelevPercentage,
		OpeningTimestamp:  w.OpeningTimestamp,
		CurrentTimestamp:  w.CurrentTimestamp,
		CurrentQty:        w.CurrentQty,
		CurrentCost:       w.CurrentCost,
		CurrentComm:       w.CurrentComm,
		UnrealisedCost:    w.UnrealisedCost,
		IsOpen:            w.IsOpen,
		MarkPrice:         w.MarkPrice,
		MarkValue:         w.MarkValue,
		PosCost:           w.PosCost,
		PosMargin:         w.PosMargin,
		PosMaint:          w.PosMaint,
		MaintMargin:       w.MaintMargin,
		RealisedPnl:       w.RealisedPnl,
		UnrealisedPnl:     w.UnrealisedPnl,
		UnrealisedPnlPcnt: w.UnrealisedPnlPcnt,
		UnrealisedRoePcnt: w.UnrealisedRoePcnt,
		AvgEntryPrice:     w.AvgEntryPrice,
		LiquidationPrice:  w.LiquidationPrice,
		BankruptPrice:     w.BankruptPrice,
		SettleCurrency:    w.SettleCurrency,
	}
}
