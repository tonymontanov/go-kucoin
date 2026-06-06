/*
Package types holds KuCoin Spot-specific domain types — those whose shape is
dictated by the classic Spot API (api.kucoin.com) and therefore cannot live
in the cross-profile root types/* (shared by futures/ and spot/). The
protocol-common types — OrderBookLevel, OrderBookSnapshot, Candle, Candles,
Timeframe, TradeUpdate, KlineUpdate, Balance, CancelOrderRequest — live in
github.com/tonymontanov/go-kucoin/v2/types and are re-exported here as
aliases so embedders import a single types package per profile.

KEY DIFFERENCES vs the Futures profile:

  - SIZING is in BASE CURRENCY (decimal), NOT integer contracts. There is no
    multiplier; SymbolInfo carries baseIncrement / priceIncrement so callers
    can round size/price to the instrument step.
  - NO leverage, position, margin mode or funding on a plain spot order.
  - Market orders take EITHER size (base) OR funds (quote).
  - Extra time-in-force values GTT/FOK and self-trade-prevention (STP).

Layout:

  - enums.go                : layer-1 enum aliases + spot TIF/STP/TradeType.
  - symbol-info.go          : instrument spec from /api/v2/symbols.
  - ticker.go               : level1 ticker + 24h stats payloads.
  - order-info.go           : order state from /api/v1/orders/*.
  - create-order-request.go : place-order request struct.
  - fill.go                 : execution/fill row from /api/v1/fills.
  - batch-result.go         : per-row wrapper for /api/v1/orders/multi.
  - account.go              : account/balance row from /api/v1/accounts.

These types are exposed verbatim to callers (no interface{} payloads,
no map[string]string).
*/
package types
