/*
FILE: account/currency.go

DESCRIPTION:
Public currency-directory sub-client: the v3 currency list and single-currency
detail. These describe each coin and its supported chains (precisions,
withdraw/deposit minimums and fees) needed to build valid withdraw requests.
The endpoints are public, so they are issued unsigned.

ENDPOINTS:
  - GET /api/v3/currencies            all currencies
  - GET /api/v3/currencies/{currency} single currency
*/

package account

import (
	"context"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// CurrencyClient — public currency-directory sub-client.
type CurrencyClient struct {
	c *Client
}

func newCurrencyClient(c *Client) *CurrencyClient { return &CurrencyClient{c: c} }

// GetAll returns the full currency directory.
func (cc *CurrencyClient) GetAll(ctx context.Context) ([]accounttypes.Currency, error) {
	var wire []currencyWire
	if err := cc.c.doGET(ctx, false, "/api/v3/currencies", nil, marketMeta, &wire); err != nil {
		return nil, err
	}
	var out []accounttypes.Currency = make([]accounttypes.Currency, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toCurrency()
	}
	return out, nil
}

// Get returns a single currency. chain optionally narrows the chain list.
func (cc *CurrencyClient) Get(ctx context.Context, currency, chain string) (*accounttypes.Currency, error) {
	if currency == "" {
		return nil, errInvalidRequest("Get", "currency is required")
	}
	var query map[string]string
	if chain != "" {
		query = map[string]string{"chain": chain}
	}
	var wire currencyWire
	if err := cc.c.doGET(ctx, false, "/api/v3/currencies/"+currency, query, marketMeta, &wire); err != nil {
		return nil, err
	}
	var cur accounttypes.Currency = wire.toCurrency()
	return &cur, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type currencyWire struct {
	Currency        string      `json:"currency"`
	Name            string      `json:"name"`
	FullName        string      `json:"fullName"`
	Precision       int         `json:"precision"`
	IsMarginEnabled bool        `json:"isMarginEnabled"`
	IsDebitEnabled  bool        `json:"isDebitEnabled"`
	ContractAddress string      `json:"contractAddress"`
	Chains          []chainWire `json:"chains"`
}

func (w currencyWire) toCurrency() accounttypes.Currency {
	var chains []accounttypes.Chain = make([]accounttypes.Chain, len(w.Chains))
	var i int
	for i = 0; i < len(w.Chains); i++ {
		chains[i] = w.Chains[i].toChain()
	}
	return accounttypes.Currency{
		Currency:        w.Currency,
		Name:            w.Name,
		FullName:        w.FullName,
		Precision:       w.Precision,
		IsMarginEnabled: w.IsMarginEnabled,
		IsDebitEnabled:  w.IsDebitEnabled,
		ContractAddress: w.ContractAddress,
		Chains:          chains,
	}
}

type chainWire struct {
	ChainName         string          `json:"chainName"`
	ChainID           string          `json:"chainId"`
	WithdrawalMinSize decimal.Decimal `json:"withdrawalMinSize"`
	WithdrawalMinFee  decimal.Decimal `json:"withdrawalMinFee"`
	WithdrawFeeRate   decimal.Decimal `json:"withdrawFeeRate"`
	DepositMinSize    decimal.Decimal `json:"depositMinSize"`
	WithdrawPrecision int             `json:"withdrawPrecision"`
	Confirms          int             `json:"confirms"`
	PreConfirms       int             `json:"preConfirms"`
	MaxWithdraw       decimal.Decimal `json:"maxWithdraw"`
	MaxDeposit        decimal.Decimal `json:"maxDeposit"`
	ContractAddress   string          `json:"contractAddress"`
	NeedTag           bool            `json:"needTag"`
	IsWithdrawEnabled bool            `json:"isWithdrawEnabled"`
	IsDepositEnabled  bool            `json:"isDepositEnabled"`
}

func (w chainWire) toChain() accounttypes.Chain {
	return accounttypes.Chain{
		ChainName:         w.ChainName,
		ChainID:           w.ChainID,
		WithdrawalMinSize: w.WithdrawalMinSize,
		WithdrawalMinFee:  w.WithdrawalMinFee,
		WithdrawFeeRate:   w.WithdrawFeeRate,
		DepositMinSize:    w.DepositMinSize,
		WithdrawPrecision: w.WithdrawPrecision,
		Confirms:          w.Confirms,
		PreConfirms:       w.PreConfirms,
		MaxWithdraw:       w.MaxWithdraw,
		MaxDeposit:        w.MaxDeposit,
		ContractAddress:   w.ContractAddress,
		NeedTag:           w.NeedTag,
		IsWithdrawEnabled: w.IsWithdrawEnabled,
		IsDepositEnabled:  w.IsDepositEnabled,
	}
}
