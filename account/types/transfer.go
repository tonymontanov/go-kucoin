/*
FILE: account/types/transfer.go

DESCRIPTION:
Transfer payloads for the KuCoin Account & Funding profile, mapped from:

  - GET  /api/v1/accounts/transferable        → TransferableBalance
  - POST /api/v3/accounts/universal-transfer   → TransferResult (flex transfer)
*/

package types

import "github.com/shopspring/decimal"

// TransferableBalance — transferable balance of one wallet for one currency.
type TransferableBalance struct {
	Currency string
	// Balance — total balance.
	Balance decimal.Decimal
	// Available — free balance.
	Available decimal.Decimal
	// Holds — held balance.
	Holds decimal.Decimal
	// Transferable — amount that may be moved out right now.
	Transferable decimal.Decimal
}

// FlexTransferRequest — body for the v3 universal (flex) transfer. It moves
// assets between wallets within one account, or between master and sub.
type FlexTransferRequest struct {
	// ClientOid — caller idempotency token. Generated when empty.
	ClientOid string
	// Type — INTERNAL / PARENT_TO_SUB / SUB_TO_PARENT. Required.
	Type TransferType
	// Currency — asset to move. Required.
	Currency string
	// Amount — amount to move. Required.
	Amount decimal.Decimal
	// FromAccountType — source wallet. Required.
	FromAccountType AccountType
	// ToAccountType — destination wallet. Required.
	ToAccountType AccountType
	// FromUserID — source sub-account uid (for SUB_TO_PARENT / sub→sub).
	FromUserID string
	// ToUserID — destination sub-account uid (for PARENT_TO_SUB / sub→sub).
	ToUserID string
	// FromAccountTag / ToAccountTag — isolated-margin symbol when the wallet
	// type is ISOLATED (e.g. "BTC-USDT").
	FromAccountTag string
	ToAccountTag   string
}

// TransferResult — acknowledgement returned by a flex transfer.
type TransferResult struct {
	// OrderID — assigned transfer order id.
	OrderID string
}
