/*
Package types holds KuCoin Futures-specific domain types — those whose
shape is dictated by the classic Futures API and therefore cannot live in
the cross-profile root types/* (shared by futures/ and a future spot/).
The protocol-common types — OrderBookLevel, OrderBookSnapshot, Candle,
Candles, Timeframe, TradeUpdate, KlineUpdate, Balance, CancelOrderRequest —
live in github.com/tonymontanov/go-kucoin/v2/types and are re-exported here
as aliases so embedders import a single types package per profile.

Layout:

  - enums.go                : alias re-exports of layer-1 enums + futures
    extensions (PositionSide, TradeType, Liquidity).
  - contract.go             : instrument spec from /api/v1/contracts/active.
  - market-ticker.go        : ticker / mark-price / funding-rate payloads.
  - order-info.go           : order state from /api/v1/orders/*.
  - position-info.go        : position state from /api/v1/position(s).
  - account.go              : account overview from /api/v1/account-overview.
  - create-order-request.go : place-order request struct.
  - fill.go                 : execution/fill row from /api/v1/fills.
  - batch-result.go         : per-row wrapper for /api/v1/orders/multi.

These types are exposed verbatim to callers (no interface{} payloads,
no map[string]string).
*/
package types
