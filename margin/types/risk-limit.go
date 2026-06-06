/*
FILE: margin/types/risk-limit.go

DESCRIPTION:
Risk-limit / borrow-configuration payloads for the KuCoin Margin profile,
mapped from GET /api/v3/margin/currencies. The endpoint returns a cross or
isolated shape depending on the isIsolated query flag.
*/

package types

import "github.com/shopspring/decimal"

// CrossRiskLimit — cross-margin risk limit + borrow config for one currency.
type CrossRiskLimit struct {
	// Currency — asset the limits apply to.
	Currency string
	// BorrowMaxAmount — maximum borrowable amount.
	BorrowMaxAmount decimal.Decimal
	// BuyMaxAmount — maximum buy amount.
	BuyMaxAmount decimal.Decimal
	// HoldMaxAmount — maximum holdable amount.
	HoldMaxAmount decimal.Decimal
	// BorrowCoefficient / MarginCoefficient — risk coefficients.
	BorrowCoefficient decimal.Decimal
	MarginCoefficient decimal.Decimal
	// Precision — decimal precision for the currency.
	Precision int
	// BorrowMinAmount / BorrowMinUnit — minimum borrow amount and step.
	BorrowMinAmount decimal.Decimal
	BorrowMinUnit   decimal.Decimal
	// BorrowEnabled — borrowing currently allowed.
	BorrowEnabled bool
	// TsMs — snapshot timestamp (ms).
	TsMs int64
}

// IsolatedRiskLimit — isolated-margin risk limit + borrow config for one pair.
type IsolatedRiskLimit struct {
	// Symbol — isolated trading pair.
	Symbol string
	// Per-leg maximum borrow / buy / hold amounts.
	BaseMaxBorrowAmount  decimal.Decimal
	QuoteMaxBorrowAmount decimal.Decimal
	BaseMaxBuyAmount     decimal.Decimal
	QuoteMaxBuyAmount    decimal.Decimal
	BaseMaxHoldAmount    decimal.Decimal
	QuoteMaxHoldAmount   decimal.Decimal
	// Per-leg precision.
	BasePrecision  int
	QuotePrecision int
	// Per-leg risk coefficients.
	BaseBorrowCoefficient  decimal.Decimal
	QuoteBorrowCoefficient decimal.Decimal
	BaseMarginCoefficient  decimal.Decimal
	QuoteMarginCoefficient decimal.Decimal
	// Per-leg minimum borrow amount + step.
	BaseBorrowMinAmount  decimal.Decimal
	BaseBorrowMinUnit    decimal.Decimal
	QuoteBorrowMinAmount decimal.Decimal
	QuoteBorrowMinUnit   decimal.Decimal
	// Per-leg borrow-enabled flags.
	BaseBorrowEnabled  bool
	QuoteBorrowEnabled bool
	// TsMs — snapshot timestamp (ms).
	TsMs int64
}
