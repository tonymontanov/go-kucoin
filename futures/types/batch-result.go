/*
FILE: futures/types/batch-result.go

DESCRIPTION:
Per-row result wrapper for the KuCoin Futures batch place endpoint
(POST /api/v1/orders/multi). KuCoin returns one element per submitted order;
a row carries either the assigned OrderID (success) or a non-empty Code/Msg
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
	// Symbol — contract the row targeted.
	Symbol string
	// Success — true when KuCoin accepted the order (Code == "200000").
	Success bool
	// Code / Msg — KuCoin per-row status; populated on failure.
	Code string
	Msg  string
}
