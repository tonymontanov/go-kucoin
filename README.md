# go-kucoin

High-performance, low-latency Go SDK for the [KuCoin](https://www.kucoin.com/) exchange,
built for HFT / market-making workloads.

- **Module:** `github.com/tonymontanov/go-kucoin/v2`
- **Go:** 1.24+
- **API target:** KuCoin **Classic** API (not the new UTA / unified-account family)
- **Status:** **v1.0 — Futures (USD-M perpetuals)** complete and live-validated. Spot is on the roadmap (v2.0).

The design mirrors the sibling in-house SDKs (`go-okx` / `go-bybit` / `go-bitget`):
a neutral transport core plus thin, section-specific profiles.

---

## Features (Futures v1.0)

**Market data (REST)**
- Server time, contract specs (`multiplier` / `tickSize` / `lotSize`), tickers
- Level-2 order-book snapshot, klines, mark price, funding rate, recent trades

**Trading (REST)**
- Place limit / market / stop orders, batch place (`orders/multi`)
- Cancel by id / clientOid / cancel-all / cancel-all-stop
- `ModifyOrder` emulated as cancel + create (KuCoin has no native amend)
- `reduceOnly`, `postOnly`, time-in-force, per-order leverage, margin mode
- Order / stop-order queries, fills

**Account (REST)**
- Account overview, balance, positions (all / one), leverage

**WebSocket (bullet-token model)**
- Public: managed level-2 order book (REST seed + sequence reconcile + auto re-seed on gap),
  ticker (last + best bid/ask), trades, klines, mark/index price
- Private: order lifecycle (`tradeOrders`), position changes, wallet/balance

**Engineering**
- `KC-API-*` HMAC-SHA256 signing (key version V1/V2/V3), passphrase encoding
- Allocation-conscious hot path: `json-iterator` decode, `shopspring/decimal` numerics
- Automatic WS reconnect with backoff + jitter and transparent resubscribe
- Uniform `*kucoin.Error` with a typed `ErrorKind` and preserved `KucoinCode`
- Pluggable logger / metrics facades (no-op by default), rate-limit header observers

---

## Install

```bash
go get github.com/tonymontanov/go-kucoin/v2@v2.0.0
```

```go
import (
    kucoin "github.com/tonymontanov/go-kucoin/v2"
    "github.com/tonymontanov/go-kucoin/v2/futures"
)
```

> The `futures` package registers its profile factory in `init()`, so importing it
> (even anonymously) is what makes `client.Futures()` non-nil.

---

## Quick start

### Public market data + streaming (no auth)

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/futures"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

func main() {
	c, err := kucoin.NewClient(kucoin.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	fc := c.Futures().(*futures.Client)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := fc.MarketData().GetContract(ctx, "XBTUSDTM")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s tickSize=%s multiplier=%s\n", info.Symbol, info.TickSize, info.Multiplier)

	// Managed (sequence-reconciled) order book.
	err = fc.Stream().WatchOrderBook(ctx, "XBTUSDTM", func(ob *roottypes.OrderBookSnapshot) {
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			fmt.Printf("seq=%d bid=%s ask=%s\n", ob.Sequence, ob.Bids[0].Price, ob.Asks[0].Price)
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	_ = fc.Stream().Close()
}
```

### Authenticated trading

```go
cfg := kucoin.DefaultConfig()
cfg.APIKey = os.Getenv("KUCOIN_API_KEY")
cfg.SecretKey = os.Getenv("KUCOIN_API_SECRET")
cfg.Passphrase = os.Getenv("KUCOIN_API_PASSPHRASE")
cfg.Demo = os.Getenv("KUCOIN_DEMO") == "1" // sandbox host

c, err := kucoin.NewClient(cfg)
if err != nil {
	log.Fatal(err)
}
defer c.Close()

fc := c.Futures().(*futures.Client)
ctx := context.Background()

ack, err := fc.Trading().PlaceOrder(ctx, futurestypes.CreateOrderRequest{
	Symbol:   "XBTUSDTM",
	Side:     futurestypes.SideBuy,
	Type:     futurestypes.OrderLimit,
	Size:     1,
	Price:    decimal.RequireFromString("10000"),
	Leverage: "5",
	PostOnly: true,
})
if err != nil {
	log.Fatal(err)
}

_, _ = fc.Trading().CancelOrder(ctx, ack.OrderID)
```

Runnable demos live in [`examples/public`](examples/public) and [`examples/private`](examples/private).

---

## Architecture

```
kucoin.Client (root)              shared transport + signing + config
  └─ futures.Client (profile)     layer 2: section-specific REST/WS
       ├─ MarketData()            contracts, klines, orderbook, mark/funding
       ├─ Trading()               place/cancel/batch, queries, fills
       ├─ Account()               balance, positions, leverage
       └─ Stream()                public + private WebSocket
```

- A single neutral core (`internal/*`) handles HTTP transport, the KuCoin
  `{ code, data, msg }` envelope, signing, error mapping and WS plumbing
  (bullet token → dial → ping/pong → topic registry → reconnect).
- Profiles wrap the core and never reuse each other's functions, so adding a
  new section (e.g. spot) does not perturb existing ones.
- Profiles register a factory in `init()`; the root returns `any` and the
  caller casts once (`c.Futures().(*futures.Client)`) to avoid an import cycle.

---

## Configuration

`kucoin.DefaultConfig()` returns sane defaults; override fields as needed:

| Field | Purpose |
| --- | --- |
| `APIKey` / `SecretKey` / `Passphrase` | API credentials (private endpoints) |
| `KeyVersion` | Passphrase encoding: V1 plaintext, V2/V3 HMAC. Default V2 |
| `Demo` | `true` → KuCoin futures sandbox host + sandbox keys |
| `Logger` / `MetricsFactory` | Observability facades (no-op by default) |
| `RateLimitEventObserver` | Receives `gw-ratelimit-*` headers per request |

WebSocket needs no URL: the SDK fetches a bullet token via
`/api/v1/bullet-public` and `/api/v1/bullet-private` on every (re)connect.

---

## Error handling

All errors are `*kucoin.Error` with a typed `Kind` and the original KuCoin code:

```go
_, err := fc.Trading().PlaceOrder(ctx, req)
var ke *kucoin.Error
if errors.As(err, &ke) {
	fmt.Println(ke.Kind, ke.KucoinCode, ke.Message)
}
```

Helpers such as `kucoin.IsRateLimit(err)` / `kucoin.IsAuth(err)` are provided for
common branches.

---

## Status & roadmap

- ✅ **v1.0 — Futures (USD-M perpetuals):** complete, live-validated, published as `v2.0.0`.
- 📋 **v2.0 — Spot:** planned.
- 📋 **v2.5 — remaining sections.**

---

## License

See [LICENSE](LICENSE).
