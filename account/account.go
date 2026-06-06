/*
FILE: account/account.go

DESCRIPTION:
Signed account sub-client: summary, API-key info, spot/margin wallet balances
and account ledgers. Converts the wire JSON into the SDK's typed structs
(decimal-normalised, ms timestamps).

ENDPOINTS:
  - GET /api/v2/user-info          account summary (sub quotas / VIP level)
  - GET /api/v1/user/api-key       API key metadata
  - GET /api/v1/accounts           wallet list (per currency + type)
  - GET /api/v1/accounts/{id}      single wallet detail
  - GET /api/v1/accounts/ledgers   spot/margin account ledgers (paged)
*/

package account

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// AccountClient — signed account-info sub-client.
type AccountClient struct {
	c *Client
}

func newAccountClient(c *Client) *AccountClient { return &AccountClient{c: c} }

// GetSummary returns the master-account summary (sub-account quotas + level).
func (a *AccountClient) GetSummary(ctx context.Context) (*accounttypes.AccountSummary, error) {
	var wire summaryWire
	if err := a.c.doGET(ctx, true, "/api/v2/user-info", nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var s accounttypes.AccountSummary = wire.toSummary()
	return &s, nil
}

// GetApiKeyInfo returns metadata for the API key in use.
func (a *AccountClient) GetApiKeyInfo(ctx context.Context) (*accounttypes.ApiKeyInfo, error) {
	var wire apiKeyWire
	if err := a.c.doGET(ctx, true, "/api/v1/user/api-key", nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info accounttypes.ApiKeyInfo = wire.toApiKeyInfo()
	return &info, nil
}

// GetAccounts returns wallet rows, optionally filtered by currency and wallet
// type. Pass an empty currency / accountType to list everything.
func (a *AccountClient) GetAccounts(ctx context.Context, currency string, accountType accounttypes.AccountType) ([]accounttypes.AccountInfo, error) {
	var query map[string]string = map[string]string{}
	if currency != "" {
		query["currency"] = currency
	}
	if accountType != "" {
		// The spot accounts endpoint expects the lowercase wallet type.
		query["type"] = strings.ToLower(string(accountType))
	}
	var wire []accountWire
	if err := a.c.doGET(ctx, true, "/api/v1/accounts", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var out []accounttypes.AccountInfo = make([]accounttypes.AccountInfo, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toAccountInfo()
	}
	return out, nil
}

// GetAccount returns a single wallet by its KuCoin account id.
func (a *AccountClient) GetAccount(ctx context.Context, accountID string) (*accounttypes.AccountInfo, error) {
	if accountID == "" {
		return nil, errInvalidRequest("GetAccount", "accountID is required")
	}
	var wire accountWire
	if err := a.c.doGET(ctx, true, "/api/v1/accounts/"+accountID, nil, queryMeta, &wire); err != nil {
		return nil, err
	}
	var info accounttypes.AccountInfo = wire.toAccountInfo()
	// The detail endpoint omits id/type; backfill the id the caller asked for.
	if info.ID == "" {
		info.ID = accountID
	}
	return &info, nil
}

// GetLedgers returns spot/margin account ledger entries (paged, latest first).
func (a *AccountClient) GetLedgers(ctx context.Context, q accounttypes.LedgerQuery) (*accounttypes.LedgerPage, error) {
	var query map[string]string = map[string]string{}
	if q.Currency != "" {
		query["currency"] = q.Currency
	}
	if q.Direction != "" {
		query["direction"] = string(q.Direction)
	}
	if q.BizType != "" {
		query["bizType"] = q.BizType
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
	var wire ledgerPageWire
	if err := a.c.doGET(ctx, true, "/api/v1/accounts/ledgers", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var page accounttypes.LedgerPage = wire.toLedgerPage()
	return &page, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type summaryWire struct {
	Level                 int `json:"level"`
	SubQuantity           int `json:"subQuantity"`
	SpotSubQuantity       int `json:"spotSubQuantity"`
	MarginSubQuantity     int `json:"marginSubQuantity"`
	FuturesSubQuantity    int `json:"futuresSubQuantity"`
	OptionSubQuantity     int `json:"optionSubQuantity"`
	MaxSubQuantity        int `json:"maxSubQuantity"`
	MaxDefaultSubQuantity int `json:"maxDefaultSubQuantity"`
	MaxSpotSubQuantity    int `json:"maxSpotSubQuantity"`
	MaxMarginSubQuantity  int `json:"maxMarginSubQuantity"`
	MaxFuturesSubQuantity int `json:"maxFuturesSubQuantity"`
	MaxOptionSubQuantity  int `json:"maxOptionSubQuantity"`
}

func (w summaryWire) toSummary() accounttypes.AccountSummary {
	return accounttypes.AccountSummary{
		Level:              w.Level,
		SubQuantity:        w.SubQuantity,
		SpotSubQuantity:    w.SpotSubQuantity,
		MarginSubQuantity:  w.MarginSubQuantity,
		FuturesSubQuantity: w.FuturesSubQuantity,
		OptionSubQuantity:  w.OptionSubQuantity,
		MaxSubQuantity:     w.MaxSubQuantity,
		MaxDefaultSubQty:   w.MaxDefaultSubQuantity,
		MaxSpotSubQty:      w.MaxSpotSubQuantity,
		MaxMarginSubQty:    w.MaxMarginSubQuantity,
		MaxFuturesSubQty:   w.MaxFuturesSubQuantity,
		MaxOptionSubQty:    w.MaxOptionSubQuantity,
	}
}

type apiKeyWire struct {
	UID         int64  `json:"uid"`
	SubName     string `json:"subName"`
	Remark      string `json:"remark"`
	APIKey      string `json:"apiKey"`
	APIVersion  int    `json:"apiVersion"`
	Permission  string `json:"permission"`
	IPWhitelist string `json:"ipWhitelist"`
	IsMaster    bool   `json:"isMaster"`
	CreatedAt   int64  `json:"createdAt"`
}

func (w apiKeyWire) toApiKeyInfo() accounttypes.ApiKeyInfo {
	return accounttypes.ApiKeyInfo{
		UID:         itoa64(w.UID),
		SubName:     w.SubName,
		Remark:      w.Remark,
		Permission:  w.Permission,
		IPWhitelist: w.IPWhitelist,
		IsMaster:    w.IsMaster,
		APIVersion:  w.APIVersion,
		CreatedAtMs: w.CreatedAt,
	}
}

type accountWire struct {
	ID        string          `json:"id"`
	Currency  string          `json:"currency"`
	Type      string          `json:"type"`
	Balance   decimal.Decimal `json:"balance"`
	Available decimal.Decimal `json:"available"`
	Holds     decimal.Decimal `json:"holds"`
}

func (w accountWire) toAccountInfo() accounttypes.AccountInfo {
	return accounttypes.AccountInfo{
		ID:        w.ID,
		Currency:  w.Currency,
		Type:      w.Type,
		Balance:   w.Balance,
		Available: w.Available,
		Holds:     w.Holds,
	}
}

type ledgerPageWire struct {
	CurrentPage int          `json:"currentPage"`
	PageSize    int          `json:"pageSize"`
	TotalNum    int          `json:"totalNum"`
	TotalPage   int          `json:"totalPage"`
	Items       []ledgerWire `json:"items"`
}

func (w ledgerPageWire) toLedgerPage() accounttypes.LedgerPage {
	var items []accounttypes.LedgerEntry = make([]accounttypes.LedgerEntry, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toLedgerEntry()
	}
	return accounttypes.LedgerPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type ledgerWire struct {
	ID          string          `json:"id"`
	Currency    string          `json:"currency"`
	Amount      decimal.Decimal `json:"amount"`
	Fee         decimal.Decimal `json:"fee"`
	Balance     decimal.Decimal `json:"balance"`
	AccountType string          `json:"accountType"`
	BizType     string          `json:"bizType"`
	Direction   string          `json:"direction"`
	Context     string          `json:"context"`
	CreatedAt   int64           `json:"createdAt"`
}

func (w ledgerWire) toLedgerEntry() accounttypes.LedgerEntry {
	return accounttypes.LedgerEntry{
		ID:          w.ID,
		Currency:    w.Currency,
		Amount:      w.Amount,
		Fee:         w.Fee,
		Balance:     w.Balance,
		AccountType: w.AccountType,
		BizType:     w.BizType,
		Direction:   accounttypes.LedgerDirection(w.Direction),
		Context:     w.Context,
		CreatedAtMs: w.CreatedAt,
	}
}
