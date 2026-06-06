/*
Package account implements the KuCoin Account & Funding profile (v2.5).

SCOPE
This profile is the cross-cutting "treasury" layer that sits beside the
trading profiles (futures/, spot/, margin/). It covers money and metadata that
are not tied to a single trading venue:

  - Account: summary (sub-account quotas / VIP level), API-key info, spot wallet
    list/detail, and account ledgers (spot/margin + trade_hf).
  - Deposit: create/query v3 deposit addresses, deposit history.
  - Withdrawal: quotas, submit (v3), cancel, history.
  - Transfer: transferable balance + the v3 flex (universal) transfer that moves
    assets between wallets (MAIN/TRADE/MARGIN/ISOLATED/CONTRACT) and between
    master and sub-accounts.
  - Fee: account base spot/margin rate and actual per-symbol trade fees.
  - Currency: the v3 currency directory (chains, precisions, withdraw/deposit
    minimums) needed to build valid withdraw requests.

HOST
All endpoints live on the SPOT host family (api.kucoin.com / sandbox), resolved
via kucoin.SpotFamilyBaseURL — the same host as the spot and margin profiles.
The "futures" account endpoints (account-overview, transaction-history,
transfer-in/out) live on the FUTURES host and remain in the futures/ profile;
they are intentionally NOT duplicated here.

ARCHITECTURE
Additive, layer-2 profile mirroring spot/ and margin/: a root Client holds a
dedicated REST client bound to the spot host (sharing the parent signer + rate
limiter) and exposes domain sub-clients. Nothing in futures/, spot/, margin/ or
the shared internal/* packages is modified. Importing this package registers
the factory so kucoin.Client.Account() returns *account.Client.

CONCURRENCY
Client and its sub-clients are safe for concurrent use; they hold no mutable
state after construction.

NOT INCLUDED (deferred)
Sub-account management (create / permissions / API-key CRUD / per-sub balances),
the legacy V1/V2 deposit-address and transfer endpoints, and futures-host
account endpoints (see futures/).
*/
package account
