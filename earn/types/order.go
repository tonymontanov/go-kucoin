/*
FILE: earn/types/order.go

DESCRIPTION:
Earn subscribe / redeem types, mapped from:

  - POST   /api/v1/earn/orders          → PurchaseResult
  - DELETE /api/v1/earn/orders          → RedeemResult
  - GET    /api/v1/earn/redeem-preview  → RedeemPreview
*/

package types

import "github.com/shopspring/decimal"

// PurchaseRequest — body for subscribing to an Earn product.
type PurchaseRequest struct {
	// ProductID — target product. Required.
	ProductID string
	// Amount — subscription amount. Required.
	Amount decimal.Decimal
	// AccountType — funding wallet ("MAIN" / "TRADE"). Defaults to "MAIN".
	AccountType string
}

// PurchaseResult — acknowledgement returned by Purchase.
type PurchaseResult struct {
	// OrderID — created holding (order) id.
	OrderID string
	// OrderTxID — subscription transaction id.
	OrderTxID string
}

// RedeemRequest — parameters for redeeming a holding.
type RedeemRequest struct {
	// OrderID — holding id to redeem. Required.
	OrderID string
	// Amount — amount to redeem (empty = full). Optional.
	Amount decimal.Decimal
	// FromAccountType — wallet to credit ("MAIN" / "TRADE"). Optional.
	FromAccountType string
	// ConfirmPunishRedeem — set 1 to confirm an early redemption that incurs a
	// penalty. Optional.
	ConfirmPunishRedeem int
}

// RedeemResult — acknowledgement returned by Redeem.
type RedeemResult struct {
	// OrderTxID — redemption transaction id.
	OrderTxID string
	// DeliverTime — expected delivery time (ms).
	DeliverTime int64
	// Status — redemption status ("PENDING", "SUCCESS", …).
	Status string
	// Amount — redeemed amount.
	Amount decimal.Decimal
}

// RedeemPreview — redemption preview for a holding.
type RedeemPreview struct {
	// Currency — asset being redeemed.
	Currency string
	// RedeemAmount — amount that would be redeemed.
	RedeemAmount decimal.Decimal
	// PenaltyInterestAmount — penalty interest for early redemption.
	PenaltyInterestAmount decimal.Decimal
	// RedeemPeriod — redemption delay in days.
	RedeemPeriod int
	// DeliverTime — expected delivery time (ms).
	DeliverTime int64
	// ManualRedeemable — whether a manual redemption is currently possible.
	ManualRedeemable bool
	// RedeemAll — whether this would redeem the entire holding.
	RedeemAll bool
}
