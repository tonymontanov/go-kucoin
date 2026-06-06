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
├── spot/                  # layer-2 Spot profile (v2.0)
│   └── types/             # spot-specific + layer-1 aliases
├── margin/                # layer-2 Margin profile (v2.5, HF cross/isolated)
│   └── types/             # margin-specific + layer-1 aliases
├── account/               # layer-2 Account & Funding profile (v2.5 Phase B)
│   └── types/             # account/funding-specific + layer-1 aliases
├── earn/                  # layer-2 Earn profile (v2.5 Phase C)
│   └── types/             # earn-specific types
├── viplending/            # layer-2 VIP Lending / OTC-loan profile (v2.5 Phase C)
│   └── types/             # OTC-loan types
├── subaccount/            # layer-2 Sub-Account management profile (v2.5 Phase D)
│   └── types/             # sub-account + sub-API-key types
├── convert/               # layer-2 Convert profile (v2.5 Phase E)
│   └── types/             # convert symbol/currency/quote/order types
├── affiliate/             # layer-2 Affiliate profile (v2.5 Phase F, spot host)
│   └── types/             # commission + rebate types
├── copytrading/           # layer-2 futures Copy-Trading profile (v2.5 Phase F)
│   └── types/             # copy-trade order/position/margin types
├── examples/              # runnable demos (public / private / spot-*)
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
**v2.5** = remaining sections (Phase A Margin · B Account/Funding · C Earn).

> **Milestone — v1.0 Futures MVP COMPLETE & PUBLISHED.** The SDK was
> live-validated end-to-end against KuCoin (public + private + trade + WS) on
> `PARTIUSDTM` driving the `market-making-desk-core` desk (Frontrun Chase /
> CQB Scale strategies). All transport, REST, account, trading and public +
> private WS paths exercised in production. Committed and pushed to `main`,
> tagged **`v2.0.0`** (module path `.../v2`). README added. Next iteration:
> v2.0 Spot.

> **Milestone — v2.0 Spot COMPLETE & LIVE-VALIDATED.** The `spot/` profile
> mirrors `futures/` on api.kucoin.com: MarketData (symbols / level1 ticker /
> 24h stats / level2_100 + full-depth signed level2 / klines / trades / time),
> Trading (place limit+market by size OR funds, batch place same-symbol,
> cancel by id/clientOid/all, order queries, fills), Account (accounts +
> Balance adapter) and Stream (public level2/ticker/match/candles + private
> tradeOrders/balance). Live-validated end-to-end on `PARTIUSDT` driving the
> `market-making-desk-core` desk (Frontrun Chase / CQB Scale strategies):
> orders place / modify / cancel, real-time balance-driven inventory, non-zero
> best bid/ask from start. Live-hardening fixes shipped (full-depth signed
> level2 seed; tolerant `orders/multi` decode that handles the nested
> `{"data":[...]}` envelope — fixes the "uncontrolled re-place" flood; flexible
> int64 for the QUOTED `time` on the `/account/balance` private push — fixes
> the dropped inventory WS). Build / vet / race green; offline contract + unit
> tests added. Published as **`v2.1.0`**.

> **Milestone — v2.5 Phase A (Margin) IMPLEMENTED (SDK-only, offline-tested).**
> New ADDITIVE `margin/` profile on `v2.5` branch — zero changes to the
> `futures/`/`spot/` profiles or shared `internal/` public surface, so the
> stable desk integration cannot regress. Targets the **HF** margin family
> exclusively: KuCoin completed the LF→HF margin migration on **2026-03-04**
> and retired the legacy `/api/v1/margin/order` LF order endpoints, so the
> earlier "HF + Classic LF" plan is moot — only HF remains (a deprecated
> `/api/v1/margin/order` "Add Order - V1" shim with no cancel/query counterpart
> is intentionally NOT wrapped). Coverage: MarketData (cross/isolated symbols,
> mark price single+all, margin config), Trading (HF place + test, cancel by
> id/clientOid/all-by-symbol, open/closed/active-symbols queries, fills —
> `tradeType` MARGIN_TRADE/MARGIN_ISOLATED_TRADE, `isIsolated`/`autoBorrow`/
> `autoRepay`), Borrow (borrow/repay + histories, interest history, v3 leverage
> update), RiskLimit (cross/isolated `/api/v3/margin/currencies`), Account
> (cross `/api/v3/margin/accounts` + isolated `/api/v3/isolated/accounts`,
> balances+liabilities+debt ratio), Stream (PRIVATE margin order updates on the
> spot/margin `/spotMarket/tradeOrders` channel). DESIGN: margin trades on the
> spot matching engine, so the PUBLIC order book / ticker / trades are
> IDENTICAL to Spot — the margin profile does NOT duplicate them (use the spot
> profile), avoiding copy-paste per the two-layer rule. Endpoints/shapes
> verified against KuCoin docs + the official `kucoin-universal-sdk` spec CSV.
> Build / vet / race green; offline contract + unit tests added. NOT yet
> live-validated; stop/OCO margin orders and the margin lending market
> ("Credit": project/purchase/redeem) are deferred fast-follows. Published as
> **`v2.2.0`**.

