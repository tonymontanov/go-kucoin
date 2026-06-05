/*
FILE: types/trade-update.go

DESCRIPTION:
TradeUpdate is one element of the KuCoin Futures "/contractMarket/execution"
WebSocket channel (public tape) — protocol-common across profiles.

FIELDS:
  - Symbol  : KuCoin contract symbol (e.g. "XBTUSDTM").
  - Price   : trade price.
  - Size    : trade size in CONTRACTS (futures matchSize).
  - Side    : taker/aggressor side (Buy = aggressor bought).
  - TradeID : KuCoin trade id ("tradeId").
  - TsMs    : match timestamp in MILLISECONDS. KuCoin ships execution ts in
              NANOSECONDS on the wire; the SDK converts to ms at the
              boundary so this field is uniform with the rest of the SDK.
*/

package types

import "github.com/shopspring/decimal"

// TradeUpdate — one trade event from the public execution channel.
type TradeUpdate struct {
	Symbol  string
	Price   decimal.Decimal
	Size    decimal.Decimal
	Side    SideType
	TradeID string
	TsMs    int64
}
