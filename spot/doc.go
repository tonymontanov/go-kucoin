/*
Package spot is the KuCoin Spot profile of the go-kucoin SDK (added in
v2.0). It speaks the classic Spot API on api.kucoin.com and exposes four
domain sub-clients:

  - MarketData — public REST: symbols, level1 ticker, 24h stats, order-book
    snapshot, klines, recent trades, server time.
  - Trading    — signed REST: place (limit/market), cancel (by id /
    clientOid / all), batch place, order queries, fills.
  - Account    — signed REST: accounts (balances) + a protocol-common
    Balance adapter.
  - Stream     — WebSocket: public (orderbook / ticker / trades / klines)
    and private (orders / balance) topics over the bullet token model.

Construction:

	import (
	    kucoin "github.com/tonymontanov/go-kucoin/v2"
	    "github.com/tonymontanov/go-kucoin/v2/spot"
	)

	var cfg = kucoin.DefaultConfig()
	cfg.APIKey, cfg.SecretKey, cfg.Passphrase = ...
	var c, _ = kucoin.NewClient(cfg)
	var sc = c.Spot().(*spot.Client) // requires blank/explicit spot import

HOST: the root Config defaults its REST host to the FUTURES endpoint, so the
spot profile builds its OWN REST client against api.kucoin.com (via
kucoin.Client.NewSectionRESTClient). An explicit non-futures Config.REST.BaseURL
(e.g. a mock server) is honoured as-is for testing.

SIZING: KuCoin Spot order size is in BASE CURRENCY (decimal), NOT contracts —
there is no multiplier. Round Size to SymbolInfo.BaseIncrement and Price to
SymbolInfo.PriceIncrement before placing orders.
*/
package spot
