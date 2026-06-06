# go-kucoin — Handoff

> Context-bridge document. Updated after every significant change
> (architecture, new module, refactor). Read this first when resuming.

---

## 1. Role & Stack

High-performance Go SDK for the **KuCoin** exchange, built for an HFT /
low-latency trading desk. It mirrors the architecture of the in-house
`go-okx` / `go-bybit` / `go-bitget` SDKs (two-layer design, lazy domain
sub-clients, neutral transport core).

- **Language:** Go 1.24, module `github.com/tonymontanov/go-kucoin/v2`.
- **API target:** KuCoin **CLASSIC** API (NOT the new UTA / unified
  account family). Private REST is signed with `KC-API-*` headers +
  HMAC-SHA256; WS uses the **bullet-token** connection model.
- **Dependencies:**
  - `github.com/gorilla/websocket` — WS transport.
  - `github.com/json-iterator/go` — fast JSON (hot path).
  - `github.com/shopspring/decimal` — exact numerics for prices/qty.

---

## 2. Architecture

### Two-layer principle (no parallel copy-paste)

A single neutral core (`internal/*`) does transport, signing, error
mapping, WS plumbing. Each trading section (profile) wraps the core with
section-specific specifics — profiles never reuse each other's functions.
Naming matches KuCoin's own section names (USD-M perpetuals = "Futures" →
package `futures`).

### Folder structure

```
go-kucoin/
├── client.go              # root kucoin.Client + lazy sub-client factories
├── config.go              # public Config, endpoints, KeyVersion, defaults
├── errors.go              # re-export of internal/kcerr (Error, Is*, Map*)
├── logger.go              # re-export of internal/kclog (Logger, fields)
├── metrics.go             # re-export of internal/kcmet (Counter, factory)
├── rate-limit-event.go    # public RateLimitEvent + RateLimitCategory
├── doc.go                 # package overview
├── internal/
│   ├── auth/              # KC-API-* signing, passphrase enc, key version
│   ├── codec/             # jsoniter wrapper + numeric parse helpers
│   ├── kcerr/             # Error type + HTTP/KuCoin code → ErrorKind
│   ├── kclog/             # logging facade (Noop default)
│   ├── kcmet/             # metrics facade (Noop default)
│   ├── rest/              # low-level REST client (envelope, headers, meta)
│   ├── ws/                # bullet-token connect/reconnect/ping
│   └── kccommon/          # shared helpers + seq orderbook engine
├── types/                 # layer-1 protocol-common types
├── futures/               # layer-2 Futures profile (v1.0)
│   └── types/             # futures-specific + layer-1 aliases
├── examples/              # runnable demos (public / private)
├── README.md              # public overview + quick start
└── docs/                  # source ToR (TS-SINGLE-EXCHANGE-SDK*.md)
```

### Key modules & interaction

```
futures.Client (profile)            <- layer 2 (section specifics)
  └─ uses kucoin.Client.REST()/WS() <- shared transport
       ├─ internal/rest  (HTTP + KuCoin { code, data, msg } envelope)
       ├─ internal/ws    (bullet token → dial → ping/pong → subscribe)
       ├─ internal/auth  (KC-API-SIGN, passphrase by key version)
       └─ internal/kcerr (uniform *Error with ErrorKind)
```

- Root package re-exports facade types (`Logger`, `Counter`, `Error`) via
  `type X = internal.X` aliases so profile packages import the root only,
  and the transport packages avoid an import cycle.
- Profiles register a factory in `init()` (`RegisterFuturesFactory`); the
  root returns `any` and the caller casts (`c.Futures().(*futures.Client)`).

### KuCoin specifics already handled

- **Per-section REST hosts:** Futures = `api-futures.kucoin.com`,
  Spot = `api.kucoin.com` (v2.0). v1.0 defaults to the futures host; the
  root transport stays section-agnostic, so a future spot profile builds
  its own REST client against the spot host.
- **Demo / sandbox:** `Config.Demo = true` selects
  `api-sandbox-futures.kucoin.com` automatically.
- **Key version:** `Config.KeyVersion` (V1 plaintext passphrase; V2/V3
  HMAC-encoded). Default V2.
- **Signature pre-hash:** `timestamp + method + requestPath(+query) + body`
  on the exact bytes sent on the wire (no re-marshal).
- **WS bullet token:** no fixed WS URL; `BulletPublicPath` /
  `BulletPrivatePath` REST calls return the endpoint + ping interval.
