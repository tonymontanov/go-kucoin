/*
FILE: margin/stream.go

DESCRIPTION:
Private WebSocket sub-client for the KuCoin Margin profile. Margin trades on
the SPOT matching engine, so:

  - the PUBLIC market data (order book / ticker / trade tape / candles) is
    identical to Spot and is NOT duplicated here — use the spot profile's
    Stream for those (same topics, same data);
  - this sub-client exposes the PRIVATE margin order lifecycle over the
    spot/margin private channel (/spotMarket/tradeOrders). Margin orders carry
    tradeType "MARGIN_TRADE"/"MARGIN_ISOLATED_TRADE", so callers filter by
    OrderInfo.TradeType when sharing the channel with spot orders.

CONNECTION:
The private connection uses a signed private bullet token
(POST /api/v1/bullet-private on the spot host). It is created lazily on first
use and survives reconnects transparently (the transport resubscribes every
registered topic).

HANDLER CONTRACT (HOT PATH):
Handlers run synchronously on the read-loop goroutine. They MUST be O(1) and
non-blocking — copy what you need and return.
*/

package margin

import (
	"context"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	"github.com/tonymontanov/go-kucoin/v2/internal/ws"
)

// StreamClient — WebSocket subscription sub-client (private margin).
type StreamClient struct {
	c *Client

	mu          sync.Mutex
	privateConn *ws.Conn
}

// newStreamClient wires the sub-client to its parent.
func newStreamClient(c *Client) *StreamClient {
	return &StreamClient{c: c}
}

// Close tears down the private WS connection. Safe to call multiple times.
func (s *StreamClient) Close() error {
	s.mu.Lock()
	var prv *ws.Conn = s.privateConn
	s.mu.Unlock()
	if prv != nil {
		_ = prv.Close()
	}
	return nil
}

// ---------------------------------------------------------------------
// Connection management.
// ---------------------------------------------------------------------

// ensurePrivate lazily creates and starts the private connection bound to
// ctx. Returns nil + error when no credentials are configured.
func (s *StreamClient) ensurePrivate(ctx context.Context) (*ws.Conn, error) {
	if !s.c.signerEnabled() {
		return nil, errAuthRequired("Stream private")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.privateConn == nil {
		s.privateConn = ws.NewConn(s.wsConfig(s.privateTokenProvider), s.c.logger(), s.c.config().Metrics)
		s.privateConn.Start(ctx)
	}
	return s.privateConn, nil
}

// wsConfig copies the WS tuning into an internal/ws Config and binds the
// supplied bullet-token provider.
func (s *StreamClient) wsConfig(tp ws.TokenProvider) ws.Config {
	var w = s.c.config().WS
	return ws.Config{
		TokenProvider:           tp,
		HandshakeTimeout:        w.HandshakeTimeout,
		ReadTimeout:             w.ReadTimeout,
		WriteTimeout:            w.WriteTimeout,
		PingInterval:            w.PingInterval,
		ReconnectInitialBackoff: w.ReconnectInitialBackoff,
		ReconnectMaxBackoff:     w.ReconnectMaxBackoff,
		ReconnectJitter:         w.ReconnectJitter,
		ReadBufferSize:          w.ReadBufferSize,
		WriteBufferSize:         w.WriteBufferSize,
	}
}

// bulletMeta is the rate-limit metadata stamped on bullet-token calls.
var bulletMeta = rest.RequestMeta{Category: "market"}

// privateTokenProvider fetches a signed private bullet token + endpoint.
func (s *StreamClient) privateTokenProvider(ctx context.Context) (ws.TokenInfo, error) {
	return s.fetchBullet(ctx, "/api/v1/bullet-private", true)
}

// fetchBullet POSTs to a bullet endpoint and maps the response into a
// ws.TokenInfo. KuCoin returns a list of instanceServers; the first is used.
func (s *StreamClient) fetchBullet(ctx context.Context, path string, signed bool) (ws.TokenInfo, error) {
	var info ws.TokenInfo
	var resp rest.Response
	var err error
	resp, _, err = s.c.rest().Do(ctx, rest.Options{Method: "POST", Path: path, Signed: signed, Meta: bulletMeta})
	if err != nil {
		return info, err
	}
	var wire bulletWire
	if err = resp.UnmarshalData(&wire); err != nil {
		return info, err
	}
	if wire.Token == "" || len(wire.InstanceServers) == 0 {
		return info, errInvalidRequest("Stream bullet", "empty bullet token / instanceServers")
	}
	var srv = wire.InstanceServers[0]
	info.Endpoint = srv.Endpoint
	info.Token = wire.Token
	if srv.PingInterval > 0 {
		info.PingInterval = time.Duration(srv.PingInterval) * time.Millisecond
	}
	return info, nil
}

// bulletWire mirrors the /api/v1/bullet-private response data.
type bulletWire struct {
	Token           string `json:"token"`
	InstanceServers []struct {
		Endpoint     string `json:"endpoint"`
		PingInterval int64  `json:"pingInterval"`
		PingTimeout  int64  `json:"pingTimeout"`
		Encrypt      bool   `json:"encrypt"`
		Protocol     string `json:"protocol"`
	} `json:"instanceServers"`
}

// ---------------------------------------------------------------------
// Private watch methods.
// ---------------------------------------------------------------------

// WatchOrders subscribes to account-wide order lifecycle updates over the
// spot/margin private channel. Margin orders carry TradeType
// MARGIN_TRADE/MARGIN_ISOLATED_TRADE — filter on OrderInfo.TradeType when the
// account also trades spot on the same channel.
func (s *StreamClient) WatchOrders(ctx context.Context, handler func(*margintypes.OrderInfo)) error {
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
			var o margintypes.OrderInfo = w.toOrderInfo()
			handler(&o)
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

/*
flexInt64 decodes an integer that KuCoin ships INCONSISTENTLY as either a
bare JSON number or a quoted string across the spot/margin private channels.
A plain int64 field rejects the quoted form and, because the decoder fails on
the WHOLE frame, the push is silently dropped. flexInt64 accepts both shapes
so the frame always decodes (same hardening as the spot profile).
*/
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
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

// orderPushWire mirrors a /spotMarket/tradeOrders push for a margin order.
// `type` is the lifecycle event (received/open/match/filled/canceled/update);
// `status` is the resulting order state (open/match/done). orderTime/ts are
// nanoseconds. tradeType distinguishes cross/isolated margin from spot.
type orderPushWire struct {
	OrderID    string          `json:"orderId"`
	Symbol     string          `json:"symbol"`
	OrderType  string          `json:"orderType"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	Side       string          `json:"side"`
	TradeType  string          `json:"tradeType"`
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

func (w orderPushWire) toOrderInfo() margintypes.OrderInfo {
	return margintypes.OrderInfo{
		OrderID:       w.OrderID,
		ClientOrderID: w.ClientOid,
		Symbol:        w.Symbol,
		Type:          margintypes.OrderType(w.OrderType),
		Side:          margintypes.SideType(w.Side),
		TradeType:     margintypes.TradeType(w.TradeType),
		Price:         w.Price,
		Size:          w.Size,
		Funds:         w.Funds,
		DealSize:      w.FilledSize,
		Status:        margintypes.OrderStatus(w.Status),
		IsActive:      w.Status == "open" || w.Status == "match",
		UpdatedAtMs:   nsToMs(w.Ts.int64()),
		CreatedAtMs:   nsToMs(w.OrderTime.int64()),
	}
}
