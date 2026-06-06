/*
Package earn implements the KuCoin Earn profile (v2.5 Phase C).

SCOPE
Additive layer-2 profile on the SPOT host (api.kucoin.com / sandbox, resolved
via kucoin.SpotFamilyBaseURL) covering KuCoin Earn:

  - Products: catalogue of savings / promotion / staking / KCS-staking /
    ETH-staking offerings (all share the same row shape).
  - Subscribe / redeem: Purchase a product, Redeem a holding (with a
    redeem-preview that surfaces any early-redemption penalty).
  - Holdings: the caller's current Earn positions (paged).

All endpoints are private (signed). Reads are issued GET, Purchase is POST and
Redeem is DELETE on /api/v1/earn/orders.

ARCHITECTURE
Additive, layer-2 profile mirroring spot/, margin/ and account/: a root Client
holds a dedicated REST client bound to the spot host (sharing the parent signer
+ rate limiter) and exposes the Earn methods directly (the surface is small and
cohesive, so no further sub-client split). Nothing in the existing profiles or
the shared internal/* packages is modified. Importing this package registers
the factory so kucoin.Client.Earn() returns *earn.Client.

NOT INCLUDED (deferred)
Structured Earn (dual investment) — KuCoin reports those endpoints as not
generally available (400100); they are intentionally omitted until live.
*/
package earn
