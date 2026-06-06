/*
Package margin implements the KuCoin CLASSIC Margin profile (SDK v2.5).

It covers HIGH-FREQUENCY (HF) cross- and isolated-margin trading on the spot
host (api.kucoin.com). KuCoin completed the migration of all low-frequency
(LF) margin accounts to HF on 2026-03-04, retiring the legacy
/api/v1/margin/order LF order endpoints; this profile therefore targets the
HF order family (/api/v3/hf/margin/*) exclusively. There is no separate
"classic LF" order path to support.

# Scope (Phase A, v2.5)

  - MarketData — margin-specific market data: cross/isolated symbol config,
    mark price (single + all), global margin config. (The live order book,
    ticker and trade tape are IDENTICAL to Spot — margin trades on the spot
    matching engine — so use the spot profile for those; this profile does
    NOT duplicate them.)
  - Trading — HF margin order lifecycle: place, cancel (by id / clientOid /
    all-by-symbol), and the order/fill queries. tradeType selects cross
    ("MARGIN_TRADE") vs isolated ("MARGIN_ISOLATED_TRADE").
  - Debit — borrow, repay, borrow/repay/interest history, and the v3
    leverage update.
  - RiskLimit — cross/isolated risk-limit + borrow configuration.
  - Account — cross- and isolated-margin account snapshots (balances +
    liabilities + debt ratio).
  - Stream — PRIVATE margin order updates over the spot/margin private WS
    channel (/spotMarket/tradeOrders). Public book/ticker/trade streams come
    from the spot profile (same topics, same data).

# Host & transport

The profile builds its own REST client bound to the spot host via
kucoin.Client.NewSectionRESTClient (the root transport defaults to the
futures host). It shares the parent client's signer, rate-limit observers,
logger and metrics. Activate it through the root client:

	import (
	    kucoin "github.com/tonymontanov/go-kucoin/v2"
	    _ "github.com/tonymontanov/go-kucoin/v2/margin" // registers the factory
	    "github.com/tonymontanov/go-kucoin/v2/margin"
	)

	root, _ := kucoin.NewClient(cfg)
	m := root.Margin().(*margin.Client)

# Sizing

Margin sizing is in BASE currency (decimal), like Spot — there is NO contract
multiplier. Market orders may instead pass Funds (quote currency). Round Size
to SymbolInfo.BaseIncrement, Price to PriceIncrement and Funds to
QuoteIncrement before placing.

# Concurrency

Client and its sub-clients are safe for concurrent use. WS handlers run on the
read-loop goroutine and MUST be O(1) and non-blocking.

# Not yet covered (future v2.5 phases / fast-follows)

Stop and OCO margin orders (/api/v3/hf/margin/stop-order, .../oco-order) and
the margin lending market ("Credit": project/purchase/redeem) are intentionally
out of Phase A and tracked for a follow-up. Account/Funding and Earn land in
their own profiles (account/, earn/).
*/
package margin
