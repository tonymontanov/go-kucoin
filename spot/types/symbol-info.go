/*
FILE: spot/types/symbol-info.go

DESCRIPTION:
Instrument specification for the KuCoin Spot profile, mapped from
GET /api/v2/symbols (or /api/v2/symbols/{symbol}).

Unlike Futures, spot sizing is in BASE CURRENCY: there is no contract
multiplier. The increments below are the authoritative step sizes a caller
must round to before placing an order:

  - BaseIncrement  — minimum step for the order SIZE (base currency).
  - QuoteIncrement — minimum step for market-order FUNDS (quote currency).
  - PriceIncrement — minimum step for the order PRICE.
*/

package types

import "github.com/shopspring/decimal"

// SymbolInfo — KuCoin Spot trading-pair specification.
type SymbolInfo struct {
	// Symbol — trading pair, e.g. "BTC-USDT".
	Symbol string
	// Name — display name (usually equals Symbol).
	Name string
	// BaseCurrency / QuoteCurrency — e.g. "BTC" / "USDT".
	BaseCurrency  string
	QuoteCurrency string
	// FeeCurrency — currency fees are charged in.
	FeeCurrency string
	// Market — market the pair belongs to (e.g. "USDS", "BTC").
	Market string

	// BaseMinSize / BaseMaxSize — order size bounds (base currency).
	BaseMinSize decimal.Decimal
	BaseMaxSize decimal.Decimal
	// QuoteMinSize / QuoteMaxSize — market-order funds bounds (quote).
	QuoteMinSize decimal.Decimal
	QuoteMaxSize decimal.Decimal

	// BaseIncrement / QuoteIncrement / PriceIncrement — step sizes.
	BaseIncrement  decimal.Decimal
	QuoteIncrement decimal.Decimal
	PriceIncrement decimal.Decimal

	// PriceLimitRate — max deviation from the reference price.
	PriceLimitRate decimal.Decimal
	// MinFunds — minimum order value (quote currency).
	MinFunds decimal.Decimal

	// IsMarginEnabled — margin trading available on the pair.
	IsMarginEnabled bool
	// EnableTrading — trading currently enabled.
	EnableTrading bool
}
