/*
FILE: spot/types/order-info.go

DESCRIPTION:
Order state for the KuCoin Spot profile, mapped from
GET /api/v1/orders/{orderId}, GET /api/v1/orders (list) and the private WS
"/spotMarket/tradeOrders" channel.

KuCoin Spot REST reports an order as active via IsActive (true = open /
partially filled, false = terminal); CancelExist distinguishes a cancel from
a full fill. The private WS channel additionally carries a lifecycle Status
("open"/"match"/"done") which is surfaced verbatim. Sizes are in BASE
currency; funds are in QUOTE currency.
*/

package types

import "github.com/shopspring/decimal"

// OrderInfo — KuCoin Spot order state.
type OrderInfo struct {
	OrderID       string
	ClientOrderID string
	Symbol        string
	Type          OrderType
	Side          SideType

	Price decimal.Decimal
	// Size — order size (base currency).
	Size decimal.Decimal
	// Funds — order value (quote currency); set for market-by-funds orders.
	Funds decimal.Decimal

	// DealSize / DealFunds — cumulative filled size (base) / value (quote).
	DealSize  decimal.Decimal
	DealFunds decimal.Decimal

	TimeInForce TimeInForceType
	TradeType   TradeType

	PostOnly    bool
	Hidden      bool
	Iceberg     bool
	VisibleSize decimal.Decimal
	CancelAfter int64

	STP SelfTradePrevention

	// Fee / FeeCurrency — cumulative fee paid.
	Fee         decimal.Decimal
	FeeCurrency string

	// Status — lifecycle subject on the private WS ("open"/"match"/"done");
	// empty on REST, where IsActive / CancelExist are authoritative.
	Status      OrderStatus
	IsActive    bool
	CancelExist bool

	Remark      string
	CreatedAtMs int64
	UpdatedAtMs int64
}
