/*
FILE: margin/borrow.go

DESCRIPTION:
Signed debit sub-client for the KuCoin Margin profile — borrow, repay, their
histories, accrued-interest history and the v3 leverage update:

  - POST /api/v3/margin/borrow             Borrow
  - GET  /api/v3/margin/borrow             Get Borrow History (paged)
  - POST /api/v3/margin/repay              Repay
  - GET  /api/v3/margin/repay              Get Repay History (paged)
  - GET  /api/v3/margin/interest           Get Interest History (paged)
  - POST /api/v3/position/update-user-leverage  Modify Leverage

WIRE NOTES: borrow/repay/interest histories paginate by currentPage/pageSize
and filter by startTime/endTime (ms). createdTime is in milliseconds.
*/

package margin

import (
	"context"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// BorrowClient — signed debit (borrow/repay/interest) sub-client.
type BorrowClient struct {
	c *Client
}

// newBorrowClient wires the sub-client to its parent.
func newBorrowClient(c *Client) *BorrowClient {
	return &BorrowClient{c: c}
}

// debitMeta is the rate-limit metadata for debit calls.
var debitMeta = rest.RequestMeta{Category: "query"}

// BorrowParams — Borrow request.
type BorrowParams struct {
	// Currency — asset to borrow. Required.
	Currency string
	// Size — amount to borrow. Required.
	Size decimal.Decimal
	// TimeInForce — IOC (default) or FOK.
	TimeInForce margintypes.TimeInForceBorrow
	// IsIsolated — borrow on the isolated account (Symbol then required).
	IsIsolated bool
	// Symbol — isolated pair; required when IsIsolated is true.
	Symbol string
}

// RepayParams — Repay request.
type RepayParams struct {
	// Currency — asset to repay. Required.
	Currency string
	// Size — amount to repay. Required.
	Size decimal.Decimal
	// IsIsolated — repay on the isolated account (Symbol then required).
	IsIsolated bool
	// Symbol — isolated pair; required when IsIsolated is true.
	Symbol string
}

// DebitHistoryParams — filter / pagination for the borrow/repay/interest
// histories.
type DebitHistoryParams struct {
	Currency    string
	Symbol      string
	IsIsolated  bool
	OrderNo     string
	StartAtMs   int64
	EndAtMs     int64
	CurrentPage int
	PageSize    int
}

// Borrow initiates a cross or isolated margin borrow. Returns the borrow
// order number and the amount actually borrowed.
func (b *BorrowClient) Borrow(ctx context.Context, p BorrowParams) (*margintypes.BorrowResult, error) {
	if p.Currency == "" {
		return nil, errInvalidRequest("Borrow", "currency is required")
	}
	if p.Size.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidRequest("Borrow", "size must be > 0")
	}
	if p.IsIsolated && p.Symbol == "" {
		return nil, errInvalidRequest("Borrow", "symbol is required for isolated borrow")
	}
	var body borrowBody = borrowBody{
		Currency:    p.Currency,
		Size:        p.Size.String(),
		TimeInForce: string(p.TimeInForce),
		IsIsolated:  p.IsIsolated,
		Symbol:      p.Symbol,
	}
	if body.TimeInForce == "" {
		body.TimeInForce = string(margintypes.BorrowIOC)
	}
	var res debitAckWire
	if err := b.c.doPOST(ctx, "/api/v3/margin/borrow", body, debitMeta, &res); err != nil {
		return nil, err
	}
	return &margintypes.BorrowResult{OrderNo: res.OrderNo, ActualSize: res.ActualSize}, nil
}

// Repay initiates a cross or isolated margin repayment. Returns the repay
// order number and the amount actually repaid.
func (b *BorrowClient) Repay(ctx context.Context, p RepayParams) (*margintypes.RepayResult, error) {
	if p.Currency == "" {
		return nil, errInvalidRequest("Repay", "currency is required")
	}
	if p.Size.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidRequest("Repay", "size must be > 0")
	}
	if p.IsIsolated && p.Symbol == "" {
		return nil, errInvalidRequest("Repay", "symbol is required for isolated repay")
	}
	var body repayBody = repayBody{
		Currency:   p.Currency,
		Size:       p.Size.String(),
		IsIsolated: p.IsIsolated,
		Symbol:     p.Symbol,
	}
	var res debitAckWire
	if err := b.c.doPOST(ctx, "/api/v3/margin/repay", body, debitMeta, &res); err != nil {
		return nil, err
	}
	return &margintypes.RepayResult{OrderNo: res.OrderNo, ActualSize: res.ActualSize}, nil
}

// GetBorrowHistory returns a page of borrow-history rows.
func (b *BorrowClient) GetBorrowHistory(ctx context.Context, p DebitHistoryParams) (*margintypes.DebitPage, error) {
	return b.debitHistory(ctx, "/api/v3/margin/borrow", p)
}

// GetRepayHistory returns a page of repay-history rows.
func (b *BorrowClient) GetRepayHistory(ctx context.Context, p DebitHistoryParams) (*margintypes.DebitPage, error) {
	return b.debitHistory(ctx, "/api/v3/margin/repay", p)
}

func (b *BorrowClient) debitHistory(ctx context.Context, path string, p DebitHistoryParams) (*margintypes.DebitPage, error) {
	var query map[string]string = debitHistoryQuery(p)
	var page debitPageWire
	if err := b.c.doGET(ctx, true, path, query, debitMeta, &page); err != nil {
		return nil, err
	}
	return page.toDebitPage(), nil
}

