/*
FILE: account/transfer.go

DESCRIPTION:
Signed transfer sub-client: transferable balance and the v3 flex (universal)
transfer that moves assets between wallets and between master / sub-accounts.

ENDPOINTS:
  - GET  /api/v1/accounts/transferable        transferable balance of a wallet
  - POST /api/v3/accounts/universal-transfer    flex transfer
*/

package account

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// TransferClient — signed transfer sub-client.
type TransferClient struct {
	c *Client
}

func newTransferClient(c *Client) *TransferClient { return &TransferClient{c: c} }

// GetTransferable returns the transferable balance of a wallet for a currency.
// tag is the isolated-margin symbol (e.g. "BTC-USDT") when accountType is
// ISOLATED; pass "" otherwise.
func (t *TransferClient) GetTransferable(ctx context.Context, currency string, accountType accounttypes.AccountType, tag string) (*accounttypes.TransferableBalance, error) {
	if currency == "" {
		return nil, errInvalidRequest("GetTransferable", "currency is required")
	}
	if accountType == "" {
		return nil, errInvalidRequest("GetTransferable", "accountType is required")
	}
	var query map[string]string = map[string]string{
		"currency": currency,
		"type":     string(accountType),
	}
	if tag != "" {
		query["tag"] = tag
	}
	var wire transferableWire
	if err := t.c.doGET(ctx, true, "/api/v1/accounts/transferable", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var b accounttypes.TransferableBalance = wire.toTransferableBalance()
	return &b, nil
}

// FlexTransfer moves assets between wallets (or master/sub). A clientOid is
// generated when the caller leaves it empty.
func (t *TransferClient) FlexTransfer(ctx context.Context, req accounttypes.FlexTransferRequest) (*accounttypes.TransferResult, error) {
	if req.Type == "" {
		return nil, errInvalidRequest("FlexTransfer", "type is required")
	}
	if req.Currency == "" {
		return nil, errInvalidRequest("FlexTransfer", "currency is required")
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidRequest("FlexTransfer", "amount must be > 0")
	}
	if req.FromAccountType == "" || req.ToAccountType == "" {
		return nil, errInvalidRequest("FlexTransfer", "fromAccountType and toAccountType are required")
	}
	var clientOid string = req.ClientOid
	if clientOid == "" {
		clientOid = generateClientOid()
	}
	var body map[string]any = map[string]any{
		"clientOid":       clientOid,
		"type":            string(req.Type),
		"currency":        req.Currency,
		"amount":          req.Amount.String(),
		"fromAccountType": string(req.FromAccountType),
		"toAccountType":   string(req.ToAccountType),
	}
	if req.FromUserID != "" {
		body["fromUserId"] = req.FromUserID
	}
	if req.ToUserID != "" {
		body["toUserId"] = req.ToUserID
	}
	if req.FromAccountTag != "" {
		body["fromAccountTag"] = req.FromAccountTag
	}
	if req.ToAccountTag != "" {
		body["toAccountTag"] = req.ToAccountTag
	}
	var wire transferResultWire
	if err := t.c.doPOST(ctx, "/api/v3/accounts/universal-transfer", body, queryMeta, &wire); err != nil {
		return nil, err
	}
	return &accounttypes.TransferResult{OrderID: wire.OrderID}, nil
}

// InnerTransfer is a convenience wrapper for the common INTERNAL flex transfer
// between two wallets of the same account.
func (t *TransferClient) InnerTransfer(ctx context.Context, currency string, amount decimal.Decimal, from, to accounttypes.AccountType) (*accounttypes.TransferResult, error) {
	return t.FlexTransfer(ctx, accounttypes.FlexTransferRequest{
		Type:            accounttypes.TransferInternal,
		Currency:        strings.TrimSpace(currency),
		Amount:          amount,
		FromAccountType: from,
		ToAccountType:   to,
	})
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type transferableWire struct {
	Currency     string          `json:"currency"`
	Balance      decimal.Decimal `json:"balance"`
	Available    decimal.Decimal `json:"available"`
	Holds        decimal.Decimal `json:"holds"`
	Transferable decimal.Decimal `json:"transferable"`
}

func (w transferableWire) toTransferableBalance() accounttypes.TransferableBalance {
	return accounttypes.TransferableBalance{
		Currency:     w.Currency,
		Balance:      w.Balance,
		Available:    w.Available,
		Holds:        w.Holds,
		Transferable: w.Transferable,
	}
}

type transferResultWire struct {
	OrderID string `json:"orderId"`
}
