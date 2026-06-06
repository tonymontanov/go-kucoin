/*
Package copytrading implements the KuCoin futures Copy-Trading (lead-trader)
profile (v2.5 Phase F).

SCOPE
Additive layer-2 profile on the FUTURES host (api-futures.kucoin.com, shared
with the futures profile via the parent's REST client). Lead-trader futures
copy-trading endpoints under /api/v1/copy-trade/futures/*:

  - Orders: PlaceOrder / PlaceOrderTest, PlaceTPSLOrder (take-profit + stop-loss),
    CancelOrder (by orderId), CancelOrderByClientOid.
  - Sizing / margin: GetMaxOpenSize, GetMaxWithdrawMargin, AddIsolatedMargin,
    RemoveIsolatedMargin, ModifyRiskLimitLevel, SetAutoDepositStatus.

ACCOUNT REQUIREMENT
These endpoints require the LeadtradeFutures permission (a lead-trader /
copy-trading account). They cannot be exercised by a regular account, so this
profile ships with offline contract tests only.

NOTES
  - Copy-trading currently supports ISOLATED margin only (CROSS → 180204) and a
    max leverage of 20x.
  - Orders use a hedge-mode PositionSide ("LONG"/"SHORT"/"BOTH").

ARCHITECTURE
Additive, layer-2 profile: the methods live directly on the Client (no
sub-clients). REST goes through the parent's futures-bound REST client, exactly
like the futures profile. Nothing in the existing profiles or the shared
internal/* packages is modified. Importing this package registers the factory so
kucoin.Client.CopyTrading() returns *copytrading.Client.
*/
package copytrading
