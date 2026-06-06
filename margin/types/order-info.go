/*
FILE: margin/types/order-info.go

DESCRIPTION:
Order state for the KuCoin Margin profile, mapped from the HF-margin order
endpoints (GET /api/v3/hf/margin/orders/{orderId}, .../orders/active,
.../orders/done) and the private "/spotMarket/tradeOrders" WS channel (margin
orders flow over the spot/margin private channel).

KuCoin reports an order as active via IsActive (true = open / partially
filled, false = terminal); CancelExist distinguishes a cancel from a full
fill. The private WS additionally carries a lifecycle Status
("open"/"match"/"done"). Sizes are in BASE currency; funds in QUOTE.

MARGIN EXTRAS: BorrowSize / LoanApplyID echo an auto-borrow that funded the
order; TradeType reports cross vs isolated.
*/

package types

import "github.com/shopspring/decimal"

// OrderInfo — KuCoin Margin order state.
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
	// TradeType — MARGIN_TRADE (cross) / MARGIN_ISOLATED_TRADE (isolated).
	TradeType TradeType

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