- **Orderbook (Futures):** sequence-number reconciliation (NOT CRC32).

---

## 3. Roadmap

Phasing: **v1.0** = Futures (USD-M perpetuals) · **v2.0** = Spot ·
**v2.5** = remaining sections.

> **Milestone — v1.0 Futures MVP COMPLETE & PUBLISHED.** The SDK was
> live-validated end-to-end against KuCoin (public + private + trade + WS) on
> `PARTIUSDTM` driving the `market-making-desk-core` desk (Frontrun Chase /
> CQB Scale strategies). All transport, REST, account, trading and public +
> private WS paths exercised in production. Committed and pushed to `main`,
> tagged **`v2.0.0`** (module path `.../v2`). README added. Next iteration:
> v2.0 Spot.

### ✅ Done

- `go.mod` (module `.../v2`, Go 1.24) + `go.sum`.
- `internal/codec` — jsoniter wrapper, `RawJSON`, numeric parse helpers.
- `internal/kclog` — logging facade + Noop.
- `internal/kcmet` — metrics facade + Noop.
- `internal/kcerr` — `Error`, `ErrorKind`, HTTP + KuCoin code mapping.
- `internal/auth` — KC-API-* signing, passphrase encoding by key version,
  ms timestamp helper.
- `internal/rest` — low-level REST client: URL build, signing, envelope
  parse, error classification, rate-limit header collection + observers.
- **Root package scaffold** — `errors.go`, `logger.go`, `metrics.go`,
  `rate-limit-event.go`, `config.go` (endpoints/defaults/KeyVersion/Demo),
  `client.go` (root client + Futures/Spot lazy factories), `doc.go`.
- `internal/ws` — bullet-token transport: `TokenProvider` per (re)connect,
  dial `endpoint?token=&connectId=`, await `welcome`, JSON ping/pong on the
  server-driven interval, topic-keyed subscription registry,
  reconnect (backoff+jitter) with transparent resubscribe + `Reset` hook.
  Callback-based `Subscription.Handler` API. Covered by `protocol_test.go`
  + `conn_test.go` (mock WS server, race-clean).
- `internal/kccommon` — numeric parse helpers + sequence-based orderbook
  engine (`orderbook/engine.go`): REST-snapshot seeding, contiguous apply,
  stale drop, gap → `ErrGap` (resync), level2 "price,side,size" change
  parser. Covered by `engine_test.go` (race-clean).
- `types/` — full layer-1: `OrderBookLevel`/`OrderBookSnapshot` (sequence),
  `SideType`/`OrderType`/`TimeInForceType`/`OrderStatus`/`MarginMode`/
  `StopType`/`StopPriceType`, `Timeframe` (minute-based `Wire()`), `Candle`/
  `Candles`, `TradeUpdate`, `KlineUpdate`, `Balance`/`CoinBalance`,
  `CancelOrderRequest`.
- `futures/` — **layer-2 Futures profile (v1.0 complete)**:
  - `client.go` — profile client + sub-client factories + `init()` factory
    registration; optional default leverage / margin mode.
  - `helpers.go` — REST GET/POST/DELETE wrappers, clientOid gen, validation
    + auth error constructors, ns→ms.
  - `market.go` — MarketData: server time, contracts (all/one), ticker,
    level2 snapshot, klines, mark price, funding rate, recent trades.
  - `trading.go` — Trading: place (limit/market/stop), batch place,
    cancel (id / clientOid / all / all-stop), order & stop-order queries,
    fills + recent fills. Per-order leverage model (no classic set-leverage
    endpoint).
  - `account.go` — Account: overview, Balance adapter, positions (all/one).
  - `stream.go` + `stream-wire.go` — public WS: managed level2 order book
    (REST seed + sequence reconcile + auto re-seed on gap), ticker, trades,
    klines (Confirmed at rollover); public/private bullet TokenProviders.
  - `stream-private.go` — private WS: orders, positions, balance.
  - `futures/types/*` — SymbolInfo, MarketTicker/MarkPrice/FundingRate,
    OrderInfo, PositionInfo, Fill, AccountOverview, CreateOrderRequest,
    BatchOrderResult + layer-1 alias re-exports + futures enums.
  - Tests: `trading_test.go` (body builder validation/defaults),
    `market_test.go` (wire decode / ns→ms / kline rows). Race-clean.
- `go build ./...`, `go vet ./...`, `go test ./... -race` green.

