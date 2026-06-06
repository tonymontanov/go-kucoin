/*
FILE: earn/orders.go

DESCRIPTION:
Earn subscribe / redeem lifecycle plus the holdings query.

ENDPOINTS:
  - POST   /api/v1/earn/orders          Purchase (subscribe)
  - DELETE /api/v1/earn/orders          Redeem
  - GET    /api/v1/earn/redeem-preview  RedeemPreview
  - GET    /api/v1/earn/hold-assets     GetHoldings (paged)
*/

package earn

import (
	"context"

	"github.com/shopspring/decimal"

	earntypes "github.com/tonymontanov/go-kucoin/v2/earn/types"
)

// Purchase subscribes to an Earn product and returns the created holding ids.
func (c *Client) Purchase(ctx context.Context, req earntypes.PurchaseRequest) (*earntypes.PurchaseResult, error) {
	if req.ProductID == "" {
		return nil, errInvalidRequest("Purchase", "productID is required")
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidRequest("Purchase", "amount must be > 0")
	}
	var accountType string = req.AccountType
	if accountType == "" {
		accountType = "MAIN"
	}
	var body map[string]any = map[string]any{
		"productId":   req.ProductID,
		"amount":      req.Amount.String(),
		"accountType": accountType,
	}
	var wire purchaseWire
	if err := c.doPOST(ctx, "/api/v1/earn/orders", body, &wire); err != nil {
		return nil, err
	}
	return &earntypes.PurchaseResult{OrderID: wire.OrderID, OrderTxID: wire.OrderTxID}, nil
}

// Redeem redeems (part of) a holding. A zero Amount redeems the full holding.
func (c *Client) Redeem(ctx context.Context, req earntypes.RedeemRequest) (*earntypes.RedeemResult, error) {
	if req.OrderID == "" {
		return nil, errInvalidRequest("Redeem", "orderID is required")
	}
	var query map[string]string = map[string]string{"orderId": req.OrderID}
	if req.Amount.GreaterThan(decimal.Zero) {
		query["amount"] = req.Amount.String()
	}
	if req.FromAccountType != "" {
		query["fromAccountType"] = req.FromAccountType
	}
	if req.ConfirmPunishRedeem != 0 {
		query["confirmPunishRedeem"] = itoa(req.ConfirmPunishRedeem)
	}
	var wire redeemWire
	if err := c.doDELETE(ctx, "/api/v1/earn/orders", query, &wire); err != nil {
		return nil, err
	}
	return &earntypes.RedeemResult{
		OrderTxID:   wire.OrderTxID,
		DeliverTime: wire.DeliverTime,
		Status:      wire.Status,
		Amount:      wire.Amount,
	}, nil
}

// RedeemPreview returns the redemption preview (penalty, deliver time) for a
// holding. fromAccountType is optional.
func (c *Client) RedeemPreview(ctx context.Context, orderID, fromAccountType string) (*earntypes.RedeemPreview, error) {
	if orderID == "" {
		return nil, errInvalidRequest("RedeemPreview", "orderID is required")
	}
	var query map[string]string = map[string]string{"orderId": orderID}
	if fromAccountType != "" {
		query["fromAccountType"] = fromAccountType
	}
	var wire redeemPreviewWire
	if err := c.doGET(ctx, "/api/v1/earn/redeem-preview", query, &wire); err != nil {
		return nil, err
	}
	return &earntypes.RedeemPreview{
		Currency:              wire.Currency,
		RedeemAmount:          wire.RedeemAmount,
		PenaltyInterestAmount: wire.PenaltyInterestAmount,
		RedeemPeriod:          wire.RedeemPeriod,
		DeliverTime:           wire.DeliverTime,
		ManualRedeemable:      wire.ManualRedeemable,
		RedeemAll:             wire.RedeemAll,
	}, nil
}