> **Milestone — v2.5 Phase B (Account & Funding) IMPLEMENTED (SDK-only,
> offline-tested).** New ADDITIVE `account/` profile on `v2.5` — zero changes to
> `futures/`/`spot/`/`margin/` or shared `internal/` public surface. This is the
> cross-cutting "treasury" layer on the SPOT host (`api.kucoin.com`, resolved via
> `kucoin.SpotFamilyBaseURL`). Coverage: Account (summary `/api/v2/user-info`,
> API-key info `/api/v1/user/api-key`, spot wallet list/detail `/api/v1/accounts`
> [+`/{id}`], spot/margin ledgers `/api/v1/accounts/ledgers`), Deposit (create +
> list v3 addresses `/api/v3/deposit-address/create`, `/api/v3/deposit-addresses`,
> history `/api/v1/deposits`), Withdrawal (quotas, v3 submit `/api/v3/withdrawals`,
> cancel, history + by-id), Transfer (transferable `/api/v1/accounts/transferable`
> + v3 flex/universal transfer `/api/v3/accounts/universal-transfer`, with an
> `InnerTransfer` convenience), Fee (base `/api/v1/base-fee`, actual per-symbol
> `/api/v1/trade-fees`), Currency (public v3 directory `/api/v3/currencies`
> [+`/{currency}`] — chains/precisions/withdraw minimums). DESIGN: the SPOT-host
> account/funding surface only — the FUTURES-host account endpoints
> (`account-overview`, `transaction-history`, `transfer-in/out`) stay in the
> `futures/` profile and are intentionally NOT duplicated. Nullable wire decimals
> (`depositMinSize`/`maxWithdraw`/…) decode to zero. Endpoints/shapes verified
> against KuCoin docs + the official `kucoin-universal-sdk` spec CSV. Build / vet /
> race green; offline contract tests added. NOT yet live-validated; sub-account
> management (create/permissions/API-key CRUD/per-sub balances), legacy V1/V2
> deposit-address & transfer endpoints, and HF/futures ledgers are deferred.
> Published as **`v2.3.0`**.

> **Milestone — v2.5 Phase C (Earn + VIP Lending) IMPLEMENTED (SDK-only,
> offline-tested).** Two new ADDITIVE profiles on `v2.5`, both on the SPOT host
> (`api.kucoin.com`) — zero changes to existing profiles or shared `internal/*`.
> `earn/`: product catalogues (savings / promotion / staking / KCS-staking /
> ETH-staking — all share one row shape), Purchase (`POST /api/v1/earn/orders`),
> Redeem (`DELETE /api/v1/earn/orders`) + redeem-preview, and holdings
> (`/api/v1/earn/hold-assets`, paged); the small cohesive surface lives directly
> on the profile client (no sub-clients). `viplending/`: read-only OTC-loan
> queries — collateral/discount-rate configs (`/api/v1/otc-loan/discount-rate-
> configs`), consolidated loan position with LTV thresholds + collateral legs
> (`/api/v1/otc-loan/loan`), and participating accounts (`/api/v1/otc-loan/
> accounts`). Kept as a SEPARATE profile from `earn/` to match the exchange's
> service taxonomy (`earn` vs `viplending`). Nullable wire ints (product
> `lockEndTime`/`applyEndTime`) decode to zero. Shapes verified against KuCoin
> docs + the official Go SDK / spec. Build / vet / race green; offline contract
> tests added. NOT yet live-validated; Structured Earn (dual investment) is
> deferred (KuCoin reports those endpoints as not generally available, 400100).
> Published as **`v2.4.0`**.

