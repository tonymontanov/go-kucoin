/*
FILE: account/types/withdrawal.go

DESCRIPTION:
Withdrawal payloads for the KuCoin Account & Funding profile, mapped from:

  - GET    /api/v1/withdrawals/quotas       → WithdrawalQuota
  - POST   /api/v3/withdrawals              → WithdrawResult
  - DELETE /api/v1/withdrawals/{id}         → cancel (no body)
  - GET    /api/v1/withdrawals              → WithdrawalRecord (paged)
  - GET    /api/v1/withdrawals/{id}         → WithdrawalRecord
*/

package types

import "github.com/shopspring/decimal"

// WithdrawalQuota — per-(currency, chain) withdrawal limits + fees.
type WithdrawalQuota struct {
	Currency string
	Chain    string
	// LimitBTCAmount / UsedBTCAmount — daily limit / used (BTC-valued).
	LimitBTCAmount decimal.Decimal
	UsedBTCAmount  decimal.Decimal
	// RemainAmount — remaining daily withdrawable (in the currency).
	RemainAmount decimal.Decimal
	// AvailableAmount — currently available balance.
	AvailableAmount decimal.Decimal
	// WithdrawMinSize / WithdrawMinFee — minimum size / fee.
	WithdrawMinSize decimal.Decimal
	WithdrawMinFee  decimal.Decimal
	// Precision — withdrawal precision (decimals).
	Precision int
	// IsWithdrawEnabled — withdrawals currently allowed.
	IsWithdrawEnabled bool
}

// WithdrawRequest — body for the v3 withdrawal.
type WithdrawRequest struct {
	// Currency — asset to withdraw. Required.
	Currency string
	// ToAddress — destination (an on-chain address, or a UID/email/phone when
	// WithdrawType selects one of those). Required.
	ToAddress string
	// Amount — amount to withdraw. Required.
	Amount decimal.Decimal
	// Chain — chain id (e.g. "trx"). Recommended; required for multi-chain
	// currencies.
	Chain string
	// WithdrawType — how ToAddress is interpreted. Defaults to ADDRESS.
	WithdrawType WithdrawType
	// IsInner — true for an internal (off-chain, no fee) KuCoin transfer.
	IsInner bool
	// Remark — optional note.
	Remark string
	// FeeDeductType — "INTERNAL" / "EXTERNAL" (optional; how the fee is taken).
	FeeDeductType string
}

// WithdrawResult — acknowledgement returned by Withdraw.
type WithdrawResult struct {
	// WithdrawalID — assigned withdrawal id.
	WithdrawalID string
}

// WithdrawalHistoryQuery — filters for GET /api/v1/withdrawals. All optional.
type WithdrawalHistoryQuery struct {
	// Currency — filter by asset (empty = all).
	Currency string
	// Status — "PROCESSING"/"WALLET_PROCESSING"/"SUCCESS"/"FAILURE".
	Status string
	// StartAtMs / EndAtMs — time window (ms).
	StartAtMs int64
	EndAtMs   int64
	// CurrentPage / PageSize — pagination.
	CurrentPage int
	PageSize    int
}

// WithdrawalRecord — one withdrawal-history row.
type WithdrawalRecord struct {
	ID       string
	Currency string
	Chain    string
	// Amount / Fee.
	Amount decimal.Decimal
	Fee    decimal.Decimal
	// Address / Memo — destination.
	Address string
	Memo    string
	// WalletTxID — on-chain transaction id.
	WalletTxID string
	// IsInner — internal (off-chain) transfer.
	IsInner bool
	// Status — "PROCESSING"/"WALLET_PROCESSING"/"SUCCESS"/"FAILURE".
	Status string
	// Remark — KuCoin remark.
	Remark string
	// CreatedAtMs / UpdatedAtMs — timestamps (ms).
	CreatedAtMs int64
	UpdatedAtMs int64
}

// WithdrawalPage — a paginated withdrawal-history response.
type WithdrawalPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []WithdrawalRecord
}
