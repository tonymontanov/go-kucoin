/*
FILE: account/types/enums.go

DESCRIPTION:
Enum surface for the KuCoin Account & Funding profile (v2.5). These encode the
wallet types, transfer kinds, withdrawal target kinds and ledger direction
used across the account/funding endpoints on api.kucoin.com.
*/

package types

// AccountType selects a KuCoin wallet. KuCoin keeps separate balances per
// wallet; funding moves assets between them. The HF migration unified the
// margin semantics: "MARGIN" == HF cross, "ISOLATED" == HF isolated.
type AccountType string

const (
	// AccountMain — funding / main wallet ("MAIN").
	AccountMain AccountType = "MAIN"
	// AccountTrade — spot trading wallet ("TRADE").
	AccountTrade AccountType = "TRADE"
	// AccountTradeHF — high-frequency spot trading wallet ("TRADE_HF").
	AccountTradeHF AccountType = "TRADE_HF"
	// AccountMargin — HF cross-margin wallet ("MARGIN").
	AccountMargin AccountType = "MARGIN"
	// AccountIsolated — HF isolated-margin wallet ("ISOLATED").
	AccountIsolated AccountType = "ISOLATED"
	// AccountContract — futures wallet ("CONTRACT").
	AccountContract AccountType = "CONTRACT"
	// AccountOption — option wallet ("OPTION").
	AccountOption AccountType = "OPTION"
)

// TransferType selects the flex (universal) transfer kind.
type TransferType string

const (
	// TransferInternal — between wallets within the same account ("INTERNAL").
	TransferInternal TransferType = "INTERNAL"
	// TransferParentToSub — master → sub-account ("PARENT_TO_SUB").
	TransferParentToSub TransferType = "PARENT_TO_SUB"
	// TransferSubToParent — sub-account → master ("SUB_TO_PARENT").
	TransferSubToParent TransferType = "SUB_TO_PARENT"
)

// WithdrawType selects how the withdrawal target is interpreted.
type WithdrawType string

const (
	// WithdrawToAddress — on-chain address ("ADDRESS"). Default.
	WithdrawToAddress WithdrawType = "ADDRESS"
	// WithdrawToUID — KuCoin user id ("UID").
	WithdrawToUID WithdrawType = "UID"
	// WithdrawToMail — registered email ("MAIL").
	WithdrawToMail WithdrawType = "MAIL"
	// WithdrawToPhone — registered phone ("PHONE").
	WithdrawToPhone WithdrawType = "PHONE"
)

// LedgerDirection filters account ledgers by flow direction.
type LedgerDirection string

const (
	// DirectionIn — inflows ("in").
	DirectionIn LedgerDirection = "in"
	// DirectionOut — outflows ("out").
	DirectionOut LedgerDirection = "out"
)
