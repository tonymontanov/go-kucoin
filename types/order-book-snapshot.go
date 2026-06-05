/*
FILE: types/order-book-snapshot.go

DESCRIPTION:
Order book snapshot — protocol-common across every KuCoin profile.
Returned by MarketData.GetOrderBook and produced by the SDK orderbook
engine after applying the WS "/contractMarket/level2" change stream on
top of a REST snapshot.

KuCoin Futures synchronisation model (SEQUENCE-based, NOT CRC32):
  - The REST level2 snapshot carries a baseline `sequence`.
  - Every WS level2 push carries a single `sequence` + a "price,side,size"
    change. Pushes are contiguous: each sequence == previous + 1.
  - The engine discards pushes with sequence <= the snapshot sequence,
    applies sequence == last+1, and triggers a resync (re-fetch snapshot)
    on any gap. There is NO checksum on KuCoin Futures level2.

FIELDS:
  - Symbol   — e.g. "XBTUSDTM".
  - Bids     — buy levels, sorted descending by price.
  - Asks     — sell levels, sorted ascending by price.
  - TsMs     — KuCoin publish timestamp (ms).
  - Sequence — sequence of the last applied change; 0 when not yet seeded.
*/

package types

// OrderBookSnapshot — order book snapshot for a single symbol.
type OrderBookSnapshot struct {
	Symbol   string
	Bids     []OrderBookLevel
	Asks     []OrderBookLevel
	TsMs     int64
	Sequence int64
}
