/*
FILE: viplending/types/types.go

DESCRIPTION:
Types for the KuCoin VIP Lending (OTC loan) read endpoints:

  - GET /api/v1/otc-loan/discount-rate-configs → []DiscountRateConfig
  - GET /api/v1/otc-loan/loan                  → LoanInfo
  - GET /api/v1/otc-loan/accounts              → []LendingAccount
*/

package types

import "github.com/shopspring/decimal"

// DiscountLevel — one tier of the gradient collateral (discount) rate, keyed by
// the USDT-valued collateral band [Left, Right).
type DiscountLevel struct {
	// Left / Right — USDT-value band bounds.
	Left  int64
	Right int64
	// DiscountRate — collateral discount applied within the band.
	DiscountRate decimal.Decimal
}

// DiscountRateConfig — the gradient collateral rate for one currency.
type DiscountRateConfig struct {
	Currency string
	// UsdtLevels — gradient tiers ordered by collateral value.
	UsdtLevels []DiscountLevel
}

// LoanOrder — one active OTC loan order.
type LoanOrder struct {
	OrderID   string
	Currency  string
	Principal decimal.Decimal
	Interest  decimal.Decimal
}

// LTV — the consolidated loan position's loan-to-value thresholds (all decimal
// ratios). Crossing them triggers transfer locks / partial close / liquidation.
type LTV struct {
	TransferLtv           decimal.Decimal
	OnlyClosePosLtv       decimal.Decimal
	DelayedLiquidationLtv decimal.Decimal
	InstantLiquidationLtv decimal.Decimal
	CurrentLtv            decimal.Decimal
}

// MarginAsset — one collateral leg of the loan position.
type MarginAsset struct {
	MarginCcy    string
	MarginQty    decimal.Decimal
	MarginFactor decimal.Decimal
}

// LoanInfo — the caller's consolidated OTC loan position.
type LoanInfo struct {
	ParentUID            string
	Orders               []LoanOrder
	Ltv                  LTV
	TotalMarginAmount    decimal.Decimal
	TransferMarginAmount decimal.Decimal
	Margins              []MarginAsset
}

// LendingAccount — one account participating in OTC lending.
type LendingAccount struct {
	UID          string
	MarginCcy    string
	MarginQty    decimal.Decimal
	MarginFactor decimal.Decimal
	AccountType  string
	IsParent     bool
}