> **Milestone — v2.5 Phase D (Sub-Account management) IMPLEMENTED (SDK-only,
> offline-tested).** One new ADDITIVE profile `subaccount/` on `v2.5`, on the
> SPOT host (`api.kucoin.com`) — zero changes to existing profiles or shared
> `internal/*`. MASTER-account-only operations: create a sub-account (`POST
> /api/v2/sub/user/created`) + grant margin/futures permission (`POST
> /api/v3/sub/user/{margin,futures}/enable`); list sub-account summaries (`GET
> /api/v2/sub/user`, paged) and spot balances — single (`GET
> /api/v1/sub-accounts/{id}`) + paged (`GET /api/v2/sub-accounts`); and the spot
> sub-account API-key lifecycle: create / list / modify / delete
> (`/api/v1/sub/api-key[/update]`). CreateAPIKey surfaces the API secret +
> passphrase ONCE (documented). A flexInt64 tolerates `createdAt` arriving as a
> bare number OR a quoted string (KuCoin is inconsistent across these
> endpoints). The futures sub-account balance endpoint
> (`/api/v1/account-overview-all`, FUTURES host) and the deprecated V1
> summary/balance endpoints are intentionally excluded to keep the profile on
> the spot host. Small cohesive surface → flat client (no sub-clients). Shapes
> verified against the official KuCoin Go SDK. Build / vet / race green; offline
> contract tests added (incl. flexInt64 number-vs-string). NOT yet
> live-validated. Published as **`v2.5.0`**.

> **Milestone — v2.5 Phase E (Convert) IMPLEMENTED (SDK-only, offline-tested).**
> One new ADDITIVE profile `convert/` on `v2.5`, on the SPOT host
> (`api.kucoin.com`) — zero changes to existing profiles or shared `internal/*`.
> KuCoin Convert is a fee-free currency swap (the quoted price embeds a spread).
> Coverage: public directories — convertible pair limits (`GET
> /api/v1/convert/symbol`) and currency list (`GET /api/v1/convert/currencies`);
> market convert — quote (`GET /api/v1/convert/quote`), place (`POST
> /api/v1/convert/order`), detail (`/order/detail`), history (`/order/history`,
> paged); limit convert — protection-price quote (`/limit/quote`), place (`POST
> /api/v1/convert/limit/order`), detail (`/limit/order/detail`), list
> (`/limit/orders`, paged), cancel (`DELETE /api/v1/convert/limit/order/cancel`,
> by clientOrderId). A flexStr normalises `orderId` (quoted string for limit
> orders, bare number for market detail). Nullable limit-order timestamps
> (`cancelTime`/`filledTime`) and `cancelType` decode to 0. Small cohesive
> surface → flat client (no sub-clients); GetSymbol/GetCurrencies are public
> (unsigned), the rest signed. Shapes verified against KuCoin docs. Build / vet /
> race green; offline contract tests added. NOT yet live-validated. Published as
> **`v2.6.0`**.

> **Milestone — v2.5 Phase F (Affiliate + Copy-Trading) IMPLEMENTED (SDK-only,
> offline-tested).** Two new ADDITIVE profiles on `v2.5` — zero changes to
> existing profiles or shared `internal/*`.
> `affiliate/` (SPOT host): read-only reports — GetCommission (`GET
> /api/v2/affiliate/queryMyCommission`) and GetInviterRebate (`GET
> /api/v2/affiliate/inviter/statistics`, the DEPRECATED "Get Account" rebate
> endpoint, kept for completeness). KuCoin's newer affiliate reports (Get
> Transaction / Get Invited / Get Trade History) are deferred fast-follows.
> `copytrading/` (FUTURES host, via the parent's futures-bound REST — same as the
> futures profile): lead-trader futures copy-trading under
> `/api/v1/copy-trade/futures/*` — PlaceOrder / PlaceOrderTest, PlaceTPSLOrder
> (st-orders), CancelOrder (by orderId → cancelledOrderIds), CancelOrderByClientOid
> (clientOid+symbol), GetMaxOpenSize, GetMaxWithdrawMargin (bare string),
> AddIsolatedMargin (deposit-margin → full Position), RemoveIsolatedMargin
> (withdraw-margin → string), ModifyRiskLimitLevel (→ bool), SetAutoDepositStatus
> (→ bool). Requires the LeadtradeFutures permission (copy-trading account), so
> offline-tested only. Copy-trading supports ISOLATED margin only (CROSS →
> 180204), max 20x, hedge-mode PositionSide (LONG/SHORT/BOTH). Shapes verified
> against KuCoin docs + the official-pattern KuCoin SDK. Build / vet / race green;
> offline contract tests added. NOT yet live-validated. To be tagged **`v2.7.0`**.

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

