/*
FILE: subaccount/types/types.go

DESCRIPTION:
Types for the KuCoin Sub-Account management profile (master-account only),
mapped from:

  - POST   /api/v2/sub/user/created       → CreateResult
  - GET    /api/v2/sub/user               → SubUser (paged)
  - GET    /api/v1/sub-accounts/{id}      → SubAccountAssets
  - GET    /api/v2/sub-accounts           → SubAccountAssets (paged)
  - POST   /api/v1/sub/api-key            → CreatedAPIKey
  - GET    /api/v1/sub/api-key            → APIKey
  - POST   /api/v1/sub/api-key/update     → UpdatedAPIKey
  - DELETE /api/v1/sub/api-key            → DeletedAPIKey
*/

package types

import "github.com/shopspring/decimal"

// CreateRequest — body for creating a sub-account.
type CreateRequest struct {
	// SubName — sub-account login name (7-32 chars, letters+numbers). Required.
	SubName string
	// Password — sub-account trading/login password (7-24 chars). Required.
	Password string
	// Access — permission scope: "Spot" / "Futures" / "Margin". Required.
	Access string
	// Remarks — optional note (1-24 chars when set).
	Remarks string
}

// CreateResult — acknowledgement returned by Create.
type CreateResult struct {
	UID     int64
	SubName string
	Remarks string
	Access  string
}

// SubUser — one sub-account summary row.
type SubUser struct {
	UserID    string
	UID       int64
	SubName   string
	Status    int
	Type      int
	Access    string
	Remarks   string
	CreatedAt int64
}

// SubUserPage — a paginated sub-account summary response.
type SubUserPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []SubUser
}

// SubBalance — one currency balance leg within a wallet type.
type SubBalance struct {
	Currency          string
	Balance           decimal.Decimal
	Available         decimal.Decimal
	Holds             decimal.Decimal
	BaseCurrency      string
	BaseCurrencyPrice decimal.Decimal
	BaseAmount        decimal.Decimal
}

// SubAccountAssets — a sub-account's balances grouped by wallet type.
type SubAccountAssets struct {
	SubUserID      string
	SubName        string
	MainAccounts   []SubBalance
	TradeAccounts  []SubBalance
	MarginAccounts []SubBalance
}

// SubAccountAssetsPage — a paginated balances response.
type SubAccountAssetsPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []SubAccountAssets
}

// CreateAPIKeyRequest — body for creating a sub-account API key.
type CreateAPIKeyRequest struct {
	// SubName — target sub-account. Required.
	SubName string
	// Passphrase — API passphrase (7-32 chars). Required.
	Passphrase string
	// Remark — key label (1-32 chars). Required.
	Remark string
	// Permission — comma-separated scopes ("General","Spot","Futures",
	// "Margin","InnerTransfer","Trade"). Empty → General.
	Permission string
	// IPWhitelist — comma-separated IP allowlist (optional).
	IPWhitelist string
	// Expire — key lifetime in seconds (optional).
	Expire string
}

// CreatedAPIKey — newly created sub-account API key. APISecret + Passphrase are
// returned ONCE here and never again — persist them securely.
type CreatedAPIKey struct {
	SubName     string
	APIKey      string
	APISecret   string
	Passphrase  string
	Permission  string
	IPWhitelist string
	Remark      string
	CreatedAt   int64
}

// APIKey — an existing sub-account API key (no secret).
type APIKey struct {
	SubName     string
	APIKey      string
	Remark      string
	Permission  string
	IPWhitelist string
	CreatedAt   int64
}

// UpdateAPIKeyRequest — body for modifying a sub-account API key.
type UpdateAPIKeyRequest struct {
	// SubName / APIKey — identify the key. Required.
	SubName string
	APIKey  string
	// Passphrase — current API passphrase. Required.
	Passphrase string
	// Permission / IPWhitelist / Expire — new values (optional).
	Permission  string
	IPWhitelist string
	Expire      string
}

// UpdatedAPIKey — result of modifying a sub-account API key.
type UpdatedAPIKey struct {
	SubName     string
	APIKey      string
	Permission  string
	IPWhitelist string
}

// DeletedAPIKey — result of deleting a sub-account API key.
type DeletedAPIKey struct {
	SubName string
	APIKey  string
}
