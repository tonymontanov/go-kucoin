/*
FILE: rate-limit-event.go

DESCRIPTION:
Public RateLimitEvent type that the SDK delivers to subscribers via
Config.RateLimitEventObserver. The observer pattern is identical to
go-bybit / go-okx / go-bitget: the SDK writes once per completed REST
call, the desk rate-limiter consumes events to update its model.

WHY HEADERS ARE FORWARDED 1:1:
KuCoin returns rate-limit metadata headers on most REST responses
(gw-ratelimit-limit / gw-ratelimit-remaining / gw-ratelimit-reset, plus
retry-after on 429). KuCoin meters by RESOURCE POOLS with per-request
weights (the spot/management/public pools, and the futures pool), so the
live gw-ratelimit-remaining value is the authoritative budget. They are
forwarded as-is so an external rate-limiter at the desk level can
reconcile its model with the live remaining budget.

THE THREE METADATA AXES:

  1. OrderCount: 1 for single-order endpoints, len(orders) for batch
     endpoints (multi-order place / cancel), 0 for non-trading queries.
  2. Symbols:    sorted unique list of symbols affected by the request.
  3. Category:   "place" | "amend" | "cancel" | "query" | "market" | "".
     Used by the rate-limiter to attribute weight to the right plane.
*/

package kucoin

// RateLimitCategory classifies a REST call from the rate-limit model
// perspective. Used by external rate-limiters to distribute usage across
// different limit planes.
type RateLimitCategory string

const (
	// RateLimitCategoryPlace — order creation. Endpoints:
	// /api/v1/orders (futures place), batch place, etc.
	RateLimitCategoryPlace RateLimitCategory = "place"

	// RateLimitCategoryAmend — order modification (where supported).
	RateLimitCategoryAmend RateLimitCategory = "amend"

	// RateLimitCategoryCancel — order cancellation. Endpoints:
	// /api/v1/orders/{id} (DELETE), cancel-all, batch cancel.
	RateLimitCategoryCancel RateLimitCategory = "cancel"

	// RateLimitCategoryQuery — private GET / non-trading POST. Endpoints:
	// order/position/account queries, set-leverage, etc.
	RateLimitCategoryQuery RateLimitCategory = "query"

	// RateLimitCategoryMarketData — public GET (per-IP / public pool).
	// Endpoints: depth, klines, ticker, contracts list, etc.
	RateLimitCategoryMarketData RateLimitCategory = "market"

	// RateLimitCategoryUnknown — fallback for requests not covered by any
	// explicit category. Treat as Query for safety in subscribers.
	RateLimitCategoryUnknown RateLimitCategory = ""
)

// String returns the string representation.
func (c RateLimitCategory) String() string { return string(c) }

// RateLimitEvent is the structured event delivered to
// Config.RateLimitEventObserver. The SDK emits exactly one event per
// completed REST call (whether successful or rejected at the application
// layer). Network-only failures (timeout before any HTTP response) do
// NOT trigger the observer.
type RateLimitEvent struct {
	// Endpoint — request path in canonical form (e.g.
	// "/api/v1/orders"). Never empty.
	Endpoint string

	// Method — HTTP method in upper-case ("GET", "POST", ...).
	Method string

	// Headers — selected rate-limit headers from the response. Populated
	// from the KuCoin gateway family when present:
	//
	//   gw-ratelimit-limit     (max weight in current window)
	//   gw-ratelimit-remaining (remaining weight)
	//   gw-ratelimit-reset     (window reset, ms)
	//   retry-after            (set on 429 responses)
	//
	// May be empty for endpoints that do not advertise limits. Always
	// non-nil.
	Headers map[string]string

	// OrderCount — number of orders this request creates / amends /
	// cancels:
	//   - 1 for single-order endpoints;
	//   - len(orders) for batch endpoints;
	//   - 0 for non-trading queries.
	OrderCount int

	// Symbols — sorted unique list of symbols affected. Empty for
	// account-level queries (wallet balance, fee rates, etc.).
	Symbols []string

	// Category — see RateLimitCategory.
	Category RateLimitCategory
}
