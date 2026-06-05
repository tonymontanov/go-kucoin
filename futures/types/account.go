/*
FILE: futures/types/account.go

DESCRIPTION:
Account-overview payload for the KuCoin Futures profile, mapped from
GET /api/v1/account-overview?currency=. KuCoin returns a single settle
currency per call. The SDK also exposes the protocol-common Balance view
(root types.Balance) built from this struct by the account sub-client.
*/

package types

import "github.com/shopspring/decimal"

// AccountOverview — KuCoin Futures account summary for one settle currency.
type AccountOverview struct {
	Currency string

	// AccountEquity = MarginBalance + UnrealizedPnL.
	AccountEquity decimal.Decimal
	// MarginBalance = walletBalance (excludes unrealised PnL).
	MarginBalance decimal.Decimal
	// AvailableBalance — funds free to open new positions / orders.
	AvailableBalance decimal.Decimal

	// PositionMargin / OrderMargin — margin held by positions / open orders.
	PositionMargin decimal.Decimal
	OrderMargin    decimal.Decimal
	// FrozenFunds — funds frozen for withdrawals / transfers.
	FrozenFunds decimal.Decimal
	// UnrealizedPnL — unrealised PnL across open positions.
	UnrealizedPnL decimal.Decimal
}
