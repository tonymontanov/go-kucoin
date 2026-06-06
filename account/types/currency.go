/*
FILE: account/types/currency.go

DESCRIPTION:
Currency directory payloads for the KuCoin Account & Funding profile, mapped
from the v3 currency endpoints (public):

  - GET /api/v3/currencies            → []Currency
  - GET /api/v3/currencies/{currency} → Currency

These describe each coin and its supported chains — precisions, withdraw/deposit
minimums and fees — needed to build valid withdraw requests.
*/

package types

import "github.com/shopspring/decimal"

// Chain — one network a currency can be deposited/withdrawn on.
type Chain struct {
	// ChainName — display name (e.g. "TRC20").
	ChainName string
	// ChainID — internal id used as the "chain" param (e.g. "trx").
	ChainID string
	// WithdrawalMinSize / WithdrawalMinFee — minimum withdrawal size / fee.
	WithdrawalMinSize decimal.Decimal
	WithdrawalMinFee  decimal.Decimal
	// WithdrawFeeRate — proportional withdrawal fee rate.
	WithdrawFeeRate decimal.Decimal
	// DepositMinSize — minimum deposit (zero when KuCoin returns null).
	DepositMinSize decimal.Decimal
	// WithdrawPrecision — decimals allowed on withdrawal.
	WithdrawPrecision int
	// Confirms / PreConfirms — confirmations for credit / pre-credit.
	Confirms    int
	PreConfirms int
	// MaxWithdraw / MaxDeposit — caps (zero when KuCoin returns null).
	MaxWithdraw decimal.Decimal
	MaxDeposit  decimal.Decimal
	// ContractAddress — token contract on this chain (empty for native).
	ContractAddress string
	// NeedTag — whether deposits/withdrawals require a memo/tag.
	NeedTag bool
	// IsWithdrawEnabled / IsDepositEnabled — per-chain toggles.
	IsWithdrawEnabled bool
	IsDepositEnabled  bool
}

// Currency — a coin and its supported chains.
type Currency struct {
	// Currency — fixed code, the only stable identity of the coin.
	Currency string
	// Name / FullName — display names (mutable).
	Name     string
	FullName string
	// Precision — account precision (decimals).
	Precision int
	// IsMarginEnabled / IsDebitEnabled — margin eligibility flags.
	IsMarginEnabled bool
	IsDebitEnabled  bool
	// ContractAddress — primary contract (empty for native coins).
	ContractAddress string
	// Chains — supported networks.
	Chains []Chain
}
