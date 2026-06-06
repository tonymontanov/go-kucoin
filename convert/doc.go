/*
Package convert implements the KuCoin Convert profile (v2.5 Phase E).

SCOPE
Additive layer-2 profile on the SPOT host (api.kucoin.com / sandbox, resolved
via kucoin.SpotFamilyBaseURL). KuCoin Convert is a fee-free currency swap; the
quoted price embeds a spread to cover volatility.

  - Public directories: convertible pair limits (GetSymbol) and the currency
    list (GetCurrencies).
  - Market convert: GetQuote → PlaceMarketOrder, plus order detail / history.
  - Limit convert: GetLimitQuote (protection price) → PlaceLimitOrder, plus
    limit-order detail / list / cancel.

All endpoints except GetSymbol / GetCurrencies are private (signed).

ARCHITECTURE
Additive, layer-2 profile mirroring the other section profiles: a root Client
holds a dedicated REST client bound to the spot host (sharing the parent signer
+ rate limiter). The domain is small and cohesive, so the methods live directly
on the Client (no sub-clients). Nothing in the existing profiles or the shared
internal/* packages is modified. Importing this package registers the factory
so kucoin.Client.Convert() returns *convert.Client.

NOTE
Convert order IDs arrive as a quoted string for limit orders and a bare number
for market-order detail; a flexStr tolerates both and normalises to string.
*/
package convert
