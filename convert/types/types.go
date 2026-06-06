/*
FILE: convert/types/types.go

DESCRIPTION:
Types for the KuCoin Convert profile (fee-free currency swap), mapped from:

  - GET    /api/v1/convert/symbol             → Symbol (public)
  - GET    /api/v1/convert/currencies         → Currencies (public)
  - GET    /api/v1/convert/quote              → Quote
  - POST   /api/v1/convert/order              → PlaceResult
  - GET    /api/v1/convert/order/detail       → Order
  - GET    /api/v1/convert/order/history      → OrderPage
  - GET    /api/v1/convert/limit/quote        → LimitQuote
  - POST   /api/v1/convert/limit/order        → PlaceResult
  - GET    /api/v1/convert/limit/order/detail → LimitOrder
  - GET    /api/v1/convert/limit/orders       → LimitOrderPage
  - DELETE /api/v1/convert/limit/order/cancel → (no payload)

Convert has no trading fee; the quoted price embeds a spread. Sizes are in the
respective currency's units (one of from/to size is supplied; the other is
derived from the quote).
*/

package types

import "github.com/shopspring/decimal"

// Symbol — convertible pair limits/steps for a from→to direction.
type Symbol struct {
	FromCurrency        string
	ToCurrency          string
	FromCurrencyMaxSize decimal.Decimal
	FromCurrencyMinSize decimal.Decimal
	FromCurrencyStep    decimal.Decimal
	ToCurrencyMaxSize   decimal.Decimal
	ToCurrencyMinSize   decimal.Decimal
	ToCurrencyStep      decimal.Decimal
}

// CurrencyLimit — one convertible currency with size limits/step.
// TradeDirection ("ALL"/"BUY"/"SELL") is empty for the USDT-limit list.
type CurrencyLimit struct {
	Currency       string
	MaxSize        decimal.Decimal
	MinSize        decimal.Decimal
	Step           decimal.Decimal
	TradeDirection string
}

// Currencies — the convert currency directory.
type Currencies struct {
	Currencies        []CurrencyLimit
	USDTCurrencyLimit []CurrencyLimit
}

// QuoteRequest — params for a market or limit quote. Supply exactly one of
// FromCurrencySize / ToCurrencySize.
type QuoteRequest struct {
	FromCurrency     string
	ToCurrency       string
	FromCurrencySize string
	ToCurrencySize   string
}

// Quote — a market convert quote. Use QuoteID when placing the market order;
// it is valid until ValidUntil (ms epoch).
type Quote struct {
	QuoteID          string
	Price            decimal.Decimal
	FromCurrencySize decimal.Decimal
	ToCurrencySize   decimal.Decimal
	ValidUntil       int64
}

// LimitQuote — the protection-price threshold for a limit order; the order
// price must be ≥ Price.
type LimitQuote struct {
	Price      decimal.Decimal
	ValidUntil int64
}

// PlaceMarketRequest — body for a market convert order.
type PlaceMarketRequest struct {
	// ClientOrderID — unique id (≤40 chars). Required.
	ClientOrderID string
	// QuoteID — id from a fresh GetQuote. Required.
	QuoteID string
	// AccountType — funding source: "TRADING" / "FUNDING" / "BOTH".
	AccountType string
}

// PlaceLimitRequest — body for a limit convert order. The implied price
// (toSize/fromSize) must be ≥ the protection price from GetLimitQuote.
type PlaceLimitRequest struct {
	// ClientOrderID — unique id (≤40 chars). Required.
	ClientOrderID string
	// FromCurrency / ToCurrency — swap direction. Required.
	FromCurrency string
	ToCurrency   string
	// FromCurrencySize / ToCurrencySize — supply both for a limit order.
	FromCurrencySize string
	ToCurrencySize   string
	// AccountType — funding source: "TRADING" / "FUNDING" / "BOTH" (optional).
	AccountType string
}

// PlaceResult — acknowledgement of a placed convert (market or limit) order.
type PlaceResult struct {
	ClientOrderID string
	OrderID       string
}

// Order — a market convert order record.
type Order struct {
	ClientOrderID    string
	OrderID          string
	Price            decimal.Decimal
	FromCurrency     string
	ToCurrency       string
	FromCurrencySize decimal.Decimal
	ToCurrencySize   decimal.Decimal
	AccountType      string
	OrderTime        int64
	Status           string
}

// LimitOrder — a limit convert order record (adds the limit-order lifecycle
// fields). CancelTime / FilledTime are 0 when null; CancelType is 0 when null.
type LimitOrder struct {
	ClientOrderID    string
	OrderID          string
	Price            decimal.Decimal
	FromCurrency     string
	ToCurrency       string
	FromCurrencySize decimal.Decimal
	ToCurrencySize   decimal.Decimal
	AccountType      string
	OrderTime        int64
	Status           string
	ExpiryTime       int64
	CancelTime       int64
	FilledTime       int64
	CancelType       int
}

// HistoryQuery — filters for order/limit-order history (all optional).
type HistoryQuery struct {
	StartAt  int64
	EndAt    int64
	Page     int
	PageSize int
	Status   string
}

// OrderPage — a paginated market-order history response.
type OrderPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []Order
}

// LimitOrderPage — a paginated limit-order history response.
type LimitOrderPage struct {
	CurrentPage int
	PageSize    int
	TotalNum    int
	TotalPage   int
	Items       []LimitOrder
}
