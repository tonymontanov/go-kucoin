/*
FILE: earn/types/holding.go

DESCRIPTION:
Earn account-holding types, mapped from GET /api/v1/earn/hold-assets (paged).
*/

package types

import "github.com/shopspring/decimal"

// Holding — one currently held Earn position.
type Holding struct {
	// OrderID — holding id.
	OrderID string
	// ProductID — source product id.
	ProductID string
	// ProductCategory — product category ("KCS_STAKING", "STAKING", …).
	ProductCategory string
	// ProductType — product type ("DEMAND", "TIME", …).
	ProductType string
	// Currency — held asset.
	Currency string
	// IncomeCurrency — currency interest is paid in.
	IncomeCurrency string
	// ReturnRate — annualised return rate.
	ReturnRate decimal.Decimal
	// HoldAmount — currently held principal.
	HoldAmount decimal.Decimal
	// RedeemedAmount — amount already redeemed.
	RedeemedAmount decimal.Decimal
	// RedeemingAmount — amount pending redemption.
	RedeemingAmount decimal.Decimal
	// LockStartTime / LockEndTime — lock window (ms; End 0 when open-ended).
	LockStartTime int64
	LockEndTime   int64
	// PurchaseTime — subscription time (ms).
	PurchaseTime int64
	// RedeemPeriod — redemption delay in days.
	RedeemPeriod int
	// Status — holding status ("HOLDING", "REDEEMING", …).
	Status string
	// EarlyRedeemSupported — 1 if early redemption is allowed.
	EarlyRedeemSupported int
}

// HoldingPage — a paginated holdings response.
type HoldingPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []Holding
}

// HoldingQuery — filters for GET /api/v1/earn/hold-assets. All optional.
type HoldingQuery struct {
	// Currency — filter by asset.
	Currency string
	// ProductID — filter by product.
	ProductID string
	// ProductCategory — filter by category.
	ProductCategory string
	// CurrentPage / PageSize — pagination (pageSize 10..100).
	CurrentPage int
	PageSize    int
}
