/*
FILE: types/cancel-order-request.go

DESCRIPTION:
Order cancellation request — protocol-common across KuCoin profiles.
Exactly one identifier (OrderID xor ClientOrderID) must be set. KuCoin
cancels by order id via DELETE /api/v1/orders/{orderId} and by client id
via DELETE /api/v1/orders/client-order/{clientOid} (the latter requires
Symbol). Symbol is optional for cancel-by-order-id and mandatory for
cancel-by-client-id; the profile validates this at the call site.
*/

package types

// CancelOrderRequest — order cancellation request.
type CancelOrderRequest struct {
	Symbol        string
	OrderID       string
	ClientOrderID string
}
