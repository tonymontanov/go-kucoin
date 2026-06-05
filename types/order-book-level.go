/*
FILE: types/order-book-level.go

DESCRIPTION:
A single order book level — protocol-common across every KuCoin profile.
Used by:
  - REST snapshot (GET /api/v1/level2/snapshot for futures);
  - the SDK orderbook engine (snapshot/delta application);
  - WebSocket "/contractMarket/level2:{symbol}" channel dispatch.

KuCoin represents a level as a positional [price, size] pair: on REST as
JSON numbers ([5000.0, 83]) and on the WS level2 channel as a single
comma-joined "price,side,size" change string. The SDK normalises price
and size into decimal.Decimal at the boundary.
*/

package types

import "github.com/shopspring/decimal"

// OrderBookLevel — one order book level.
type OrderBookLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}
