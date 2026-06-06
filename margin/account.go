/*
FILE: margin/account.go

DESCRIPTION:
Signed account sub-client for the KuCoin Margin profile:

  - GET /api/v3/margin/accounts?quoteCurrency=&queryType=MARGIN   cross account
  - GET /api/v3/isolated/accounts?symbol=&quoteCurrency=&queryType=ISOLATED  isolated

A margin account couples balances with liabilities (borrowed principal +
interest) and an aggregate debt ratio that drives liquidation. KuCoin unified
the HF semantics: queryType "MARGIN" == HF cross, "ISOLATED" == HF isolated
(the *_V2 forms are equivalent and being phased out).
*/

package margin

import (
	"context"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// AccountClient — signed cross/isolated margin account sub-client.
type AccountClient struct {
	c *Client
}

// newAccountClient wires the sub-client to its parent.
func newAccountClient(c *Client) *AccountClient {
	return &AccountClient{c: c}
}

// accountMeta is the rate-limit metadata for account queries.
var accountMeta = rest.RequestMeta{Category: "query"}

// GetCrossAccount returns the cross-margin account snapshot. quoteCurrency
// (e.g. "USDT") values the aggregate totals; empty lets KuCoin default.
func (a *AccountClient) GetCrossAccount(ctx context.Context, quoteCurrency string) (*margintypes.CrossMarginAccount, error) {
	var query map[string]string = map[string]string{"queryType": string(margintypes.QueryCross)}
	if quoteCurrency != "" {
		query["quoteCurrency"] = quoteCurrency
	}
	var wire crossAccountWire
	if err := a.c.doGET(ctx, true, "/api/v3/margin/accounts", query, accountMeta, &wire); err != nil {
		return nil, err
	}
	var acc margintypes.CrossMarginAccount = wire.toCrossAccount()
	return &acc, nil
}

// GetIsolatedAccount returns the isolated-margin account snapshot. symbol
// scopes to one pair (empty for all); quoteCurrency values the totals.
func (a *AccountClient) GetIsolatedAccount(ctx context.Context, symbol, quoteCurrency string) (*margintypes.IsolatedMarginAccount, error) {
	var query map[string]string = map[string]string{"queryType": string(margintypes.QueryIsolated)}
	if symbol != "" {
		query["symbol"] = symbol
	}
	if quoteCurrency != "" {
		query["quoteCurrency"] = quoteCurrency
	}
	var wire isolatedAccountWire
	if err := a.c.doGET(ctx, true, "/api/v3/isolated/accounts", query, accountMeta, &wire); err != nil {
		return nil, err
	}
	var acc margintypes.IsolatedMarginAccount = wire.toIsolatedAccount()
	return &acc, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type marginAssetWire struct {
	Currency           string          `json:"currency"`
	Total              decimal.Decimal `json:"total"`
	Available          decimal.Decimal `json:"available"`
	Hold               decimal.Decimal `json:"hold"`
	Liability          decimal.Decimal `json:"liability"`
	LiabilityPrincipal decimal.Decimal `json:"liabilityPrincipal"`
	LiabilityInterest  decimal.Decimal `json:"liabilityInterest"`
	MaxBorrowSize      decimal.Decimal `json:"maxBorrowSize"`
	BorrowEnabled      bool            `json:"borrowEnabled"`
	TransferInEnabled  bool            `json:"transferInEnabled"`
}

func (w marginAssetWire) toMarginAsset() margintypes.MarginAsset {
	return margintypes.MarginAsset{
		Currency:           w.Currency,
		Total:              w.Total,
		Available:          w.Available,
		Hold:               w.Hold,
		Liability:          w.Liability,
		LiabilityPrincipal: w.LiabilityPrincipal,
		LiabilityInterest:  w.LiabilityInterest,
		MaxBorrowSize:      w.MaxBorrowSize,
		BorrowEnabled:      w.BorrowEnabled,
		TransferInEnabled:  w.TransferInEnabled,
	}
}

type crossAccountWire struct {
	TotalAssetOfQuoteCurrency     decimal.Decimal   `json:"totalAssetOfQuoteCurrency"`
	TotalLiabilityOfQuoteCurrency decimal.Decimal   `json:"totalLiabilityOfQuoteCurrency"`
	DebtRatio                     decimal.Decimal   `json:"debtRatio"`
	Status                        string            `json:"status"`
	Accounts                      []marginAssetWire `json:"accounts"`
}

func (w crossAccountWire) toCrossAccount() margintypes.CrossMarginAccount {
	var rows []margintypes.MarginAsset = make([]margintypes.MarginAsset, len(w.Accounts))
	var i int
	for i = 0; i < len(w.Accounts); i++ {
		rows[i] = w.Accounts[i].toMarginAsset()
	}
	return margintypes.CrossMarginAccount{
		TotalAssetOfQuoteCurrency:     w.TotalAssetOfQuoteCurrency,
		TotalLiabilityOfQuoteCurrency: w.TotalLiabilityOfQuoteCurrency,
		DebtRatio:                     w.DebtRatio,
		Status:                        w.Status,
		Accounts:                      rows,
	}
}

type isolatedLegWire struct {
	Currency           string          `json:"currency"`
	BorrowEnabled      bool            `json:"borrowEnabled"`
	TransferInEnabled  bool            `json:"transferInEnabled"`
	Liability          decimal.Decimal `json:"liability"`
	LiabilityPrincipal decimal.Decimal `json:"liabilityPrincipal"`
	LiabilityInterest  decimal.Decimal `json:"liabilityInterest"`
	Total              decimal.Decimal `json:"total"`
	Available          decimal.Decimal `json:"available"`
	Hold               decimal.Decimal `json:"hold"`
	MaxBorrowSize      decimal.Decimal `json:"maxBorrowSize"`
}

func (w isolatedLegWire) toLeg() margintypes.IsolatedMarginAssetLeg {
	return margintypes.IsolatedMarginAssetLeg{
		Currency:           w.Currency,
		BorrowEnabled:      w.BorrowEnabled,
		TransferInEnabled:  w.TransferInEnabled,
		Liability:          w.Liability,
		LiabilityPrincipal: w.LiabilityPrincipal,
		LiabilityInterest:  w.LiabilityInterest,
		Total:              w.Total,
		Available:          w.Available,
		Hold:               w.Hold,
		MaxBorrowSize:      w.MaxBorrowSize,
	}
}

type isolatedPairWire struct {
	Symbol     string          `json:"symbol"`
	Status     string          `json:"status"`
	DebtRatio  decimal.Decimal `json:"debtRatio"`
	BaseAsset  isolatedLegWire `json:"baseAsset"`
	QuoteAsset isolatedLegWire `json:"quoteAsset"`
}

func (w isolatedPairWire) toPair() margintypes.IsolatedMarginPair {
	return margintypes.IsolatedMarginPair{
		Symbol:     w.Symbol,
		Status:     w.Status,
		DebtRatio:  w.DebtRatio,
		BaseAsset:  w.BaseAsset.toLeg(),
		QuoteAsset: w.QuoteAsset.toLeg(),
	}
}

type isolatedAccountWire struct {
	TotalAssetOfQuoteCurrency     decimal.Decimal    `json:"totalAssetOfQuoteCurrency"`
	TotalLiabilityOfQuoteCurrency decimal.Decimal    `json:"totalLiabilityOfQuoteCurrency"`
	Timestamp                     int64              `json:"timestamp"`
	Assets                        []isolatedPairWire `json:"assets"`
}

func (w isolatedAccountWire) toIsolatedAccount() margintypes.IsolatedMarginAccount {
	var rows []margintypes.IsolatedMarginPair = make([]margintypes.IsolatedMarginPair, len(w.Assets))
	var i int
	for i = 0; i < len(w.Assets); i++ {
		rows[i] = w.Assets[i].toPair()
	}
	return margintypes.IsolatedMarginAccount{
		TotalAssetOfQuoteCurrency:     w.TotalAssetOfQuoteCurrency,
		TotalLiabilityOfQuoteCurrency: w.TotalLiabilityOfQuoteCurrency,
		TsMs:                          w.Timestamp,
		Assets:                        rows,
	}
}
