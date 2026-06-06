/*
FILE: spot/types/account.go

DESCRIPTION:
Account / balance payloads for the KuCoin Spot profile, mapped from
GET /api/v1/accounts (list) and GET /api/v1/accounts/{accountId}.

KuCoin Spot keeps SEPARATE accounts per (currency, type) — e.g. a "trade"
and a "main" account each hold one currency. The SDK also exposes the
protocol-common Balance view (root types.Balance) aggregated from these rows
by the account sub-client.
*/

package types

import "github.com/shopspring/decimal"

// AccountType selects which KuCoin spot account a query targets.
type AccountType string

const (
	// AccountTrade — spot trading account ("trade").
	AccountTrade AccountType = "trade"
	// AccountMain — funding / main account ("main").
	AccountMain AccountType = "main"
	// AccountMargin — cross-margin account ("margin").
	AccountMargin AccountType = "margin"
)

// AccountInfo — one KuCoin spot account row.
type AccountInfo struct {
	// ID — KuCoin account id.
	ID string
	// Currency — asset held (e.g. "USDT").
	Currency string
	// Type — "trade"/"main"/"margin".
	Type AccountType
	// Balance — total balance (Available + Holds).
	Balance decimal.Decimal
	// Available — free balance.
	Available decimal.Decimal
	// Holds — balance held by open orders / withdrawals.
	Holds decimal.Decimal
}
