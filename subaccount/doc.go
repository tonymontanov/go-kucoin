/*
Package subaccount implements the KuCoin Sub-Account management profile
(v2.5 Phase D).

SCOPE
Additive layer-2 profile on the SPOT host (api.kucoin.com / sandbox, resolved
via kucoin.SpotFamilyBaseURL). Master-account-only operations:

  - Create a sub-account and grant it margin / futures permission.
  - List sub-account summaries (paged) and spot balances (single + paged).
  - Manage the spot sub-account API-key lifecycle: create / list / modify /
    delete.

All endpoints are private (signed) and must be called with a MASTER-account API
key.

HOST NOTE
The futures sub-account balance endpoint (/api/v1/account-overview-all) lives on
the FUTURES host and is intentionally NOT included here (keep this profile on
the spot host); query it via the futures profile if needed. The deprecated V1
summary / balance endpoints are also omitted in favour of their V2 successors.

SECURITY
CreateAPIKey returns the API secret and passphrase EXACTLY ONCE; the caller must
persist them immediately. They are never retrievable afterwards.

ARCHITECTURE
Additive, layer-2 profile mirroring the other section profiles: a root Client
holds a dedicated REST client bound to the spot host (sharing the parent signer
+ rate limiter) and exposes the methods directly. Nothing in the existing
profiles or the shared internal/* packages is modified. Importing this package
registers the factory so kucoin.Client.SubAccount() returns *subaccount.Client.
*/
package subaccount