// GetInterestHistory returns a page of accrued-interest rows.
func (b *BorrowClient) GetInterestHistory(ctx context.Context, p DebitHistoryParams) (*margintypes.InterestPage, error) {
	var query map[string]string = debitHistoryQuery(p)
	var page interestPageWire
	if err := b.c.doGET(ctx, true, "/api/v3/margin/interest", query, debitMeta, &page); err != nil {
		return nil, err
	}
	return page.toInterestPage(), nil
}

// ModifyLeverage updates the user's margin leverage (cross or isolated). For
// isolated, pass the symbol; for cross, leave it empty.
func (b *BorrowClient) ModifyLeverage(ctx context.Context, symbol string, isolated bool, leverage string) error {
	if leverage == "" {
		return errInvalidRequest("ModifyLeverage", "leverage is required")
	}
	if isolated && symbol == "" {
		return errInvalidRequest("ModifyLeverage", "symbol is required for isolated leverage")
	}
	var body leverageBody = leverageBody{Symbol: symbol, IsIsolated: isolated, Leverage: leverage}
	return b.c.doPOST(ctx, "/api/v3/position/update-user-leverage", body, debitMeta, nil)
}

// ---------------------------------------------------------------------
// Request bodies + shared query.
// ---------------------------------------------------------------------

type borrowBody struct {
	Currency    string `json:"currency"`
	Size        string `json:"size"`
	TimeInForce string `json:"timeInForce,omitempty"`
	IsIsolated  bool   `json:"isIsolated,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
}

type repayBody struct {
	Currency   string `json:"currency"`
	Size       string `json:"size"`
	IsIsolated bool   `json:"isIsolated,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
}

type leverageBody struct {
	Symbol     string `json:"symbol,omitempty"`
	IsIsolated bool   `json:"isIsolated"`
	Leverage   string `json:"leverage"`
}

func debitHistoryQuery(p DebitHistoryParams) map[string]string {
	var query map[string]string = map[string]string{}
	if p.Currency != "" {
		query["currency"] = p.Currency
	}
	if p.Symbol != "" {
		query["symbol"] = p.Symbol
	}
	if p.IsIsolated {
		query["isIsolated"] = "true"
	}
	if p.OrderNo != "" {
		query["orderNo"] = p.OrderNo
	}
	if p.StartAtMs > 0 {
		query["startTime"] = itoa(p.StartAtMs)
	}
	if p.EndAtMs > 0 {
		query["endTime"] = itoa(p.EndAtMs)
	}
	if p.CurrentPage > 0 {
		query["currentPage"] = itoa(int64(p.CurrentPage))
	}
	if p.PageSize > 0 {
		query["pageSize"] = itoa(int64(p.PageSize))
	}
	return query
}

// ---------------------------------------------------------------------
// Response wire structs + converters.
// ---------------------------------------------------------------------

type debitAckWire struct {
	OrderNo    string          `json:"orderNo"`
	ActualSize decimal.Decimal `json:"actualSize"`
}

type debitRecordWire struct {
	OrderNo     string          `json:"orderNo"`
	Symbol      string          `json:"symbol"`
	Currency    string          `json:"currency"`
	Size        decimal.Decimal `json:"size"`
	ActualSize  decimal.Decimal `json:"actualSize"`
	Principal   decimal.Decimal `json:"principal"`
	Interest    decimal.Decimal `json:"interest"`
	Status      string          `json:"status"`
	CreatedTime int64           `json:"createdTime"`
}

func (w debitRecordWire) toDebitRecord() margintypes.DebitRecord {
	return margintypes.DebitRecord{
		OrderNo:     w.OrderNo,
		Symbol:      w.Symbol,
		Currency:    w.Currency,
		Size:        w.Size,
		ActualSize:  w.ActualSize,
		Principal:   w.Principal,
		Interest:    w.Interest,
		Status:      w.Status,
		CreatedAtMs: w.CreatedTime,
	}
}

type debitPageWire struct {
	CurrentPage int               `json:"currentPage"`
	PageSize    int               `json:"pageSize"`
	TotalNum    int               `json:"totalNum"`
	TotalPage   int               `json:"totalPage"`
	Items       []debitRecordWire `json:"items"`
}

func (w debitPageWire) toDebitPage() *margintypes.DebitPage {
	var items []margintypes.DebitRecord = make([]margintypes.DebitRecord, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toDebitRecord()
	}
	return &margintypes.DebitPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type interestRecordWire struct {
	Currency       string          `json:"currency"`
	Symbol         string          `json:"symbol"`
	DayRatio       decimal.Decimal `json:"dayRatio"`
	InterestAmount decimal.Decimal `json:"interestAmount"`
	CreatedTime    int64           `json:"createdTime"`
}

func (w interestRecordWire) toInterestRecord() margintypes.InterestRecord {
	return margintypes.InterestRecord{
		Currency:       w.Currency,
		Symbol:         w.Symbol,
		DayRatio:       w.DayRatio,
		InterestAmount: w.InterestAmount,
		CreatedAtMs:    w.CreatedTime,
	}
}

type interestPageWire struct {
	CurrentPage int                  `json:"currentPage"`
	PageSize    int                  `json:"pageSize"`
	TotalNum    int                  `json:"totalNum"`
	TotalPage   int                  `json:"totalPage"`
	Items       []interestRecordWire `json:"items"`
}

func (w interestPageWire) toInterestPage() *margintypes.InterestPage {
	var items []margintypes.InterestRecord = make([]margintypes.InterestRecord, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toInterestRecord()
	}
	return &margintypes.InterestPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}
