/*
FILE: spot/stream-private.go

DESCRIPTION:
Private WebSocket watchers for the KuCoin Spot profile. They run over the
PRIVATE bullet connection (POST /api/v1/bullet-private, signed) created
lazily by ensurePrivate. Calling any of these without configured API
credentials returns ErrorKindAuth.

TOPICS (private):
  - /spotMarket/tradeOrders   order lifecycle (account-wide)
  - /account/balance          balance changes (per currency)

HANDLER CONTRACT: handlers run on the read-loop goroutine — O(1), non-blocking.
*/

package spot

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
	"github.com/tonymontanov/go-kucoin/v2/internal/ws"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

/*
flexInt64 decodes an integer that KuCoin ships INCONSISTENTLY as either a
bare JSON number or a quoted string across spot private channels. The
/account/balance push delivers `"time":"1730269283892"` (QUOTED), while
/spotMarket/tradeOrders delivers `"orderTime":169...` (BARE). A plain int64
field rejects the quoted form, and because the decoder fails on the WHOLE
frame, the balance push was silently dropped — which looked like "the
inventory websocket isn't working" (position only refreshed on the 60s REST
poll). flexInt64 accepts both shapes so the frame always decodes.
*/
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	// Strip surrounding quotes if present (quoted-string form).
	if b[0] == '"' && len(b) >= 2 && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}
	if len(b) == 0 {
		*f = 0
		return nil
	}
	var v int64
	var err error
	v, err = codec.ParseInt64(string(b))
	if err != nil {
		return err
	}
	*f = flexInt64(v)
	return nil
}

func (f flexInt64) int64() int64 { return int64(f) }

// WatchOrders subscribes to account-wide order lifecycle updates.
func (s *StreamClient) WatchOrders(ctx context.Context, handler func(*spottypes.OrderInfo)) error {
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
		Topic:          "/spotMarket/tradeOrders",
		PrivateChannel: true,
		Handler: func(_, _ string, data []byte) {
			var w orderPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			var o spottypes.OrderInfo = w.toOrderInfo()
			handler(&o)
		},
	})
}

// WatchBalance subscribes to account balance changes.
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
		Topic:          "/account/balance",
		PrivateChannel: true,
		Handler: func(_, _ string, data []byte) {
			var w balancePushWire
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

// orderPushWire mirrors a /spotMarket/tradeOrders push. `type` is the
// lifecycle event (received/open/match/filled/canceled/update); `status` is
// the resulting order state (open/match/done). orderTime/ts are nanoseconds.
type orderPushWire struct {
	OrderID    string          `json:"orderId"`
	Symbol     string          `json:"symbol"`
	OrderType  string          `json:"orderType"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	Side       string          `json:"side"`
	Price      decimal.Decimal `json:"price"`
	Size       decimal.Decimal `json:"size"`
	FilledSize decimal.Decimal `json:"filledSize"`
	RemainSize decimal.Decimal `json:"remainSize"`
	Funds      decimal.Decimal `json:"funds"`
	ClientOid  string          `json:"clientOid"`
	TradeID    string          `json:"tradeId"`
	OrderTime  flexInt64       `json:"orderTime"`
	Ts         flexInt64       `json:"ts"`
}

func (w orderPushWire) toOrderInfo() spottypes.OrderInfo {
	return spottypes.OrderInfo{
		OrderID:       w.OrderID,
		ClientOrderID: w.ClientOid,
		Symbol:        w.Symbol,
		Type:          spottypes.OrderType(w.OrderType),
		Side:          spottypes.SideType(w.Side),
		Price:         w.Price,
		Size:          w.Size,
		Funds:         w.Funds,
		DealSize:      w.FilledSize,
		Status:        spottypes.OrderStatus(w.Status),
		IsActive:      w.Status == "open" || w.Status == "match",
		UpdatedAtMs:   nsToMs(w.Ts.int64()),
		CreatedAtMs:   nsToMs(w.OrderTime.int64()),
	}
}

// balancePushWire mirrors an /account/balance push.
type balancePushWire struct {
	Currency        string          `json:"currency"`
	Total           decimal.Decimal `json:"total"`
	Available       decimal.Decimal `json:"available"`
	AvailableChange decimal.Decimal `json:"availableChange"`
	Hold            decimal.Decimal `json:"hold"`
	HoldChange      decimal.Decimal `json:"holdChange"`
	Time            flexInt64       `json:"time"`
}

func (w balancePushWire) toBalance() roottypes.Balance {
	return roottypes.Balance{
		MarginCoin:       w.Currency,
		TotalEquity:      w.Total,
		AvailableBalance: w.Available,
		LockedBalance:    w.Hold,
		Coins: []roottypes.CoinBalance{
			{
				Coin:        w.Currency,
				Equity:      w.Total,
				Available:   w.Available,
				FrozenFunds: w.Hold,
			},
		},
	}
}
