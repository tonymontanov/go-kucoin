/*
Package affiliate implements the KuCoin Affiliate profile (v2.5 Phase F).

SCOPE
Additive layer-2 profile on the SPOT host (api.kucoin.com / sandbox, resolved
via kucoin.SpotFamilyBaseURL). Read-only affiliate reports:

  - GetCommission — my settled-commission records (GET
    /api/v2/affiliate/queryMyCommission).
  - GetInviterRebate — inviter rebate statistics (GET
    /api/v2/affiliate/inviter/statistics). KuCoin marks this "Get Account"
    endpoint as DEPRECATED; it is included for completeness.

All endpoints are private (signed) and require the affiliate API permission.

DEFERRED
KuCoin recently added newer affiliate reports (Get Transaction / Get Invited /
Get Trade History); those are deferred fast-follows.

ARCHITECTURE
Additive, layer-2 profile: a root Client holds a dedicated REST client bound to
the spot host (sharing the parent signer + rate limiter). The small surface
lives directly on the Client (no sub-clients). Nothing in the existing profiles
or the shared internal/* packages is modified. Importing this package registers
the factory so kucoin.Client.Affiliate() returns *affiliate.Client.
*/
package affiliate
