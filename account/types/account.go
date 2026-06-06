/*
FILE: account/types/account.go

DESCRIPTION:
Account summary, balance rows, ledger entries and API-key info for the KuCoin
Account & Funding profile, mapped from:

  - GET /api/v2/user-info          → AccountSummary
  - GET /api/v1/user/api-key       → ApiKeyInfo
  - GET /api/v1/accounts[/{id}]    → AccountInfo (spot/margin wallet rows)
  - GET /api/v1/accounts/ledgers   → LedgerEntry (paged)
  - GET /api/v1/hf/accounts/ledgers→ LedgerEntry (paged, trade_hf)
*/

package types

import "github.com/shopspring/decimal"

// AccountSummary — master-account summary (sub-account quotas + VIP level).
type AccountSummary struct {
	Level              int
	SubQuantity        int
	SpotSubQuantity    int
	MarginSubQuantity  int
	FuturesSubQuantity int
	OptionSubQuantity  int
	MaxSubQuantity     int
	MaxDefaultSubQty   int
	MaxSpotSubQty      int
	MaxMarginSubQty    int
	MaxFuturesSubQty   int
	MaxOptionSubQty    int
}

// ApiKeyInfo — metadata of the API key in use.
type ApiKeyInfo struct {
	// UID — owning user id.
	UID string
	// SubName — sub-account name (empty for master).
	SubName string
	// Remark — key label.
	Remark string
	// Permission — comma-separated permission scopes.
	Permission string
	// IPWhitelist — comma-separated IP allowlist (empty = none).
	IPWhitelist string
	// IsMaster — true when the key belongs to the master account.
	IsMaster bool
	// APIVersion — key version.
	APIVersion int
	// CreatedAtMs — key creation time (ms).
	CreatedAtMs int64
}

// AccountInfo — one KuCoin wallet row (per currency + type).
type AccountInfo struct {
	// ID — KuCoin account id.
	ID string
	// Currency — asset held (e.g. "USDT").
	Currency string
	// Type — wallet type ("main"/"trade"/"margin"/…). KuCoin returns these
	// lowercase on the spot accounts endpoint.
	Type string
	// Balance — total balance (Available + Holds).
	Balance decimal.Decimal
	// Available — free balance.
	Available decimal.Decimal
	// Holds — balance held by open orders / withdrawals.
	Holds decimal.Decimal
}

// LedgerEntry — one account-ledger row (a balance-affecting event).
type LedgerEntry struct {
	// ID — ledger entry id.
	ID string
	// Currency — affected asset.
	Currency string
	// Amount — event amount (always positive; see Direction).
	Amount decimal.Decimal
	// Fee — fee charged on the event.
	Fee decimal.Decimal
	// Balance — resulting wallet balance after the event.
	Balance decimal.Decimal
	// AccountType — wallet the event touched ("TRADE"/"MAIN"/…).
	AccountType string
	// BizType — business type ("Deposit"/"Withdrawal"/"Transfer"/
	// "Trade_Exchange"/…).
	BizType string
	// Direction — "in" or "out".
	Direction LedgerDirection
	// Context — extra JSON (order/trade ids for trade events); raw string.
	Context string
	// CreatedAtMs — event time (ms).
	CreatedAtMs int64
}

// LedgerPage — a paginated ledger response.
type LedgerPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []LedgerEntry
}

// LedgerQuery — filters for the spot/margin account ledgers endpoint. All
// fields are optional; KuCoin caps the [StartAtMs, EndAtMs] window at 24h.
type LedgerQuery struct {
	// Currency — filter by asset (empty = all).
	Currency string
	// Direction — "in" / "out" (empty = both).
	Direction LedgerDirection
	// BizType — business-type filter (e.g. "TRANSFER", "TRADE_EXCHANGE").
	BizType string
	// StartAtMs / EndAtMs — time window (ms).
	StartAtMs int64
	EndAtMs   int64
	// CurrentPage / PageSize — pagination (1-based; defaults applied by KuCoin).
	CurrentPage int
	PageSize    int
}