- `examples/` — `examples/public` (REST contract + managed order-book and
  trade stream) and `examples/private` (balance → place → query → cancel).
- **Contract tests (offline, recorded fixtures):**
  - `contract_rest_test.go` — mock HTTP server serving recorded KuCoin
    envelopes; drives MarketData / Trading / Account end-to-end (URL/path,
    envelope parse, signed fail-fast, wire→type converters, body assembly).
  - `contract_ws_test.go` — single mock server playing bullet + REST
    snapshot + WS upgrade; verifies welcome/subscribe handshake and
    dispatch for ticker / trades / klines and the managed order book
    (REST seed + sequence apply).
- `go build ./...`, `go vet ./...`, `go test ./... -race` green.

- **Repo B (market-making-desk-core) — `kucoin/futures` connector (DONE,
  live-validated):** `kucoin-connector` branch off `qa`. Wraps this SDK with
  minimal, localised changes to the stable core. Live-hardened fixes shipped:
  - KuCoin Futures rate-limiter strategy driven by `gw-ratelimit-*` headers
    (fixes the 429000 user-level ban seen on first live run).
  - Margin-mode resolution + cache (fixes order reject `330005`).
  - Price alignment to instrument `tickSize` (fixes `100001`).
  - Real-time inventory: WS `position.change` used as trigger +
    account-wide order-stream fill detection → debounced REST refresh
    (KuCoin position WS frames are sparse and often omit `currentQty`).
  - Parallel batch modify (cancel-legs in parallel + single `orders/multi`
    place) — removes the per-order lag / "overshoot to opposite side".
  - Post-only (`GTX`) propagated on batch modify (was dropped → taker fills).

### 🔧 In progress

- **v2.0 — Spot profile** (next iteration): `spot/` package against
  `api.kucoin.com`, registered via `RegisterSpotFactory`.

### ✅ Reconciled against live API (v1.0)
Wire field names below were taken from KuCoin docs + official SDKs and have
been reconciled against live/sandbox responses during the v1.0 validation:
  - private WS payloads (`tradeOrders` / `position` / `wallet`) field names;
  - `privateChannel` flag necessity per private topic;
  - level2 REST snapshot `ts` unit (ms);
  - contract `maxLeverage` presence on `/contracts/active`.

  Live nuance captured: the `/contract/position` WS topic emits sparse,
  mark-price-driven `position.change` frames that OMIT `currentQty`. The SDK
  surfaces this via `PositionInfo.CurrentQtyKnown` so consumers don't read a
  mark tick as a flat position (see `futures/types/position-info.go`).

---

## 4. Rules & Code Style

- **Language:** all SDK code, comments and docs in **English** (public
  project). Chat/PM communication with the maintainer in Russian.
- **Naming:** `camelCase`; section packages match KuCoin's own section
  naming (`futures`, `spot`, …).
- **Explicitness:** explicit variable/const declarations (`var x T = ...`),
  GoDoc comment on every exported symbol.
- **Performance:** minimise allocations on the hot path (WS dispatch, REST
  envelope decode); reuse buffers / `sync.Pool` where it pays off; jsoniter
  over `encoding/json`; `decimal.Decimal` for trading numerics.
- **No cross-profile reuse:** profiles wrap the shared core, never each
  other.
- **Errors:** always `*kucoin.Error` with a correct `ErrorKind`; preserve
  `KucoinCode`.

---

## 5. Integration Secrets (paths / env — NO real keys)

- **Credentials (env, suggested):** `KUCOIN_API_KEY`, `KUCOIN_API_SECRET`,
  `KUCOIN_API_PASSPHRASE`, `KUCOIN_KEY_VERSION` (1|2|3, default 2).
- **Endpoints:**
  - Futures REST (prod): `https://api-futures.kucoin.com`
  - Futures REST (sandbox/demo): `https://api-sandbox-futures.kucoin.com`
  - Spot REST (prod, v2.0): `https://api.kucoin.com`
  - WS: no fixed URL — obtained via bullet token
    (`/api/v1/bullet-public`, `/api/v1/bullet-private`).
- **Demo:** `Config.Demo = true` → sandbox host + sandbox keys.
- Never log secret/passphrase or the signature pre-hash. `auth.Signer`
  redacts on `String()`.

---

## 6. Source ToR

`docs/TS-SINGLE-EXCHANGE-SDK.md` (+ `-RU`) — single-exchange SDK technical
requirements used as the build spec.
