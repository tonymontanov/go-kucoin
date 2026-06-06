/*
FILE: earn/types/product.go

DESCRIPTION:
Earn product catalogue types. The five product-list endpoints (savings,
promotion, staking, KCS staking, ETH staking) all share the SAME row shape, so
one Product struct covers them:

  - GET /api/v1/earn/saving/products
  - GET /api/v1/earn/promotion/products
  - GET /api/v1/earn/staking/products
  - GET /api/v1/earn/kcs-staking/products
  - GET /api/v1/earn/eth-staking/products
*/

package types

import "github.com/shopspring/decimal"

// Product — one Earn product offering.
type Product struct {
	// ID — product id, used as productId when subscribing.
	ID string
	// Currency — staked/subscribed asset.
	Currency string
	// Category — product category ("DEMAND", "KCS_STAKING", "STAKING", …).
	Category string
	// Type — product type ("DEMAND", "TIME", …).
	Type string
	// Precision — amount precision (decimals).
	Precision int
	// ProductUpperLimit — total product cap.
	ProductUpperLimit decimal.Decimal
	// ProductRemainAmount — remaining subscribable amount.
	ProductRemainAmount decimal.Decimal
	// UserUpperLimit / UserLowerLimit — per-user max / min subscription.
	UserUpperLimit decimal.Decimal
	UserLowerLimit decimal.Decimal
	// ReturnRate — annualised return rate.
	ReturnRate decimal.Decimal
	// IncomeCurrency — currency interest is paid in.
	IncomeCurrency string
	// RedeemPeriod — redemption delay in days.
	RedeemPeriod int
	// LockStartTime / LockEndTime — lock window (ms; End 0 when open-ended).
	LockStartTime int64
	LockEndTime   int64
	// ApplyStartTime / ApplyEndTime — subscription window (ms; End 0 = open).
	ApplyStartTime int64
	ApplyEndTime   int64
	// Duration — lock duration in days (0 for demand).
	Duration int
	// EarlyRedeemSupported — 1 if early redemption is allowed.
	EarlyRedeemSupported int
	// Status — product status ("ONGOING", "FULL", …).
	Status string
	// RedeemType — "MANUAL" / "AUTO".
	RedeemType string
	// IncomeReleaseType — "DAILY" / "AT_MATURITY", …
	IncomeReleaseType string
	// InterestDate — first interest accrual time (ms).
	InterestDate int64
	// NewUserOnly — 1 if restricted to new users.
	NewUserOnly int
}
