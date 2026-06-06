/*
FILE: account/types/deposit.go

DESCRIPTION:
Deposit address + history payloads for the KuCoin Account & Funding profile,
mapped from the v3 deposit endpoints:

  - POST /api/v3/deposit-address/create  → DepositAddress
  - GET  /api/v3/deposit-addresses       → []DepositAddress
  - GET  /api/v1/deposits                → DepositRecord (paged)
*/

package types

import "github.com/shopspring/decimal"

// DepositAddress — a per-(currency, chain) deposit address.
type DepositAddress struct {
	// Address — the deposit address.
	Address string
	// Memo — address tag / memo (empty when the chain needs none).
	Memo string
	// Remark — KuCoin remark.
	Remark string
	// Currency — asset.
	Currency string
	// ChainID — internal chain id (e.g. "trx", "aptos").
	ChainID string
	// ChainName — display chain name (e.g. "TRC20", "APT").
	ChainName string
	// To — destination wallet ("MAIN"/"TRADE").
	To string
	// ExpirationDate — address expiry (ms; 0 = none).
	ExpirationDate int64
}

// DepositRecord — one deposit-history row.
type DepositRecord struct {
	// Currency — deposited asset.
	Currency string
	// Chain — chain id.
	Chain string
	// Amount — deposited amount.
	Amount decimal.Decimal
	// Fee — deposit fee.
	Fee decimal.Decimal
	// WalletTxID — on-chain transaction id.
	WalletTxID string
	// Address / Memo — destination.
	Address string
	Memo    string
	// IsInner — true for an internal (off-chain) KuCoin transfer.
	IsInner bool
	// Status — "PROCESSING"/"SUCCESS"/"FAILURE".
	Status string
	// Remark — KuCoin remark.
	Remark string
	// CreatedAtMs / UpdatedAtMs — timestamps (ms).
	CreatedAtMs int64
	UpdatedAtMs int64
}

// DepositPage — a paginated deposit-history response.
type DepositPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []DepositRecord
}

// DepositHistoryQuery — filters for GET /api/v1/deposits. All fields optional.
type DepositHistoryQuery struct {
	// Currency — filter by asset (empty = all).
	Currency string
	// Status — "PROCESSING" / "SUCCESS" / "FAILURE".
	Status string
	// StartAtMs / EndAtMs — time window (ms).
	StartAtMs int64
	EndAtMs   int64
	// CurrentPage / PageSize — pagination.
	CurrentPage int
	PageSize    int
}
