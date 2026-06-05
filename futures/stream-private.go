/*
FILE: futures/stream-private.go

DESCRIPTION:
Private WebSocket watchers for the KuCoin Futures profile. They run over the
PRIVATE bullet connection (POST /api/v1/bullet-private, signed) created
lazily by ensurePrivate. Calling any of these without configured API
credentials returns ErrorKindAuth.

TOPICS (private):
  - /contractMarket/tradeOrders        order lifecycle (account-wide)
  - /contract/position:{symbol}        position changes (per symbol)
  - /contractAccount/wallet            balance / margin changes

The private push payloads use KuCoin's WS field names, which differ from the
REST shapes; the converters below map the overlapping fields into the same
typed structs (OrderInfo / PositionInfo / Balance). Fields the WS frame does
not carry are left at their zero value.

HANDLER CONTRACT: handlers run on the read-loop goroutine — O(1), non-blocking.
*/

package futures

import (
	"context"

	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/ws"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// WatchOrders subscribes to account-wide order lifecycle updates.
func (s *StreamClient) WatchOrders(ctx context.Context, handler func(*futurestypes.OrderInfo)) error {
	if handler == nil {
		return errInvalidRequest("WatchOrders", "handler is required")
	}
	var conn *ws.Conn
	var err error
	conn, err = s.ensurePrivate(ctx)
	if err != nil {
		return err
	}
	return conn.Subscribe(&ws.Subscription{
		Topic:          "/contractMarket/tradeOrders",
		PrivateChannel: true,
		Handler: func(_, _ string, data []byte) {
			var w orderPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			var o futurestypes.OrderInfo = w.toOrderInfo()
			handler(&o)
		},
	})
}

// WatchPositions subscribes to position changes for a single contract.
func (s *StreamClient) WatchPositions(ctx context.Context, symbol string, handler func(*futurestypes.PositionInfo)) error {
	if symbol == "" || handler == nil {
		return errInvalidRequest("WatchPositions", "symbol and handler are required")
	}
	var conn *ws.Conn
	var err error
	conn, err = s.ensurePrivate(ctx)
	if err != nil {
		return err
	}
	return conn.Subscribe(&ws.Subscription{
		Topic: "/contract/position:" + symbol,
		Handler: func(_, _ string, data []byte) {
			var w positionPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			// KuCoin шлёт несколько подвидов position.change: операционные
			// (positionChange / marginChange / changeRiskLimit / liquidation /
			// adl) НЕСУТ currentQty, тогда как кадры, вызванные mark price
			// (и position.settlement), его НЕ несут. Кадр без currentQty
			// НЕЛЬЗЯ трактовать как qty=0 — иначе подписчик обнулял бы
			// инвентарь на каждом mark-тике. Мы НЕ дропаем такие кадры (они
			// полезны как сигнал «позиция открыта / возможно изменилась»),
			// а помечаем CurrentQtyKnown=false и пробрасываем — пусть
			// потребитель сам решит (см. WatchPosition в коннекторе:
			// known → эмит напрямую, unknown → REST-доуточнение).
			var p futurestypes.PositionInfo = w.toPositionInfo(symbol)
			handler(&p)
		},
	})
}

// WatchBalance subscribes to account balance / margin changes.
func (s *StreamClient) WatchBalance(ctx context.Context, handler func(*roottypes.Balance)) error {
	if handler == nil {
		return errInvalidRequest("WatchBalance", "handler is required")
	}
	var conn *ws.Conn
	var err error
	conn, err = s.ensurePrivate(ctx)
	if err != nil {
		return err
	}
	return conn.Subscribe(&ws.Subscription{
		Topic: "/contractAccount/wallet",
		Handler: func(_, _ string, data []byte) {
			var w walletPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			var b roottypes.Balance = w.toBalance()
			handler(&b)
		},
	})
}

// UnsubscribePrivate removes a private subscription by full topic string.
func (s *StreamClient) UnsubscribePrivate(topic string) error {
	s.mu.Lock()
	var conn *ws.Conn = s.privateConn
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Unsubscribe(topic)
}

// ---------------------------------------------------------------------
// Private push wire structs + converters.
// ---------------------------------------------------------------------

// orderPushWire mirrors a /contractMarket/tradeOrders push. `type` is the
// lifecycle event (received/open/match/filled/canceled/update); `status` is
// the resulting order state (match/open/done). ts/orderTime are nanoseconds.
type orderPushWire struct {
	OrderID    string          `json:"orderId"`
	Symbol     string          `json:"symbol"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	OrderType  string          `json:"orderType"`
	Side       string          `json:"side"`
	Price      decimal.Decimal `json:"price"`
	Size       decimal.Decimal `json:"size"`
	FilledSize decimal.Decimal `json:"filledSize"`
	RemainSize decimal.Decimal `json:"remainSize"`
	ClientOid  string          `json:"clientOid"`
	TradeID    string          `json:"tradeId"`
	OrderTime  int64           `json:"orderTime"`
	Ts         int64           `json:"ts"`
}

func (w orderPushWire) toOrderInfo() futurestypes.OrderInfo {
	return futurestypes.OrderInfo{
		OrderID:       w.OrderID,
		ClientOrderID: w.ClientOid,
		Symbol:        w.Symbol,
		Type:          futurestypes.OrderType(w.OrderType),
		Side:          futurestypes.SideType(w.Side),
		Price:         w.Price,
		Size:          w.Size,
		FilledSize:    w.FilledSize,
		Status:        futurestypes.OrderStatus(w.Status),
		IsActive:      w.Status == "open" || w.Status == "match",
		UpdatedAtMs:   nsToMs(w.Ts),
		CreatedAtMs:   nsToMs(w.OrderTime),
	}
}

// positionPushWire mirrors a /contract/position push.
//
// CurrentQty is a POINTER on purpose: KuCoin omits it on non-position
// subjects (markPriceChange / marginChange / riskLimitChange / ...). A nil
// pointer means "this frame does not report position size" — distinct from a
// genuine flat position (currentQty present with value 0). WatchPositions
// uses this to skip non-qty frames instead of emitting a spurious 0.
type positionPushWire struct {
	Symbol           string           `json:"symbol"`
	CurrentQty       *decimal.Decimal `json:"currentQty"`
	AvgEntryPrice    decimal.Decimal  `json:"avgEntryPrice"`
	MarkPrice        decimal.Decimal `json:"markPrice"`
	MarkValue        decimal.Decimal `json:"markValue"`
	UnrealisedPnl    decimal.Decimal `json:"unrealisedPnl"`
	LiquidationPrice decimal.Decimal `json:"liquidationPrice"`
	BankruptPrice    decimal.Decimal `json:"bankruptPrice"`
	PosMargin        decimal.Decimal `json:"posMargin"`
	PosCost          decimal.Decimal `json:"posCost"`
	RealLeverage     decimal.Decimal `json:"realLeverage"`
	MaintMarginReq   decimal.Decimal `json:"maintMarginReq"`
	RiskLimit        decimal.Decimal `json:"riskLimit"`
	SettleCurrency   string          `json:"settleCurrency"`
	CrossMode        bool            `json:"crossMode"`
}

func (w positionPushWire) toPositionInfo(symbol string) futurestypes.PositionInfo {
	var mode futurestypes.MarginMode = futurestypes.MarginIsolated
	if w.CrossMode {
		mode = futurestypes.MarginCross
	}
	var sym string = w.Symbol
	if sym == "" {
		sym = symbol
	}
	// CurrentQty приходит указателем: nil ⇒ кадр не несёт размер позиции
	// (mark-price / settlement). Тогда CurrentQtyKnown=false, qty=0 — но
	// потребитель ОБЯЗАН проверять флаг, а не qty.
	var qty decimal.Decimal
	var qtyKnown bool
	if w.CurrentQty != nil {
		qty = *w.CurrentQty
		qtyKnown = true
	}
	return futurestypes.PositionInfo{
		Symbol:           sym,
		SettleCurrency:   w.SettleCurrency,
		IsOpen:           qtyKnown && !qty.IsZero(),
		CrossMode:        w.CrossMode,
		MarginMode:       mode,
		CurrentQty:       qty,
		CurrentQtyKnown:  qtyKnown,
		AvgEntryPrice:    w.AvgEntryPrice,
		MarkPrice:        w.MarkPrice,
		MarkValue:        w.MarkValue,
		LiquidationPrice: w.LiquidationPrice,
		BankruptPrice:    w.BankruptPrice,
		RealLeverage:     w.RealLeverage,
		PosMargin:        w.PosMargin,
		PosCost:          w.PosCost,
		MaintMarginReq:   w.MaintMarginReq,
		RiskLimit:        w.RiskLimit,
		UnrealizedPnL:    w.UnrealisedPnl,
	}
}

// walletPushWire mirrors a /contractAccount/wallet push. Depending on the
// subject, either availableBalance or orderMargin carries the changed value.
type walletPushWire struct {
	Currency         string          `json:"currency"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
	OrderMargin      decimal.Decimal `json:"orderMargin"`
	HoldBalance      decimal.Decimal `json:"holdBalance"`
	Timestamp        int64           `json:"timestamp"`
}

func (w walletPushWire) toBalance() roottypes.Balance {
	return roottypes.Balance{
		MarginCoin:       w.Currency,
		AvailableBalance: w.AvailableBalance,
		LockedBalance:    w.OrderMargin,
		Coins: []roottypes.CoinBalance{
			{
				Coin:        w.Currency,
				Available:   w.AvailableBalance,
				OrderMargin: w.OrderMargin,
			},
		},
	}
}
