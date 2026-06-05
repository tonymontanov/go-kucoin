/*
Package kucoin is a high-performance Go SDK for the KuCoin exchange API,
targeting HFT / algorithmic trading. It speaks the KuCoin CLASSIC API
(NOT the new UTA / unified account family): every private REST call is
authenticated with the KC-API-* headers and an HMAC-SHA256 signature, and
WebSocket streams use the bullet-token connection model.

The package is organised as a domain-based "fat" client with a layered
type system:

  - kucoin.Client    — root SDK object: REST transport, signer, config,
    logger; lazily exposes domain sub-clients.
  - futures.Client   — Futures category (KuCoin USD-M perpetual futures on
    api-futures.kucoin.com). Exposes Trading / Account /
    MarketData / Stream sub-clients. Default in v1.0.
  - spot.Client      — Spot category (added in v2.0). Same shape as futures.

Type layout:

  - github.com/tonymontanov/go-kucoin/v2/types         layer 1 — protocol-common
    types reused by every profile (Side / OrderType / TIF /
    OrderBookLevel / Snapshot / Candle / Timeframe / TradeUpdate /
    KlineUpdate / CancelOrderRequest / Balance).
  - github.com/tonymontanov/go-kucoin/v2/futures/types layer 2 (profile)
    — alias re-exports of layer 1 + futures-only types
    (PositionMode, MarginMode, SymbolInfo, OrderInfo,
    Create/Modify Request, ExecutionInfo, TickerUpdate,
    PositionInfo, BatchOrderResult).

Sibling profile packages never import one another; each imports only the
neutral layer-1 package. Mixing futures methods into the spot client (or
vice versa) is impossible by construction.

Errors are typed as *kucoin.Error (Kind = Network|RateLimit|Auth|
InvalidRequest|Exchange|Unknown). Callers branch on kucoin.IsRateLimit /
kucoin.IsAuth / etc. The KuCoin business code is preserved in
Error.KucoinCode for debugging and contract tests.

Rate-limiting is exposed via Config.RateLimitEventObserver: every REST
response yields one RateLimitEvent with the path, the gw-ratelimit-*
headers and structured metadata (OrderCount/Symbols/Category) so an
external rate-limiter can model KuCoin's resource-pool budgets accurately.

WebSocket streams (orderbook/ticker/orders/positions) obtain a bullet
token, connect to the returned endpoint, reconnect with exponential
backoff + jitter, and re-subscribe transparently. The server-driven
ping/pong keep-alive is built in; users do not interact with it.

The SDK module path is github.com/tonymontanov/go-kucoin/v2. Versioning
follows semver:

  - v1.0 — Futures (USD-M perpetuals).
  - v2.0 — adds Spot.
  - v2.5 — adds the remaining sections (margin, earn, …) to the extent
    KuCoin exposes them on the classic API.

Quick start:

	import (
	    kucoin "github.com/tonymontanov/go-kucoin/v2"
	    "github.com/tonymontanov/go-kucoin/v2/futures"
	    "github.com/tonymontanov/go-kucoin/v2/futures/types"
	)

	func main() {
	    var cfg kucoin.Config = kucoin.DefaultConfig()
	    cfg.APIKey = "..."
	    cfg.SecretKey = "..."
	    cfg.Passphrase = "..."

	    var c, err = kucoin.NewClient(cfg)
	    if err != nil { panic(err) }
	    defer c.Close()

	    var fc = c.Futures().(*futures.Client)
	    _ = fc
	}

End-to-end runnable demos live under examples/.
*/
package kucoin
