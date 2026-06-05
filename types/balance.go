/*
FILE: types/balance.go

DESCRIPTION:
Wallet state — protocol-common across KuCoin profiles, sourced from the
account-overview endpoint (GET /api/v1/account-overview for futures) and
the private "account" WebSocket channel.

KuCoin Futures keeps ONE settlement currency per account-overview call
(USDT, USDC, XBT, …), selected via the `currency` query parameter. The
shared struct fits both single-coin (futures) and multi-coin (spot, later)
shapes: a single-coin response yields a Balance with one CoinBalance.
*/

package types

import "github.com/shopspring/decimal"

// Balance — account-level wallet state.
type Balance struct {
	// MarginCoin — settlement currency of the account (USDT / USDC / XBT).
	MarginCoin string
	// TotalEquity — account equity in MarginCoin (accountEquity).
	TotalEquity decimal.Decimal
	// AvailableBalance — funds available to open new positions / orders.
	AvailableBalance decimal.Decimal
	// LockedBalance — order margin + position margin held by the account.
	LockedBalance decimal.Decimal
	// UnrealizedPnL — unrealised PnL across open positions.
	UnrealizedPnL decimal.Decimal
	// MaintenanceMargin — maintenance margin requirement.
	MaintenanceMargin decimal.Decimal
	// Coins — per-currency breakdown. For futures the slice holds the
	// single MarginCoin entry.
	Coins []CoinBalance
}

// CoinBalance — wallet state for a single asset within Balance.
type CoinBalance struct {
	Coin           string
	Equity         decimal.Decimal
	Available      decimal.Decimal
	OrderMargin    decimal.Decimal
	PositionMargin decimal.Decimal
	FrozenFunds    decimal.Decimal
	UnrealizedPnL  decimal.Decimal
}
