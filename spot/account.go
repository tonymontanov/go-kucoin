/*
FILE: spot/account.go

DESCRIPTION:
Signed account sub-client for the KuCoin Spot profile:

  - GET /api/v1/accounts?type=&currency=   list accounts (balances)
  - GET /api/v1/accounts/{accountId}        one account

GetAccounts returns the KuCoin-shaped rows; GetBalance adapts the spot
trading account(s) into the protocol-common root types.Balance so desk code
can stay profile-agnostic.
*/

package spot

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// AccountClient — signed account sub-client.
type AccountClient struct {
	c *Client
}

// newAccountClient wires the sub-client to its parent.
func newAccountClient(c *Client) *AccountClient {
	return &AccountClient{c: c}
}

// accountMeta is the rate-limit metadata for account queries.
var accountMeta = rest.RequestMeta{Category: "query"}

// GetAccounts returns the account rows, optionally filtered by currency and
// account type. Empty filters return every account on the key.
func (a *AccountClient) GetAccounts(ctx context.Context, currency string, accountType spottypes.AccountType) ([]spottypes.AccountInfo, error) {
	var query map[string]string = map[string]string{}
	if currency != "" {
		query["currency"] = currency
	}
	if accountType != "" {
		query["type"] = string(accountType)
	}
	var wire []accountWire
	if err := a.c.doGET(ctx, true, "/api/v1/accounts", query, accountMeta, &wire); err != nil {
		return nil, err
	}
	var out []spottypes.AccountInfo = make([]spottypes.AccountInfo, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toAccountInfo()
	}
	return out, nil
}

// GetBalance returns the spot TRADE account state for a currency as the
// protocol-common Balance. When the currency holds no trade account the
// returned Balance is zero-valued (MarginCoin set).
func (a *AccountClient) GetBalance(ctx context.Context, currency string) (*roottypes.Balance, error) {
	if currency == "" {
		return nil, errInvalidRequest("GetBalance", "currency is required")
	}
	var rows []spottypes.AccountInfo
	var err error
	rows, err = a.GetAccounts(ctx, currency, spottypes.AccountTrade)
	if err != nil {
		return nil, err
	}
	var bal roottypes.Balance = roottypes.Balance{MarginCoin: currency}
	var i int
	for i = 0; i < len(rows); i++ {
		if rows[i].Currency != currency {
			continue
		}
		bal.TotalEquity = rows[i].Balance
		bal.AvailableBalance = rows[i].Available
		bal.LockedBalance = rows[i].Holds
		bal.Coins = []roottypes.CoinBalance{
			{
				Coin:        rows[i].Currency,
				Equity:      rows[i].Balance,
				Available:   rows[i].Available,
				FrozenFunds: rows[i].Holds,
			},
		}
		break
	}
	return &bal, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

// accountWire mirrors one element of /api/v1/accounts. Money fields are
// strings; decimal decodes them.
type accountWire struct {
	ID        string          `json:"id"`
	Currency  string          `json:"currency"`
	Type      string          `json:"type"`
	Balance   decimal.Decimal `json:"balance"`
	Available decimal.Decimal `json:"available"`
	Holds     decimal.Decimal `json:"holds"`
}

func (w accountWire) toAccountInfo() spottypes.AccountInfo {
	return spottypes.AccountInfo{
		ID:        w.ID,
		Currency:  w.Currency,
		Type:      spottypes.AccountType(w.Type),
		Balance:   w.Balance,
		Available: w.Available,
		Holds:     w.Holds,
	}
}
