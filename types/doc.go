/*
Package types holds the protocol-common (layer-1) types shared by every
KuCoin profile (futures, and later spot, …). These types are neutral: they
carry no profile-specific semantics and never import a profile package.

Profile packages re-export the relevant subset from their own
"<profile>/types" package (layer 2) alongside section-specific types, so
embedders import a single types package per profile.

Current contents:

  - OrderBookLevel    — one [price, size] book level (decimal).
  - OrderBookSnapshot — a full book snapshot (sequence-based for futures).

More layer-1 types (Side / OrderType / TIF / Candle / Timeframe /
TradeUpdate / KlineUpdate / Balance / CancelOrderRequest) are added as the
futures profile lands.
*/
package types
