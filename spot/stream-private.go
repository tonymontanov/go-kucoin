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

ORDER LIFECYCLE PUSH: every tradeOrders frame carries the lifecycle event
(`type`: received/open/match/filled/canceled/update) and, on "match", the
execution itself (`matchSize`, `matchPrice`, `tradeId`, `liquidity`). All of
them are surfaced on OrderInfo (EventType, MatchSize, MatchPrice, TradeID,
Liquidity, TsNs) so a consumer can build an exact fill ledger. Non-match
frames ship `matchSize`/`matchPrice` as "" or omit them — flexDecimal
decodes both to zero instead of failing the whole frame.

RECONNECT HOOK: WatchOrdersWithOptions{OnConnect} fires on every (re)connect
of the private socket after the tradeOrders subscription was re-sent and
before the first push of that connection — the consumer reseeds its order
state there (see ws.Subscription.Ready).

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

/*
flexDecimal decodes a decimal that KuCoin may ship as a quoted number, a bare
number, an EMPTY string or omit entirely. The tradeOrders push carries
`matchSize`/`matchPrice` only on "match" frames and sends "" on the others;
a plain decimal.Decimal rejects "" and — because the decoder fails on the
WHOLE frame — the open/canceled/filled push would be silently dropped.
Empty / null / absent → zero.
*/
type flexDecimal struct{ decimal.Decimal }

func (f *flexDecimal) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" || string(b) == `""` {
		f.Decimal = decimal.Zero
		return nil
	}
	return f.Decimal.UnmarshalJSON(b)
}

// OrderStreamOptions — optional hooks for WatchOrdersWithOptions.
type OrderStreamOptions struct {
	// OnConnect is invoked on the read-loop goroutine after every successful
	// (re)connect of the private socket, once the tradeOrders subscription
	// has been (re)sent and BEFORE the first push of that connection reaches
	// the handler. A consumer that keeps order state reseeds it here (e.g. a
	// REST open-orders snapshot) with the guarantee that no push of the new
	// connection precedes the reseed. It may block — only this socket's
	// reads wait for it; the heartbeat keeps running.
	// NOT invoked when the private socket is already connected at
	// subscription time: seed once right after WatchOrdersWithOptions
	// returns, then rely on OnConnect for reconnects. May be nil.
	OnConnect func()
}

// WatchOrders subscribes to account-wide order lifecycle updates.
func (s *StreamClient) WatchOrders(ctx context.Context, handler func(*spottypes.OrderInfo)) error {
	return s.WatchOrdersWithOptions(ctx, handler, OrderStreamOptions{})
}

// WatchOrdersWithOptions is WatchOrders plus the reconnect hook (see
// OrderStreamOptions). Subscribing the same topic twice replaces the earlier
// handler on the shared private connection — fan out in the caller.
func (s *StreamClient) WatchOrdersWithOptions(ctx context.Context, handler func(*spottypes.OrderInfo), opts OrderStreamOptions) error {
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
			var o spottypes.OrderInfo
			var perr error
			o, perr = DecodeOrderPush(data)
			if perr != nil {
				return
			}
			handler(&o)
		},
		Ready: opts.OnConnect,
	})
}

// DecodeOrderPush decodes the `data` payload of a /spotMarket/tradeOrders
// push into OrderInfo — the exact conversion WatchOrders applies. Exported
// so consumers can replay recorded frames (tests, offline analysis) through
// the same decoder.
func DecodeOrderPush(data []byte) (spottypes.OrderInfo, error) {
	var w orderPushWire
	var err error = codecUnmarshal(data, &w)
	if err != nil {
		return spottypes.OrderInfo{}, err
	}
	return w.toOrderInfo(), nil
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
// matchSize/matchPrice/tradeId/liquidity are set on "match" frames only
// (empty string or absent otherwise — hence flexDecimal).
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
	MatchSize  flexDecimal     `json:"matchSize"`
	MatchPrice flexDecimal     `json:"matchPrice"`
	Liquidity  string          `json:"liquidity"`
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
		EventType:     w.Type,
		TradeID:       w.TradeID,
		MatchSize:     w.MatchSize.Decimal,
		MatchPrice:    w.MatchPrice.Decimal,
		Liquidity:     spottypes.Liquidity(w.Liquidity),
		TsNs:          w.Ts.int64(),
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