- `spot/` — **layer-2 Spot profile (v2.0 COMPLETE & live-validated)**:
  - `client.go` — profile client + sub-client factories + `init()` factory
    registration. Builds its OWN REST client on the spot host via
    `kucoin.Client.NewSectionRESTClient` (root REST defaults to the futures
    host); `resolveSpotBaseURL` maps futures-host/empty → spot host and
    honours any explicit non-futures URL (mock / override). Optional default
    trade type (TRADE/MARGIN_TRADE).
  - `helpers.go` — REST GET/POST/DELETE wrappers, clientOid gen, spot kline
    granularity map ("1min"/"1hour"/…), ns→ms.
  - `market.go` — MarketData: server time, symbols (all/one), level1 ticker,
    24h stats, level2_100 snapshot (public) + **GetOrderBookFull** (signed
    `/api/v3/market/orderbook/level2`, full depth for market-making), klines
    (sec window, [t,o,CLOSE,h,l,…] column order), recent trades.
  - `trading.go` — Trading: place (limit/market by size OR funds), batch
    place (one symbol, `orders/multi` {symbol,orderList}), cancel
    (id / clientOid via `/order/client-order/` / all), order & fill queries.
  - `account.go` — Account: accounts list + Balance adapter (trade account).
  - `stream.go` + `stream-wire.go` — public WS: managed level2 (REST seed —
    FULL depth when keyed, else level2_100 — + multi-change-per-frame,
    per-entry sequences sorted then applied through the shared engine +
    re-seed on gap), ticker, match (trades), candles.
  - `stream-private.go` — private WS: tradeOrders, balance.
  - `spot/types/*` — SymbolInfo, MarketTicker/MarketStats, OrderInfo,
    CreateOrderRequest, Fill, BatchOrderResult, AccountInfo + layer-1 alias
    re-exports + spot enums (GTT/FOK, STP, TradeType, AccountType).
  - Tests: `trading_test.go` (body builder: limit vs market size/funds +
    `decodeBatchRows` nested/bare/empty), `market_test.go` (spot candle column
    order, level2 change sort, granularity), `contract_rest_test.go` (mock REST
    end-to-end incl. full-depth signed level2), `contract_ws_test.go` (mock WS
    bullet + multi-change-per-frame reconcile), `stream_private_test.go`
    (`flexInt64` quoted/bare time). Race-clean.
  - Config: `DefaultSpotRestBaseURL` (+ sandbox) added; root
    `NewSectionRESTClient` shares the signer + rate-limit observers.
  - Live-hardening fixes (from desk validation on `PARTIUSDT`):
    - **Full-depth signed level2** seed (`GetOrderBookFull` via
      `/api/v3/market/orderbook/level2`) — `level2_100` is too shallow for MM;
      WS book manager prefers it when keyed.
    - **Tolerant `orders/multi` decode** (`decodeBatchRows`): KuCoin nests the
      batch rows under `{"data":[...]}`; the old bare-array decode failed on a
      200 OK, so the strategy retried and FLOODED the book — now accepts both
      nested and bare shapes.
    - **`flexInt64`** for the `/account/balance` private push `time` (delivered
      as a QUOTED string) — the int64 field silently dropped EVERY balance
      frame, which looked like "inventory WS not working" (only the 60s REST
      poll moved the position).

