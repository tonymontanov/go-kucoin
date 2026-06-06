/*
FILE: account/withdrawal.go

DESCRIPTION:
Signed withdrawal sub-client: quotas, submit (v3), cancel and history.

ENDPOINTS:
  - GET    /api/v1/withdrawals/quotas    per-(currency, chain) limits + fees
  - POST   /api/v3/withdrawals           submit a withdrawal
  - DELETE /api/v1/withdrawals/{id}      cancel a pending withdrawal
  - GET    /api/v1/withdrawals           history (paged)
  - GET    /api/v1/withdrawals/{id}      single withdrawal
*/

package account

import (
	"context"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// WithdrawalClient — signed withdrawal sub-client.
type WithdrawalClient struct {
	c *Client
}

func newWithdrawalClient(c *Client) *WithdrawalClient { return &WithdrawalClient{c: c} }

// GetQuotas returns withdrawal limits + fees for a currency (optionally on a
// specific chain).
func (w *WithdrawalClient) GetQuotas(ctx context.Context, currency, chain string) (*accounttypes.WithdrawalQuota, error) {
	if currency == "" {
		return nil, errInvalidRequest("GetQuotas", "currency is required")
	}
	var query map[string]string = map[string]string{"currency": currency}
	if chain != "" {
		query["chain"] = chain
	}
	var wire withdrawalQuotaWire
	if err := w.c.doGET(ctx, true, "/api/v1/withdrawals/quotas", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var q accounttypes.WithdrawalQuota = wire.toWithdrawalQuota()
	return &q, nil
}

// Withdraw submits a withdrawal and returns its assigned id.
func (w *WithdrawalClient) Withdraw(ctx context.Context, req accounttypes.WithdrawRequest) (*accounttypes.WithdrawResult, error) {
	if req.Currency == "" {
		return nil, errInvalidRequest("Withdraw", "currency is required")
	}
	if req.ToAddress == "" {
		return nil, errInvalidRequest("Withdraw", "toAddress is required")
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidRequest("Withdraw", "amount must be > 0")
	}
	var wt accounttypes.WithdrawType = req.WithdrawType
	if wt == "" {
		wt = accounttypes.WithdrawToAddress
	}
	var body map[string]any = map[string]any{
		"currency":     req.Currency,
		"toAddress":    req.ToAddress,
		"amount":       req.Amount.String(),
		"withdrawType": string(wt),
		"isInner":      req.IsInner,
	}
	if req.Chain != "" {
		body["chain"] = req.Chain
	}
	if req.Remark != "" {
		body["remark"] = req.Remark
	}
	if req.FeeDeductType != "" {
		body["feeDeductType"] = req.FeeDeductType
	}
	var wire withdrawResultWire
	if err := w.c.doPOST(ctx, "/api/v3/withdrawals", body, queryMeta, &wire); err != nil {
		return nil, err
	}
	return &accounttypes.WithdrawResult{WithdrawalID: wire.WithdrawalID}, nil
}

// Cancel cancels a pending withdrawal by id.
func (w *WithdrawalClient) Cancel(ctx context.Context, withdrawalID string) error {
	if withdrawalID == "" {
		return errInvalidRequest("Cancel", "withdrawalID is required")
	}
	return w.c.doDELETE(ctx, "/api/v1/withdrawals/"+withdrawalID, nil, queryMeta, nil)
}

// GetHistory returns paginated withdrawal history (latest first).
func (w *WithdrawalClient) GetHistory(ctx context.Context, q accounttypes.WithdrawalHistoryQuery) (*accounttypes.WithdrawalPage, error) {
	var query map[string]string = map[string]string{}
	if q.Currency != "" {
		query["currency"] = q.Currency
	}
	if q.Status != "" {
		query["status"] = q.Status
	}
	if q.StartAtMs > 0 {
		query["startAt"] = itoa64(q.StartAtMs)
	}
	if q.EndAtMs > 0 {
		query["endAt"] = itoa64(q.EndAtMs)
	}
	if q.CurrentPage > 0 {
		query["currentPage"] = itoa(q.CurrentPage)
	}
	if q.PageSize > 0 {
		query["pageSize"] = itoa(q.PageSize)
	}
	var wire withdrawalPageWire
	if err := w.c.doGET(ctx, true, "/api/v1/withdrawals", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var page accounttypes.WithdrawalPage = wire.toWithdrawalPage()
	return &page, nil
}

// GetByID returns a single withdrawal by id.
func (w *WithdrawalClient) GetByID(ctx context.Context, withdrawalID string) (*accounttypes.WithdrawalRecord, error) {
	if withdrawalID == "" {
		return nil, errInvalidRequest("GetByID", "withdrawalID is required")
	}
	var wire withdrawalRecordWire
	if err := w.c.doGET(ctx, true, "/api/v1/withdrawals/"+withdrawalID, nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var rec accounttypes.WithdrawalRecord = wire.toWithdrawalRecord()
	return &rec, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type withdrawalQuotaWire struct {
	Currency          string          `json:"currency"`
	Chain             string          `json:"chain"`
	LimitBTCAmount    decimal.Decimal `json:"limitBTCAmount"`
	UsedBTCAmount     decimal.Decimal `json:"usedBTCAmount"`
	RemainAmount      decimal.Decimal `json:"remainAmount"`
	AvailableAmount   decimal.Decimal `json:"availableAmount"`
	WithdrawMinSize   decimal.Decimal `json:"withdrawMinSize"`
	WithdrawMinFee    decimal.Decimal `json:"withdrawMinFee"`
	Precision         int             `json:"precision"`
	IsWithdrawEnabled bool            `json:"isWithdrawEnabled"`
}

func (w withdrawalQuotaWire) toWithdrawalQuota() accounttypes.WithdrawalQuota {
	return accounttypes.WithdrawalQuota{
		Currency:          w.Currency,
		Chain:             w.Chain,
		LimitBTCAmount:    w.LimitBTCAmount,
		UsedBTCAmount:     w.UsedBTCAmount,
		RemainAmount:      w.RemainAmount,
		AvailableAmount:   w.AvailableAmount,
		WithdrawMinSize:   w.WithdrawMinSize,
		WithdrawMinFee:    w.WithdrawMinFee,
		Precision:         w.Precision,
		IsWithdrawEnabled: w.IsWithdrawEnabled,
	}
}

type withdrawResultWire struct {
	WithdrawalID string `json:"withdrawalId"`
}

type withdrawalPageWire struct {
	CurrentPage int                    `json:"currentPage"`
	PageSize    int                    `json:"pageSize"`
	TotalNum    int                    `json:"totalNum"`
	TotalPage   int                    `json:"totalPage"`
	Items       []withdrawalRecordWire `json:"items"`
}

func (w withdrawalPageWire) toWithdrawalPage() accounttypes.WithdrawalPage {
	var items []accounttypes.WithdrawalRecord = make([]accounttypes.WithdrawalRecord, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toWithdrawalRecord()
	}
	return accounttypes.WithdrawalPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type withdrawalRecordWire struct {
	ID         string          `json:"id"`
	Currency   string          `json:"currency"`
	Chain      string          `json:"chain"`
	Amount     decimal.Decimal `json:"amount"`
	Fee        decimal.Decimal `json:"fee"`
	Address    string          `json:"address"`
	Memo       string          `json:"memo"`
	WalletTxID string          `json:"walletTxId"`
	IsInner    bool            `json:"isInner"`
	Status     string          `json:"status"`
	Remark     string          `json:"remark"`
	CreatedAt  int64           `json:"createdAt"`
	UpdatedAt  int64           `json:"updatedAt"`
}

func (w withdrawalRecordWire) toWithdrawalRecord() accounttypes.WithdrawalRecord {
	return accounttypes.WithdrawalRecord{
		ID:          w.ID,
		Currency:    w.Currency,
		Chain:       w.Chain,
		Amount:      w.Amount,
		Fee:         w.Fee,
		Address:     w.Address,
		Memo:        w.Memo,
		WalletTxID:  w.WalletTxID,
		IsInner:     w.IsInner,
		Status:      w.Status,
		Remark:      w.Remark,
		CreatedAtMs: w.CreatedAt,
		UpdatedAtMs: w.UpdatedAt,
	}
}
