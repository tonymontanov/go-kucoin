/*
FILE: account/deposit.go

DESCRIPTION:
Signed deposit sub-client: create / query v3 deposit addresses and deposit
history.

ENDPOINTS:
  - POST /api/v3/deposit-address/create  create a deposit address
  - GET  /api/v3/deposit-addresses       list deposit addresses (per currency)
  - GET  /api/v1/deposits                deposit history (paged)
*/

package account

import (
	"context"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// DepositClient — signed deposit sub-client.
type DepositClient struct {
	c *Client
}

func newDepositClient(c *Client) *DepositClient { return &DepositClient{c: c} }

// CreateAddress creates a deposit address for currency on chain, crediting the
// `to` wallet ("main"/"trade"). chain/to may be empty to use KuCoin defaults.
func (d *DepositClient) CreateAddress(ctx context.Context, currency, chain, to string) (*accounttypes.DepositAddress, error) {
	if currency == "" {
		return nil, errInvalidRequest("CreateAddress", "currency is required")
	}
	var body map[string]string = map[string]string{"currency": currency}
	if chain != "" {
		body["chain"] = chain
	}
	if to != "" {
		body["to"] = to
	}
	var wire depositAddressWire
	if err := d.c.doPOST(ctx, "/api/v3/deposit-address/create", body, queryMeta, &wire); err != nil {
		return nil, err
	}
	var addr accounttypes.DepositAddress = wire.toDepositAddress()
	return &addr, nil
}

// GetAddresses returns the existing deposit addresses for a currency
// (optionally narrowed to a chain).
func (d *DepositClient) GetAddresses(ctx context.Context, currency, chain string) ([]accounttypes.DepositAddress, error) {
	if currency == "" {
		return nil, errInvalidRequest("GetAddresses", "currency is required")
	}
	var query map[string]string = map[string]string{"currency": currency}
	if chain != "" {
		query["chain"] = chain
	}
	var wire []depositAddressWire
	if err := d.c.doGET(ctx, true, "/api/v3/deposit-addresses", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var out []accounttypes.DepositAddress = make([]accounttypes.DepositAddress, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toDepositAddress()
	}
	return out, nil
}

// GetHistory returns paginated deposit history (latest first).
func (d *DepositClient) GetHistory(ctx context.Context, q accounttypes.DepositHistoryQuery) (*accounttypes.DepositPage, error) {
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
	var wire depositPageWire
	if err := d.c.doGET(ctx, true, "/api/v1/deposits", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var page accounttypes.DepositPage = wire.toDepositPage()
	return &page, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type depositAddressWire struct {
	Address        string `json:"address"`
	Memo           string `json:"memo"`
	Remark         string `json:"remark"`
	Currency       string `json:"currency"`
	ChainID        string `json:"chainId"`
	ChainName      string `json:"chainName"`
	To             string `json:"to"`
	ExpirationDate int64  `json:"expirationDate"`
}

func (w depositAddressWire) toDepositAddress() accounttypes.DepositAddress {
	return accounttypes.DepositAddress{
		Address:        w.Address,
		Memo:           w.Memo,
		Remark:         w.Remark,
		Currency:       w.Currency,
		ChainID:        w.ChainID,
		ChainName:      w.ChainName,
		To:             w.To,
		ExpirationDate: w.ExpirationDate,
	}
}

type depositPageWire struct {
	CurrentPage int                 `json:"currentPage"`
	PageSize    int                 `json:"pageSize"`
	TotalNum    int                 `json:"totalNum"`
	TotalPage   int                 `json:"totalPage"`
	Items       []depositRecordWire `json:"items"`
}

func (w depositPageWire) toDepositPage() accounttypes.DepositPage {
	var items []accounttypes.DepositRecord = make([]accounttypes.DepositRecord, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toDepositRecord()
	}
	return accounttypes.DepositPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type depositRecordWire struct {
	Currency   string          `json:"currency"`
	Chain      string          `json:"chain"`
	Amount     decimal.Decimal `json:"amount"`
	Fee        decimal.Decimal `json:"fee"`
	WalletTxID string          `json:"walletTxId"`
	Address    string          `json:"address"`
	Memo       string          `json:"memo"`
	IsInner    bool            `json:"isInner"`
	Status     string          `json:"status"`
	Remark     string          `json:"remark"`
	CreatedAt  int64           `json:"createdAt"`
	UpdatedAt  int64           `json:"updatedAt"`
}

func (w depositRecordWire) toDepositRecord() accounttypes.DepositRecord {
	return accounttypes.DepositRecord{
		Currency:    w.Currency,
		Chain:       w.Chain,
		Amount:      w.Amount,
		Fee:         w.Fee,
		WalletTxID:  w.WalletTxID,
		Address:     w.Address,
		Memo:        w.Memo,
		IsInner:     w.IsInner,
		Status:      w.Status,
		Remark:      w.Remark,
		CreatedAtMs: w.CreatedAt,
		UpdatedAtMs: w.UpdatedAt,
	}
}