- `margin/` — **layer-2 Margin profile (v2.5 Phase A, HF cross/isolated)**:
  - `doc.go` — profile overview (HF-only rationale, scope, spot-shared public
    market data, deferred stop/OCO + lending).
  - `client.go` — profile client + sub-client factories (`MarketData`,
    `Trading`, `Borrow`, `Account`, `RiskLimit`, `Stream`) + `init()` factory
    registration; default trade type (cross/isolated). Builds its own REST
    client on the spot host via `kucoin.SpotFamilyBaseURL`.
  - `helpers.go` — REST GET/POST/DELETE wrappers, clientOid gen (`kcm-…`),
    validation + auth error constructors, ns→ms.
  - `market.go` — cross/isolated symbols, mark price (single + all), margin
    config.
  - `trading.go` — HF place + test, cancel (id / clientOid / all-by-symbol),
    open / closed / active-symbols / order-by-id / by-clientOid / fills.
    Body carries `isIsolated`/`autoBorrow`/`autoRepay`; queries require
    `symbol`+`tradeType`.
  - `borrow.go` — borrow / repay (+ histories), interest history, v3 leverage
    update.
  - `account.go` — cross + isolated account snapshots (balances + liabilities
    + debt ratio).
  - `risk-limit.go` — cross/isolated risk limit + borrow config.
  - `stream.go` — private bullet WS + `WatchOrders` (`/spotMarket/tradeOrders`,
    margin `tradeType`); `flexInt64` for bare/quoted timestamps.
  - `margin/types/*` — CreateOrderRequest, OrderInfo, Fill, SymbolInfo +
    IsolatedSymbol, MarkPrice/MarginConfig, Cross/IsolatedMarginAccount,
    Borrow/Repay/Debit/Interest, Cross/IsolatedRiskLimit + layer-1 alias
    re-exports + margin enums (TradeType cross/isolated, QueryType, STP,
    GTT/FOK, BorrowIOC/FOK).
  - Tests: `trading_test.go` (body builder + isIsolated derivation),
    `contract_rest_test.go` (mock REST end-to-end across market/orders/account/
    borrow/risk-limit), `stream_private_test.go` (`flexInt64` bare/quoted).
    Race-clean.
  - Root wiring (additive): `RegisterMarginFactory` + `Client.Margin()` in
    `client.go`; exported `SpotFamilyBaseURL` host resolver in `config.go`
    (shared by spot-family section profiles).

- `account/` — **layer-2 Account & Funding profile (v2.5 Phase B)**:
  - `doc.go` — profile overview (cross-cutting treasury, spot-host scope,
    futures-host endpoints stay in `futures/`, deferred sub-account/V1/V2).
  - `client.go` — profile client + sub-client factories (`Account`, `Deposit`,
    `Withdrawal`, `Transfer`, `Fee`, `Currency`) + `init()` factory
    registration. Builds its own REST client on the spot host via
    `kucoin.SpotFamilyBaseURL`.
  - `helpers.go` — REST GET/POST/DELETE wrappers, clientOid gen (`kca-…`),
    validation + auth error constructors, int/int64 fmt.
  - `account.go` — summary (`/api/v2/user-info`), API-key info, spot wallet
    list/detail, spot/margin ledgers (paged).
  - `deposit.go` — create + list v3 deposit addresses, deposit history.
  - `withdrawal.go` — quotas, v3 submit, cancel, history + by-id.
  - `transfer.go` — transferable balance, v3 flex/universal transfer (+
    `InnerTransfer` convenience).
  - `fee.go` — account base spot/margin fee, actual per-symbol trade fees.
  - `currency.go` — public v3 currency directory (all + one), chains/precisions.
  - `account/types/*` — AccountSummary/ApiKeyInfo/AccountInfo/Ledger(+Query),
    DepositAddress/Record(+Query), WithdrawalQuota/Request/Record(+Query),
    TransferableBalance/FlexTransferRequest/Result, BaseFee/TradeFee,
    Currency/Chain + enums (AccountType, TransferType, WithdrawType,
    LedgerDirection).
  - Tests: `contract_rest_test.go` (mock REST end-to-end across account/deposit/
    withdrawal/transfer/fee/currency; null-decimal + auth-required cases).
    Race-clean.
  - Root wiring (additive): `RegisterAccountFactory` + `Client.Account()` in
    `client.go`.

