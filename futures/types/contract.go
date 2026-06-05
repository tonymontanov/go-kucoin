/*
FILE: futures/types/contract.go

DESCRIPTION:
Instrument specification for a KuCoin Futures contract, mapped from
GET /api/v1/contracts/active and GET /api/v1/contracts/{symbol}.

Only the fields a market maker needs for order sizing, price/qty rounding
and risk are surfaced; the raw response carries many more (funding symbols,
index symbols, etc.) that the SDK omits to keep the type focused. All
numeric fields arrive as JSON numbers on the wire and are normalised into
decimal.Decimal.

SIZING NOTE (critical):
KuCoin Futures order `size` is an INTEGER NUMBER OF CONTRACTS, not the base
quantity. One contract equals `Multiplier` units of the base currency
(e.g. XBTUSDTM multiplier = 0.001 BTC). To trade N base units, send
size = N / Multiplier (must be a whole number of LotSize steps).
*/

package types

import "github.com/shopspring/decimal"

// SymbolInfo — KuCoin Futures contract specification.
type SymbolInfo struct {
	// Symbol — contract symbol (e.g. "XBTUSDTM").
	Symbol string
	// RootSymbol — settle-currency family (e.g. "USDT").
	RootSymbol string
	// Type — contract type code (e.g. "FFWCSX" perpetual).
	Type string
	// BaseCurrency / QuoteCurrency / SettleCurrency.
	BaseCurrency   string
	QuoteCurrency  string
	SettleCurrency string
	// Status — "Open", "BeingSettled", "Paused", "Closed", ...
	Status string
	// IsInverse — true for coin-margined (inverse) contracts.
	IsInverse bool

	// Multiplier — base units per 1 contract. Drives size conversion.
	Multiplier decimal.Decimal
	// LotSize — minimum order size step in contracts.
	LotSize decimal.Decimal
	// TickSize — minimum price increment.
	TickSize decimal.Decimal
	// IndexPriceTickSize — index price increment.
	IndexPriceTickSize decimal.Decimal
	// MaxOrderQty — maximum order size in contracts (limit orders).
	MaxOrderQty decimal.Decimal
	// MaxPrice — maximum order price.
	MaxPrice decimal.Decimal

	// MaxLeverage — maximum leverage allowed.
	MaxLeverage decimal.Decimal
	// InitialMargin / MaintainMargin — base margin rates.
	InitialMargin  decimal.Decimal
	MaintainMargin decimal.Decimal
	// MakerFeeRate / TakerFeeRate — fee rates.
	MakerFeeRate decimal.Decimal
	TakerFeeRate decimal.Decimal

	// MarkPrice / IndexPrice / LastTradePrice — last snapshot prices.
	MarkPrice      decimal.Decimal
	IndexPrice     decimal.Decimal
	LastTradePrice decimal.Decimal

	// FundingFeeRate — current funding rate.
	FundingFeeRate decimal.Decimal
	// OpenInterest — open interest in contracts (KuCoin ships it as string).
	OpenInterest decimal.Decimal
	// VolumeOf24h / TurnoverOf24h — 24h rolling stats.
	VolumeOf24h   decimal.Decimal
	TurnoverOf24h decimal.Decimal
}
