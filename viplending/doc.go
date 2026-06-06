/*
Package viplending implements the KuCoin VIP Lending (OTC loan) profile
(v2.5 Phase C).

SCOPE
Additive layer-2 profile on the SPOT host (api.kucoin.com / sandbox, resolved
via kucoin.SpotFamilyBaseURL). VIP Lending is institutional off-exchange
funding; the public REST surface is read-only:

  - GetCollateralConfigs: the gradient collateral (discount) rate per currency
    (/api/v1/otc-loan/discount-rate-configs).
  - GetLoanInfo: the caller's consolidated loan position — orders, LTV
    thresholds and collateral legs (/api/v1/otc-loan/loan).
  - GetAccounts: the accounts participating in OTC lending
    (/api/v1/otc-loan/accounts).

All endpoints are private (signed) GETs. Loan origination / repayment is
arranged off-exchange (OTC) and is not part of the API.

ARCHITECTURE
Additive, layer-2 profile mirroring the other section profiles: a root Client
holds a dedicated REST client bound to the spot host (sharing the parent signer
+ rate limiter) and exposes the read methods directly. Nothing in the existing
profiles or the shared internal/* packages is modified. Importing this package
registers the factory so kucoin.Client.VIPLending() returns *viplending.Client.
*/
package viplending
