/*
Package futures is the KuCoin Futures (USD-M perpetuals) profile of the
go-kucoin SDK — the default and only profile in v1.0. It speaks the classic
Futures API on api-futures.kucoin.com and exposes four domain sub-clients:

  - MarketData — public REST: contracts, ticker, order-book snapshot,
    klines, mark price, funding rate, recent trades, server time.
  - Trading    — signed REST: place (limit/market/stop), cancel
    (by id / clientOid / all), batch place, order & stop-order queries,
    fills.
  - Account    — signed REST: account overview (balance), positions.
  - Stream     — WebSocket: public (orderbook / ticker / trades / klines)
    and private (orders / positions / balance) topics over the bullet
    token model.

Construction:

	import (
	    kucoin "github.com/tonymontanov/go-kucoin/v2"
	    "github.com/tonymontanov/go-kucoin/v2/futures"
	)

	var cfg = kucoin.DefaultConfig()
	cfg.APIKey, cfg.SecretKey, cfg.Passphrase = ...
	var c, _ = kucoin.NewClient(cfg)
	var fc = c.Futures().(*futures.Client) // requires blank/explicit futures import

SIZING: KuCoin Futures order size is an INTEGER NUMBER OF CONTRACTS, not
base quantity. One contract = SymbolInfo.Multiplier base units. Convert
before placing orders (see futures/types/contract.go).
*/
package futures
