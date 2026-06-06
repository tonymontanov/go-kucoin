# go-kucoin

High-performance, low-latency Go SDK for the [KuCoin](https://www.kucoin.com/) exchange,
built for HFT / market-making workloads.

- **Module:** `github.com/tonymontanov/go-kucoin/v2`
- **Go:** 1.24+
- **API target:** KuCoin **Classic** API (not the new UTA / unified-account family)
- **Status:** **v1.0 — Futures (USD-M perpetuals)** and **v2.0 — Spot** complete and live-validated (`v2.1.0`). **v2.5** profiles implemented & offline-tested on the `v2.5` branch (live-validation pending): **Phase A — Margin** (HF cross/isolated, `v2.2.0`), **Phase B — Account & Funding** (`v2.3.0`), **Phase C — Earn + VIP Lending** (`v2.4.0`).

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

## Features (Spot v2.0)

Hosted on `api.kucoin.com` (separate REST host from Futures; the profile builds
its own signed transport sharing the root signer + observers).

**Market data (REST)**
- Server time, symbols (`baseIncrement` / `priceIncrement` / min sizes), level-1 ticker, 24h stats
- Order book: public `level2_100` snapshot **and** full-depth signed `level2` (for market-making)
- Klines (spot column order `[t,o,close,h,l,…]`), recent trades

**Trading (REST)**
- Place limit / market orders (market by `size` in base **or** `funds` in quote)
- Batch place (`orders/multi`, one symbol, ≤5 per call), cancel by id / clientOid / all
- GTT / GTC / IOC / FOK time-in-force, post-only, self-trade-prevention (STP)
- Order & fill queries

**Account (REST)**
- Accounts list + balance adapter (trade account)

**WebSocket**
- Public: managed level-2 order book (full-depth or `level2_100` seed +
  multi-change-per-frame sequence reconcile + auto re-seed on gap), ticker, match (trades), candles
- Private: order lifecycle (`tradeOrders`), wallet/balance

---

## Features (Margin v2.5 — Phase A)

> Additive `margin/` profile (on the `v2.5` branch). HF **only** — KuCoin
> retired the legacy LF margin order endpoints in the 2026-03-04 HF migration.
> Margin trades on the spot matching engine, so the live order book / ticker /
> trades are identical to Spot — use the spot profile for those; this profile
> adds the margin-specific surface below.

**Market data (REST)**
- Cross-margin symbols (`/api/v3/margin/symbols`) + isolated-margin pair config
- Mark price (single + all symbols), global margin config

**Trading (REST, HF)**
- Place limit / market (market by `size` or `funds`) + validate-only test order
- `isIsolated` / `autoBorrow` / `autoRepay`; cross (`MARGIN_TRADE`) vs isolated (`MARGIN_ISOLATED_TRADE`)
- Cancel by id / clientOid / all-by-symbol; open / closed / active-symbols / order & fill queries
- GTT / GTC / IOC / FOK, post-only, STP

**Borrow / debit (REST)**
- Borrow, repay, borrow/repay/interest history, v3 leverage update

**Risk limit & account (REST)**
- Cross/isolated risk limit + borrow config (`/api/v3/margin/currencies`)
- Cross (`/api/v3/margin/accounts`) + isolated (`/api/v3/isolated/accounts`) accounts (balances + liabilities + debt ratio)

**WebSocket**
- Private: margin order lifecycle on the spot/margin `tradeOrders` channel (filter by `TradeType`)

_Deferred fast-follows: stop/OCO margin orders, the margin lending market ("Credit")._

---

## Features (Account & Funding v2.5 — Phase B)

> Additive `account/` profile (on the `v2.5` branch) — the cross-cutting
> "treasury" layer on the spot host (`api.kucoin.com`). The futures-host account
> endpoints (account overview, transaction history, futures transfers) stay in
> the `futures/` profile and are not duplicated here.

**Account (REST)**
- Account summary (`/api/v2/user-info`), API-key info (`/api/v1/user/api-key`)
- Spot wallet list / detail (`/api/v1/accounts`), spot/margin ledgers (paged)

**Deposit (REST)**
- Create + list v3 deposit addresses, deposit history

**Withdrawal (REST)**
- Quotas, submit (v3 `/api/v3/withdrawals`), cancel, history + by-id

**Transfer (REST)**
- Transferable balance, v3 flex/universal transfer between wallets and master/sub (plus an `InnerTransfer` convenience)

**Fee & currencies (REST)**
- Account base spot/margin fee, actual per-symbol trade fees (≤10 symbols)
- Public v3 currency directory (all + one): chains, precisions, withdraw/deposit minimums

_Deferred fast-follows: sub-account management, legacy V1/V2 deposit-address & transfer endpoints, HF/futures ledgers._

---

## Features (Earn + VIP Lending v2.5 — Phase C)

> Two additive profiles on the spot host (`api.kucoin.com`), kept separate to
> match KuCoin's service taxonomy. All endpoints are private (signed).

**Earn (`earn/`)**
- Product catalogues: savings, promotion, staking, KCS-staking, ETH-staking
- Subscribe (`Purchase`) / redeem (`Redeem` + `RedeemPreview` with early-redemption penalty)
- Current holdings (`GetHoldings`, paged)

**VIP Lending / OTC loan (`viplending/`, read-only)**
- Collateral / discount-rate configs (gradient tiers per currency)
- Consolidated loan info: orders, LTV thresholds, collateral legs
- Participating OTC-lending accounts

_Deferred: Structured Earn (dual investment) — KuCoin reports those endpoints as not generally available._

---

## Install

```bash
go get github.com/tonymontanov/go-kucoin/v2@v2.1.0
```

```go
import (
    kucoin "github.com/tonymontanov/go-kucoin/v2"
    "github.com/tonymontanov/go-kucoin/v2/futures" // or .../spot
)
```

> Each profile package (`futures`, `spot`) registers its factory in `init()`, so
> importing it (even anonymously) is what makes `client.Futures()` / `client.Spot()` non-nil.

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
  ├─ futures.Client (profile)     layer 2: api-futures.kucoin.com
  │    ├─ MarketData()            contracts, klines, orderbook, mark/funding
  │    ├─ Trading()               place/cancel/batch, queries, fills
  │    ├─ Account()               balance, positions, leverage
  │    └─ Stream()                public + private WebSocket
  ├─ spot.Client (profile)        layer 2: api.kucoin.com (own signed REST)
  │    ├─ MarketData()            symbols, klines, orderbook (incl. full depth)
  │    ├─ Trading()               place/cancel/batch (size or funds), queries, fills
  │    ├─ Account()               accounts + balance
  │    └─ Stream()                public + private WebSocket
  ├─ margin.Client (profile)      layer 2: api.kucoin.com, HF margin (v2.5)
  │    ├─ MarketData()            cross/isolated symbols, mark price, margin config
  │    ├─ Trading()               HF place/cancel (cross/isolated), queries, fills
  │    ├─ Borrow()                borrow/repay (+ histories), interest, leverage
  │    ├─ Account()               cross/isolated accounts (balances + liabilities)
  │    ├─ RiskLimit()             cross/isolated risk limit + borrow config
  │    └─ Stream()                private margin order WS (book/ticker via spot)
  ├─ account.Client (profile)     layer 2: api.kucoin.com, treasury (v2.5)
  │    ├─ Account()               summary, api-key, wallets, ledgers
  │    ├─ Deposit()               v3 addresses + history
  │    ├─ Withdrawal()            quotas, v3 withdraw, cancel, history
  │    ├─ Transfer()              transferable + v3 flex transfer
  │    ├─ Fee()                   base + actual trade fees
  │    └─ Currency()              v3 currency directory (chains/precisions)
  ├─ earn.Client (profile)        layer 2: api.kucoin.com, Earn (v2.5)
  │    ├─ Get*Products()          savings/promotion/staking/kcs/eth
  │    ├─ Purchase()/Redeem()     subscribe / redeem (+ preview)
  │    └─ GetHoldings()           current Earn positions
  └─ viplending.Client (profile)  layer 2: api.kucoin.com, OTC loan (v2.5)
       ├─ GetCollateralConfigs()  gradient discount rates
       ├─ GetLoanInfo()           orders + LTV + collateral
       └─ GetAccounts()           participating accounts
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
- ✅ **v2.0 — Spot:** complete, live-validated, published as `v2.1.0`.
- 🔄 **v2.5 — remaining sections** (additive `v2.5` branch):
  - **Phase A — Margin** (HF cross/isolated): implemented & offline-tested → `v2.2.0`.
  - **Phase B — Account & Funding:** implemented & offline-tested; live-validation pending → `v2.3.0`.
  - **Phase C — Earn + VIP Lending:** implemented & offline-tested; live-validation pending → `v2.4.0`.

---

## License

See [LICENSE](LICENSE).
