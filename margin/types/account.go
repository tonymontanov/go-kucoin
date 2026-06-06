/*
FILE: margin/types/account.go

DESCRIPTION:
Cross- and isolated-margin account payloads, mapped from
GET /api/v3/margin/accounts (cross) and GET /api/v3/isolated/accounts
(isolated).

A margin account couples balances with LIABILITIES: each currency carries
its borrowed principal + accrued interest, and the account exposes an
aggregate debt ratio that drives liquidation. The isolated view nests a
base/quote asset pair per symbol.
*/

package types

import "github.com/shopspring/decimal"

// MarginAsset — one currency row inside a margin account.
type MarginAsset struct {
	// Currency — asset code, e.g. "USDT".
	Currency string
	// Total — total balance (Available + Hold).
	Total decimal.Decimal
	// Available — free balance.
	Available decimal.Decimal
	// Hold — balance held by open orders / withdrawals.
	Hold decimal.Decimal
	// Liability — outstanding debt (principal + interest).
	Liability decimal.Decimal
	// LiabilityPrincipal / LiabilityInterest — debt breakdown.
	LiabilityPrincipal decimal.Decimal
	LiabilityInterest  decimal.Decimal
	// MaxBorrowSize — remaining borrowable amount for the currency.
	MaxBorrowSize decimal.Decimal
	// BorrowEnabled / TransferInEnabled — capability flags.
	BorrowEnabled     bool
	TransferInEnabled bool
}

// CrossMarginAccount — cross-margin account snapshot.
type CrossMarginAccount struct {
	// TotalAssetOfQuoteCurrency / TotalLiabilityOfQuoteCurrency — aggregate
	// asset / liability valued in the requested quote currency.
	TotalAssetOfQuoteCurrency     decimal.Decimal
	TotalLiabilityOfQuoteCurrency decimal.Decimal
	// DebtRatio — liability / asset ratio (drives liquidation).
	DebtRatio decimal.Decimal
	// Status — account status, e.g. "EFFECTIVE", "BANKRUPT", "LIQUIDATION".
	Status string
	// Accounts — per-currency rows.
	Accounts []MarginAsset
}

// IsolatedMarginAssetLeg — one leg (base or quote) of an isolated pair.
type IsolatedMarginAssetLeg struct {
	Currency           string
	BorrowEnabled      bool
	TransferInEnabled  bool
	Liability          decimal.Decimal
	LiabilityPrincipal decimal.Decimal
	LiabilityInterest  decimal.Decimal
	Total              decimal.Decimal
	Available          decimal.Decimal
	Hold               decimal.Decimal
	MaxBorrowSize      decimal.Decimal
}

// IsolatedMarginPair — the base/quote pair state for one isolated symbol.
type IsolatedMarginPair struct {
	// Symbol — trading pair, e.g. "BTC-USDT".
	Symbol string
	// Status — pair status, e.g. "EFFECTIVE".
	Status string
	// DebtRatio — pair-scoped liability/asset ratio.
	DebtRatio decimal.Decimal
	// BaseAsset / QuoteAsset — the two legs.
	BaseAsset  IsolatedMarginAssetLeg
	QuoteAsset IsolatedMarginAssetLeg
}

// IsolatedMarginAccount — isolated-margin account snapshot.
type IsolatedMarginAccount struct {
	TotalAssetOfQuoteCurrency     decimal.Decimal
	TotalLiabilityOfQuoteCurrency decimal.Decimal
	// TsMs — snapshot timestamp (ms).
	TsMs int64
	// Assets — per-symbol pair rows.
	Assets []IsolatedMarginPair
}
