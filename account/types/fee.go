/*
FILE: account/types/fee.go

DESCRIPTION:
Fee payloads for the KuCoin Account & Funding profile, mapped from:

  - GET /api/v1/base-fee    → BaseFee (account-level spot/margin base rates)
  - GET /api/v1/trade-fees  → TradeFee (actual per-symbol rates, ≤10 symbols)
*/

package types

import "github.com/shopspring/decimal"

// BaseFee — the account's base spot/margin fee rates.
type BaseFee struct {
	TakerFeeRate decimal.Decimal
	MakerFeeRate decimal.Decimal
}

// TradeFee — actual maker/taker rates for one symbol.
type TradeFee struct {
	Symbol       string
	TakerFeeRate decimal.Decimal
	MakerFeeRate decimal.Decimal
}
