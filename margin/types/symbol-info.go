/*
FILE: margin/types/symbol-info.go

DESCRIPTION:
Instrument specifications for the KuCoin Margin profile.

  - SymbolInfo       — cross-margin trading pair, mapped from
                       GET /api/v3/margin/symbols (data.items[]).
  - IsolatedSymbol   — isolated-margin pair config, mapped from
                       GET /api/v1/isolated/symbols.

Like Spot, margin sizing is in BASE CURRENCY (no contract multiplier). The
increments are the authoritative step sizes to round to before placing an
order:

  - BaseIncrement  — minimum step for the order SIZE (base currency).
  - QuoteIncrement — minimum step for market-order FUNDS (quote currency).
  - PriceIncrement — minimum step for the order PRICE.
*/

package types

import "github.com/shopspring/decimal"

// SymbolInfo — KuCoin cross-margin trading-pair specification.
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
	// Market — market the pair belongs to (e.g. "USDS").
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

	// EnableTrading — trading currently enabled.
	EnableTrading bool
}

// IsolatedSymbol — KuCoin isolated-margin pair configuration, mapped from
// GET /api/v1/isolated/symbols.
type IsolatedSymbol struct {
	// Symbol — trading pair, e.g. "BTC-USDT".
	Symbol string
	// SymbolName — display name.
	SymbolName string
	// BaseCurrency / QuoteCurrency — the isolated pair's two legs.
	BaseCurrency  string
	QuoteCurrency string
	// MaxLeverage — maximum leverage allowed on the pair.
	MaxLeverage int
	// FlDebtRatio — forced-liquidation debt ratio threshold.
	FlDebtRatio decimal.Decimal
	// TradeEnable — trading enabled on the pair.
	TradeEnable bool
	// AutoRenewMaxDebtRatio — debt-ratio cap for auto-renew borrowing.
	AutoRenewMaxDebtRatio decimal.Decimal
	// BaseBorrowEnable / QuoteBorrowEnable — borrowing enabled per leg.
	BaseBorrowEnable  bool
	QuoteBorrowEnable bool
	// BaseTransferInEnable / QuoteTransferInEnable — transfer-in per leg.
	BaseTransferInEnable  bool
	QuoteTransferInEnable bool
}