- `earn/` — **layer-2 Earn profile (v2.5 Phase C)**:
  - `doc.go` — overview (spot host, signed, deferred Structured Earn).
  - `client.go` — profile client (flat surface, no sub-clients) + `init()`
    factory registration; spot host via `kucoin.SpotFamilyBaseURL`.
  - `helpers.go` — signed REST GET/POST/DELETE wrappers, validation/auth errors.
  - `products.go` — savings/promotion/staking/kcs-staking/eth-staking (one
    shared fetch + `Product` converter; `lockEndTime`/`applyEndTime` null→0).
  - `orders.go` — Purchase, Redeem (+ ConfirmPunishRedeem), RedeemPreview,
    GetHoldings (paged).
  - `earn/types/*` — Product, Purchase/Redeem request+result, RedeemPreview,
    Holding(+Query)/HoldingPage.
  - Tests: `contract_rest_test.go` (mock REST end-to-end across products/orders/
    holdings; null-int + auth-required cases). Race-clean.
  - Root wiring (additive): `RegisterEarnFactory` + `Client.Earn()`.

- `viplending/` — **layer-2 VIP Lending / OTC-loan profile (v2.5 Phase C)**:
  - `doc.go` — overview (spot host, read-only OTC-loan queries).
  - `client.go` — profile client + inline signed-GET helper + `init()` factory.
  - `viplending.go` — GetCollateralConfigs, GetLoanInfo (orders + LTV +
    collateral legs), GetAccounts.
  - `viplending/types/*` — DiscountRateConfig/DiscountLevel, LoanInfo (LoanOrder,
    LTV, MarginAsset), LendingAccount.
  - Tests: `contract_rest_test.go` (mock REST end-to-end; auth-required).
    Race-clean.
  - Root wiring (additive): `RegisterVIPLendingFactory` + `Client.VIPLending()`.

- `subaccount/` — **layer-2 Sub-Account management profile (v2.5 Phase D)**:
  - `doc.go` — overview (spot host, master-account-only, host/security notes).
  - `client.go` — profile client (spot-bound REST) + `init()` factory.
  - `helpers.go` — signed GET/POST/DELETE core, `flexInt64` (number|string
    `createdAt`), validation/auth error constructors.
  - `subaccount.go` — Create, EnableMargin, EnableFutures, GetSummaries (paged),
    GetBalance, GetBalances (paged), CreateAPIKey, GetAPIKeys, UpdateAPIKey,
    DeleteAPIKey + wire structs/converters.
  - `subaccount/types/*` — CreateRequest/Result, SubUser(+Page),
    SubBalance/SubAccountAssets(+Page), CreateAPIKeyRequest/CreatedAPIKey,
    APIKey, UpdateAPIKeyRequest/UpdatedAPIKey, DeletedAPIKey.
  - Tests: `contract_rest_test.go` (mock REST end-to-end across create/perms/
    summaries/balances/API-key CRUD; flexInt64 number+string; auth-required +
    validation). Race-clean.
  - Root wiring (additive): `RegisterSubAccountFactory` + `Client.SubAccount()`.

- `convert/` — **layer-2 Convert profile (v2.5 Phase E)**:
  - `doc.go` — overview (spot host, fee-free swap, public vs signed split).
  - `client.go` — profile client (spot-bound REST) + `init()` factory.
  - `helpers.go` — public + signed GET/POST/DELETE core, `flexStr` (string|number
    orderId), validation/auth error constructors, query/format helpers.
  - `convert.go` — GetSymbol, GetCurrencies (public); GetQuote, PlaceMarketOrder,
    GetOrder, GetOrderHistory; GetLimitQuote, PlaceLimitOrder, GetLimitOrder,
    GetLimitOrders, CancelLimitOrder + wire structs/converters.
  - `convert/types/*` — Symbol, CurrencyLimit/Currencies, QuoteRequest/Quote,
    LimitQuote, PlaceMarket/LimitRequest, PlaceResult, Order(+Page),
    LimitOrder(+Page), HistoryQuery.
  - Tests: `contract_rest_test.go` (mock REST end-to-end across public + signed;
    flexStr number+string; nullable limit timestamps; auth-required +
    validation). Race-clean.
  - Root wiring (additive): `RegisterConvertFactory` + `Client.Convert()`.

- `affiliate/` — **layer-2 Affiliate profile (v2.5 Phase F, spot host)**:
  - `doc.go`, `client.go` (spot-bound REST + signed-GET helper + `init()`),
    `affiliate.go` — GetCommission, GetInviterRebate (deprecated) + wires.
  - `affiliate/types/*` — CommissionQuery/Commission, Rebate.
  - Tests: `contract_rest_test.go` (mock REST; auth-required). Race-clean.
  - Root wiring (additive): `RegisterAffiliateFactory` + `Client.Affiliate()`.

