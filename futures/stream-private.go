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

ORDER LIFECYCLE PUSH: every tradeOrders frame carries the lifecycle event
(`type`: open/match/filled/canceled/update) and, on "match", the execution
itself (`matchSize` in contracts, `matchPrice`, `tradeId`, `liquidity`). All
of them are surfaced on OrderInfo (EventType, MatchSize, MatchPrice, TradeID,
Liquidity, TsNs) so a consumer can build an exact fill ledger. Non-match
frames ship `matchSize`/`matchPrice` as "" — flexDecimal decodes that to
zero instead of failing the whole frame.

RECONNECT HOOK: WatchOrdersWithOptions{OnConnect} fires on every (re)connect
of the private socket after the tradeOrders subscription was re-sent and
before the first push of that connection — the consumer reseeds its order
state there (see ws.Subscription.Ready).

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
func (s *StreamClient) WatchOrders(ctx context.Context, handler func(*futurestypes.OrderInfo)) error {
	return s.WatchOrdersWithOptions(ctx, handler, OrderStreamOptions{})
}

// WatchOrdersWithOptions is WatchOrders plus the reconnect hook (see
// OrderStreamOptions). Subscribing the same topic twice replaces the earlier
// handler on the shared private connection — fan out in the caller.
func (s *StreamClient) WatchOrdersWithOptions(ctx context.Context, handler func(*futurestypes.OrderInfo), opts OrderStreamOptions) error {
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
			var o futurestypes.OrderInfo
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

// DecodeOrderPush decodes the `data` payload of a /contractMarket/tradeOrders
// push into OrderInfo — the exact conversion WatchOrders applies. Exported
// so consumers can replay recorded frames (tests, offline analysis) through
// the same decoder.
func DecodeOrderPush(data []byte) (futurestypes.OrderInfo, error) {
	var w orderPushWire
	var err error = codecUnmarshal(data, &w)
	if err != nil {
		return futurestypes.OrderInfo{}, err
	}
	return w.toOrderInfo(), nil
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
// matchSize/matchPrice/tradeId/liquidity are set on "match" frames only
// (empty string otherwise — hence flexDecimal). Sizes are in contracts.
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
	MatchSize  flexDecimal     `json:"matchSize"`
	MatchPrice flexDecimal     `json:"matchPrice"`
	Liquidity  string          `json:"liquidity"`
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
		EventType:     w.Type,
		TradeID:       w.TradeID,
		MatchSize:     w.MatchSize.Decimal,
		MatchPrice:    w.MatchPrice.Decimal,
		Liquidity:     futurestypes.Liquidity(w.Liquidity),
		TsNs:          w.Ts,
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
