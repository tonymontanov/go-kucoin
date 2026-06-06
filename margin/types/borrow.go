/*
FILE: margin/types/borrow.go

DESCRIPTION:
Debit (borrow/repay/interest) payloads for the KuCoin Margin profile, mapped
from the v3 debit endpoints:

  - POST /api/v3/margin/borrow    Borrow            → BorrowResult
  - GET  /api/v3/margin/borrow    Get Borrow History → DebitRecord (paged)
  - POST /api/v3/margin/repay     Repay             → RepayResult
  - GET  /api/v3/margin/repay     Get Repay History  → DebitRecord (paged)
  - GET  /api/v3/margin/interest  Get Interest History → InterestRecord (paged)
*/

package types

import "github.com/shopspring/decimal"

// TimeInForceBorrow selects the borrow order's time-in-force. KuCoin accepts
// IOC (fill now or cancel) or FOK (fill in full or cancel).
type TimeInForceBorrow string

const (
	// BorrowIOC — Immediate Or Cancel.
	BorrowIOC TimeInForceBorrow = "IOC"
	// BorrowFOK — Fill Or Kill.
	BorrowFOK TimeInForceBorrow = "FOK"
)

// BorrowResult — acknowledgement returned by Borrow.
type BorrowResult struct {
	// OrderNo — borrow order number.
	OrderNo string
	// ActualSize — amount actually borrowed.
	ActualSize decimal.Decimal
}

// RepayResult — acknowledgement returned by Repay.
type RepayResult struct {
	// OrderNo — repay order number.
	OrderNo string
	// ActualSize — amount actually repaid.
	ActualSize decimal.Decimal
}

// DebitRecord — one borrow- or repay-history row.
type DebitRecord struct {
	// OrderNo — order number.
	OrderNo string
	// Symbol — isolated pair (empty for cross).
	Symbol string
	// Currency — borrowed/repaid asset.
	Currency string
	// Size — requested amount.
	Size decimal.Decimal
	// ActualSize — amount actually borrowed/repaid.
	ActualSize decimal.Decimal
	// Principal / Interest — repay-history breakdown (zero on borrow rows).
	Principal decimal.Decimal
	Interest  decimal.Decimal
	// Status — "PROCESSING"/"SUCCESS"/"FAILED".
	Status string
	// CreatedAtMs — creation time (ms).
	CreatedAtMs int64
}

// InterestRecord — one accrued-interest row.
type InterestRecord struct {
	// Currency — asset interest accrued on.
	Currency string
	// Symbol — isolated pair (empty for cross).
	Symbol string
	// DayRatio — daily interest rate.
	DayRatio decimal.Decimal
	// InterestAmount — interest charged for the period.
	InterestAmount decimal.Decimal
	// CreatedAtMs — accrual time (ms).
	CreatedAtMs int64
}

// DebitPage — a paginated debit/interest history response.
type DebitPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []DebitRecord
}

// InterestPage — a paginated interest history response.
type InterestPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []InterestRecord
}