// GetHoldings returns the caller's current Earn holdings (paged).
func (c *Client) GetHoldings(ctx context.Context, q earntypes.HoldingQuery) (*earntypes.HoldingPage, error) {
	var query map[string]string = map[string]string{}
	if q.Currency != "" {
		query["currency"] = q.Currency
	}
	if q.ProductID != "" {
		query["productId"] = q.ProductID
	}
	if q.ProductCategory != "" {
		query["productCategory"] = q.ProductCategory
	}
	if q.CurrentPage > 0 {
		query["currentPage"] = itoa(q.CurrentPage)
	}
	if q.PageSize > 0 {
		query["pageSize"] = itoa(q.PageSize)
	}
	var wire holdingPageWire
	if err := c.doGET(ctx, "/api/v1/earn/hold-assets", query, &wire); err != nil {
		return nil, err
	}
	return wire.toHoldingPage(), nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type purchaseWire struct {
	OrderID   string `json:"orderId"`
	OrderTxID string `json:"orderTxId"`
}

type redeemWire struct {
	OrderTxID   string          `json:"orderTxId"`
	DeliverTime int64           `json:"deliverTime"`
	Status      string          `json:"status"`
	Amount      decimal.Decimal `json:"amount"`
}

type redeemPreviewWire struct {
	Currency              string          `json:"currency"`
	RedeemAmount          decimal.Decimal `json:"redeemAmount"`
	PenaltyInterestAmount decimal.Decimal `json:"penaltyInterestAmount"`
	RedeemPeriod          int             `json:"redeemPeriod"`
	DeliverTime           int64           `json:"deliverTime"`
	ManualRedeemable      bool            `json:"manualRedeemable"`
	RedeemAll             bool            `json:"redeemAll"`
}

type holdingPageWire struct {
	CurrentPage int           `json:"currentPage"`
	PageSize    int           `json:"pageSize"`
	TotalNum    int           `json:"totalNum"`
	TotalPage   int           `json:"totalPage"`
	Items       []holdingWire `json:"items"`
}

func (w holdingPageWire) toHoldingPage() *earntypes.HoldingPage {
	var items []earntypes.Holding = make([]earntypes.Holding, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toHolding()
	}
	return &earntypes.HoldingPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type holdingWire struct {
	OrderID              string          `json:"orderId"`
	ProductID            string          `json:"productId"`
	ProductCategory      string          `json:"productCategory"`
	ProductType          string          `json:"productType"`
	Currency             string          `json:"currency"`
	IncomeCurrency       string          `json:"incomeCurrency"`
	ReturnRate           decimal.Decimal `json:"returnRate"`
	HoldAmount           decimal.Decimal `json:"holdAmount"`
	RedeemedAmount       decimal.Decimal `json:"redeemedAmount"`
	RedeemingAmount      decimal.Decimal `json:"redeemingAmount"`
	LockStartTime        int64           `json:"lockStartTime"`
	LockEndTime          int64           `json:"lockEndTime"`
	PurchaseTime         int64           `json:"purchaseTime"`
	RedeemPeriod         int             `json:"redeemPeriod"`
	Status               string          `json:"status"`
	EarlyRedeemSupported int             `json:"earlyRedeemSupported"`
}

func (w holdingWire) toHolding() earntypes.Holding {
	return earntypes.Holding{
		OrderID:              w.OrderID,
		ProductID:            w.ProductID,
		ProductCategory:      w.ProductCategory,
		ProductType:          w.ProductType,
		Currency:             w.Currency,
		IncomeCurrency:       w.IncomeCurrency,
		ReturnRate:           w.ReturnRate,
		HoldAmount:           w.HoldAmount,
		RedeemedAmount:       w.RedeemedAmount,
		RedeemingAmount:      w.RedeemingAmount,
		LockStartTime:        w.LockStartTime,
		LockEndTime:          w.LockEndTime,
		PurchaseTime:         w.PurchaseTime,
		RedeemPeriod:         w.RedeemPeriod,
		Status:               w.Status,
		EarlyRedeemSupported: w.EarlyRedeemSupported,
	}
}