- `copytrading/` — **layer-2 futures Copy-Trading profile (v2.5 Phase F,
  futures host)**:
  - `doc.go` — overview (futures host, LeadtradeFutures permission, ISOLATED-only).
  - `client.go` — profile client using the parent's futures-bound REST
    (`parent.REST()`) + signed GET/POST/DELETE core + `init()` factory.
  - `copytrading.go` — PlaceOrder/PlaceOrderTest/PlaceTPSLOrder, CancelOrder,
    CancelOrderByClientOid, GetMaxOpenSize, GetMaxWithdrawMargin,
    AddIsolatedMargin, RemoveIsolatedMargin, ModifyRiskLimitLevel,
    SetAutoDepositStatus + Position wire.
  - `copytrading/types/*` — OrderRequest, TPSLOrderRequest, OrderResult,
    CancelResult, MaxOpenSize, AddMarginRequest, Position.
  - Tests: `contract_rest_test.go` (mock futures REST end-to-end; string/bool
    data shapes; position payload; auth-required + validation). Race-clean.
  - Root wiring (additive): `RegisterCopyTradingFactory` + `Client.CopyTrading()`.

- **Repo B (market-making-desk-core) — `kucoin/spot` connector (DONE,
  live-validated):** mirrors `kucoin/futures` on the `kucoin-connector`
  branch. Spot specifics: size in base currency (not contracts); position =
  total base-asset balance from the account WS (+ initial REST), `EntryPrice`
  0; batch chunked to 5 orders/call same-symbol; leverage / margin-mode /
  mark-index are no-ops with a warning. Plus a spot rate-limiter strategy
  (`spot`/`public` pools, `gw-ratelimit-*` driven), credential env-fallback for
  `kucoin_spot[_demo]`, an initial REST ticker seed (non-zero best bid/ask at
  start), and a generic `context.Canceled` classifier in high-level execution
  (restart/shutdown cancels no longer logged as "API error").

### 🔧 In progress

- **v2.5 — remaining sections** on top of the stable Futures + Spot core
  (additive `v2.5` branch):
  - **Phase A — Margin** (`margin/`): ✅ implemented & offline-tested; tagged
    `v2.2.0`. Fast-follows: stop/OCO margin orders, the margin lending market
    ("Credit").
  - **Phase B — Account/Funding** (`account/`): ✅ implemented & offline-tested;
    to be tagged `v2.3.0`. Covers account summary/ledgers, transfers (flex v3),
    deposit/withdrawal (v3), trade-fee, currencies (v3). Deferred fast-follows:
    sub-account management, legacy V1/V2 deposit-address & transfer endpoints,
    HF/futures ledgers.
  - **Phase C — Earn + VIP Lending** (`earn/`, `viplending/`): ✅ implemented &
    offline-tested; tagged `v2.4.0`. Earn products/purchase/redeem/holdings
    + OTC-loan read queries. Deferred: Structured Earn (dual investment).
  - **Phase D — Sub-Account management** (`subaccount/`): ✅ implemented &
    offline-tested; to be tagged `v2.5.0`. Create/permissions, summaries +
    spot balances (single/paged), spot API-key CRUD. Excluded: futures
    sub-account balance (futures host) + deprecated V1 list endpoints.
  - **Phase E — Convert** (`convert/`): ✅ implemented & offline-tested; to be
    tagged `v2.6.0`. Public symbol/currency directories + market & limit convert
    order lifecycle.
  - **Phase F — Affiliate + Copy-trading** (`affiliate/`, `copytrading/`): ✅
    implemented & offline-tested; to be tagged `v2.7.0`. Affiliate commission +
    rebate (spot host); futures copy-trade order/margin lifecycle (futures host,
    lead-trader account required).
  - **Phase G — Broker (nd + api, partner-only)** (`broker/`): planned
    (`v2.8.0`).
  - **Phase H — UTA / API v3.0** (`uta/`): separate large track (`/api/ua/v1/*`,
    unified account); its own sub-plan after D–G, likely the `v3.0.0` line.

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
