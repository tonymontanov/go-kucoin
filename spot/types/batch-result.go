/*
FILE: spot/types/batch-result.go

DESCRIPTION:
Per-row result wrapper for the KuCoin Spot batch place endpoint
(POST /api/v1/orders/multi). KuCoin returns one element per submitted order;
a row carries either the assigned OrderID (success) or a failReason / status
(failure), so callers can reconcile a partial batch without aborting the
whole request.
*/

package types

// BatchOrderResult — outcome of one order within a batch request.
type BatchOrderResult struct {
	// OrderID — assigned id on success; empty on failure.
	OrderID string
	// ClientOrderID — echoed clientOid (success or failure).
	ClientOrderID string
	// Symbol — pair the row targeted.
	Symbol string
	// Success — true when KuCoin accepted the order (status == "success").
	Success bool
	// Status / FailMsg — KuCoin per-row status ("success"/"fail") and the
	// failure reason when rejected.
	Status  string
	FailMsg string
}
