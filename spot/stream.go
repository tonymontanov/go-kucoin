/*
FILE: spot/stream.go

DESCRIPTION:
Public WebSocket sub-client for the KuCoin Spot profile. Wires the
internal/ws bullet-token transport to the public market-data topics and
exposes callback-based Watch* methods.

CONNECTION:
The public connection uses a public bullet token (POST /api/v1/bullet-public
on the spot host, no auth). The private connection (see stream-private.go)
uses the signed private bullet. Both are created lazily on first use and
survive reconnects transparently (the transport resubscribes every
registered topic).

TOPICS (public):
  - /market/level2:{symbol}          incremental order book (managed)
  - /market/ticker:{symbol}          level1 ticker (last + best bid/ask)
  - /market/match:{symbol}           public trade tape
  - /market/candles:{symbol}_{type}  klines

HANDLER CONTRACT (HOT PATH):
Handlers run synchronously on the read-loop goroutine. They MUST be O(1) and
non-blocking — copy what you need and return.

ORDER BOOK (spot level2 — sequence model):
The spot level2 push carries a sequenceStart/sequenceEnd range and a
`changes` object with [price, size, sequence] entries per side. Each entry's
sequence is globally monotonic and contiguous, so WatchOrderBook seeds from
the REST level2_100 snapshot, applies the per-entry changes in sequence order
through the shared sequence engine, detects gaps and re-seeds automatically.
*/

package spot

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/kccommon/orderbook"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	"github.com/tonymontanov/go-kucoin/v2/internal/ws"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// pendingChangeCap bounds the level2 change buffer held while the engine
// re-seeds from REST. A burst beyond this forces a fresh re-seed.
const pendingChangeCap = 4096

// StreamClient — WebSocket subscription sub-client (public + private).
type StreamClient struct {
	c *Client

	mu          sync.Mutex
	publicConn  *ws.Conn
	privateConn *ws.Conn
}

// newStreamClient wires the sub-client to its parent.
func newStreamClient(c *Client) *StreamClient {
	return &StreamClient{c: c}
}

// Close tears down both WS connections. Safe to call multiple times.
func (s *StreamClient) Close() error {
	s.mu.Lock()
	var pub, prv *ws.Conn = s.publicConn, s.privateConn
	s.mu.Unlock()
	if pub != nil {
		_ = pub.Close()
	}
	if prv != nil {
		_ = prv.Close()
	}
	return nil
}

// ---------------------------------------------------------------------
// Connection management.
// ---------------------------------------------------------------------

// ensurePublic lazily creates and starts the public connection bound to ctx.
func (s *StreamClient) ensurePublic(ctx context.Context) *ws.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.publicConn == nil {
		s.publicConn = ws.NewConn(s.wsConfig(s.publicTokenProvider), s.c.logger(), s.c.config().Metrics)
		s.publicConn.Start(ctx)
	}
	return s.publicConn
}

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

// wsConfig copies the public WS tuning into an internal/ws Config and binds
// the supplied bullet-token provider.
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

// publicTokenProvider fetches a public bullet token + endpoint.
func (s *StreamClient) publicTokenProvider(ctx context.Context) (ws.TokenInfo, error) {
	return s.fetchBullet(ctx, "/api/v1/bullet-public", false)
}

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

// bulletWire mirrors the /api/v1/bullet-{public,private} response data.
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
// Public watch methods.
// ---------------------------------------------------------------------

// WatchOrderBook subscribes to the managed level-2 order book for a symbol.
// The handler receives a consistent snapshot after every applied frame; the
// SDK seeds from REST (level2_100), reconciles by sequence and re-seeds on a
// gap.
func (s *StreamClient) WatchOrderBook(ctx context.Context, symbol string, handler func(*roottypes.OrderBookSnapshot)) error {
	if symbol == "" || handler == nil {
		return errInvalidRequest("WatchOrderBook", "symbol and handler are required")
	}
	var conn *ws.Conn = s.ensurePublic(ctx)
	var bm *bookManager = newBookManager(s.c, ctx, symbol, handler)
	return conn.Subscribe(&ws.Subscription{
		Topic:   "/market/level2:" + symbol,
		Handler: bm.onMessage,
		Reset:   bm.onReset,
	})
}

// WatchTicker subscribes to the real-time ticker for a symbol.
func (s *StreamClient) WatchTicker(ctx context.Context, symbol string, handler func(*spottypes.MarketTicker)) error {
	if symbol == "" || handler == nil {
		return errInvalidRequest("WatchTicker", "symbol and handler are required")
	}
	var conn *ws.Conn = s.ensurePublic(ctx)
	return conn.Subscribe(&ws.Subscription{
		Topic: "/market/ticker:" + symbol,
		Handler: func(_, _ string, data []byte) {
			var w tickerPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			var t spottypes.MarketTicker = w.toTicker(symbol)
			handler(&t)
		},
	})
}

// WatchTrades subscribes to the public trade tape for a symbol.
func (s *StreamClient) WatchTrades(ctx context.Context, symbol string, handler func(*roottypes.TradeUpdate)) error {
	if symbol == "" || handler == nil {
		return errInvalidRequest("WatchTrades", "symbol and handler are required")
	}
	var conn *ws.Conn = s.ensurePublic(ctx)
	return conn.Subscribe(&ws.Subscription{
		Topic: "/market/match:" + symbol,
		Handler: func(_, _ string, data []byte) {
			var w matchPushWire
			if codecUnmarshal(data, &w) != nil {
				return
			}
			var tr roottypes.TradeUpdate = w.toTradeUpdate(symbol)
			handler(&tr)
		},
	})
}

// WatchKlines subscribes to the candle stream for a symbol + timeframe. The
// handler receives the in-progress candle on each update and the previous
// candle once with Confirmed=true at the interval rollover.
func (s *StreamClient) WatchKlines(ctx context.Context, symbol string, tf spottypes.Timeframe, handler func(*roottypes.KlineUpdate)) error {
	if symbol == "" || handler == nil {
		return errInvalidRequest("WatchKlines", "symbol and handler are required")
	}
	var gran string = spotGranularity[tf]
	if gran == "" {
		return errInvalidRequest("WatchKlines", "unsupported timeframe")
	}
	var conn *ws.Conn = s.ensurePublic(ctx)
	var km *klineManager = &klineManager{symbol: symbol, tf: tf, handler: handler}
	return conn.Subscribe(&ws.Subscription{
		Topic:   "/market/candles:" + symbol + "_" + gran,
		Handler: km.onMessage,
		Reset:   km.onReset,
	})
}

// Unsubscribe removes a public subscription by full topic string.
func (s *StreamClient) Unsubscribe(topic string) error {
	s.mu.Lock()
	var conn *ws.Conn = s.publicConn
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Unsubscribe(topic)
}

// ---------------------------------------------------------------------
// Managed order book (spot level2 — multi-change frames).
// ---------------------------------------------------------------------

// bookManager owns the local order book for one symbol: it seeds from REST,
// applies the level2 change stream, and re-seeds on a sequence gap.
type bookManager struct {
	c       *Client
	ctx     context.Context
	symbol  string
	engine  *orderbook.Engine
	handler func(*roottypes.OrderBookSnapshot)

	mu      sync.Mutex
	seeded  bool
	seeding bool
	pending []pendingChange
}

// pendingChange is a single level2 change buffered while the engine re-seeds.
type pendingChange struct {
	side  string
	price decimal.Decimal
	size  decimal.Decimal
	seq   int64
}

func newBookManager(c *Client, ctx context.Context, symbol string, handler func(*roottypes.OrderBookSnapshot)) *bookManager {
	return &bookManager{
		c:       c,
		ctx:     ctx,
		symbol:  symbol,
		engine:  orderbook.NewEngine(symbol, c.config().Orderbook.MaxDepth),
		handler: handler,
	}
}

// onReset is invoked by the transport before every (re)subscribe. It drops
// engine state and schedules an async re-seed.
func (b *bookManager) onReset() {
	b.mu.Lock()
	b.engine.Reset()
	b.seeded = false
	b.pending = b.pending[:0]
	var launch bool = !b.seeding
	if launch {
		b.seeding = true
	}
	b.mu.Unlock()
	if launch {
		go b.reseed()
	}
}

// onMessage applies one level2 frame (which carries multiple per-side
// changes). Changes are sorted by their own sequence so the contiguous
// engine applies them in order.
func (b *bookManager) onMessage(_, _ string, data []byte) {
	var w level2PushWire
	if codecUnmarshal(data, &w) != nil {
		return
	}
	var changes []pendingChange = w.changes()
	if len(changes) == 0 {
		return
	}

	b.mu.Lock()
	if !b.seeded {
		var room int = pendingChangeCap - len(b.pending)
		if room > 0 {
			if len(changes) > room {
				changes = changes[:room]
			}
			b.pending = append(b.pending, changes...)
		}
		b.mu.Unlock()
		return
	}
	var needReseed bool
	var i int
	for i = 0; i < len(changes); i++ {
		var p pendingChange = changes[i]
		var err error = b.engine.ApplyChange(p.side, p.price, p.size, p.seq, w.timeMs())
		if err == orderbook.ErrGap {
			needReseed = true
			break
		}
	}
	if needReseed {
		b.seeded = false
		b.pending = b.pending[:0]
		var launch bool = !b.seeding
		if launch {
			b.seeding = true
		}
		b.mu.Unlock()
		if launch {
			go b.reseed()
		}
		return
	}
	var snap roottypes.OrderBookSnapshot = b.engine.Snapshot()
	b.mu.Unlock()
	b.handler(&snap)
}

// reseed fetches the REST snapshot, installs it, replays buffered changes
// and resumes live application.
func (b *bookManager) reseed() {
	defer func() {
		b.mu.Lock()
		b.seeding = false
		b.mu.Unlock()
	}()

	// Prefer the FULL-depth signed snapshot when credentials are configured
	// (market-making needs the whole book); fall back to the public
	// level2_100 otherwise. Both share the sequence space, so either seeds
	// the same engine.
	var snap *roottypes.OrderBookSnapshot
	var err error
	if b.c.signerEnabled() {
		snap, err = b.c.MarketData().GetOrderBookFull(b.ctx, b.symbol)
	} else {
		snap, err = b.c.MarketData().GetOrderBook(b.ctx, b.symbol)
	}
	if err != nil {
		b.c.logger().Warn("ws: orderbook reseed failed", logStr("symbol", b.symbol), logErr(err))
		return
	}

	b.engine.ApplySnapshot(levelsToEngine(snap.Asks), levelsToEngine(snap.Bids), snap.Sequence, snap.TsMs)

	b.mu.Lock()
	b.seeded = true
	var i int
	for i = 0; i < len(b.pending); i++ {
		var p pendingChange = b.pending[i]
		_ = b.engine.ApplyChange(p.side, p.price, p.size, p.seq, snap.TsMs)
	}
	b.pending = b.pending[:0]
	var out roottypes.OrderBookSnapshot = b.engine.Snapshot()
	b.mu.Unlock()
	b.handler(&out)
}

// levelsToEngine converts root book levels into engine levels.
func levelsToEngine(src []roottypes.OrderBookLevel) []orderbook.Level {
	var out []orderbook.Level = make([]orderbook.Level, len(src))
	var i int
	for i = 0; i < len(src); i++ {
		out[i] = orderbook.Level{Price: src[i].Price, Size: src[i].Size}
	}
	return out
}

// sortPendingBySeq orders changes ascending by sequence — required because a
// spot frame interleaves ask/bid changes whose sequences are globally
// monotonic but not array-ordered.
func sortPendingBySeq(c []pendingChange) {
	sort.Slice(c, func(i, j int) bool { return c[i].seq < c[j].seq })
}

// ---------------------------------------------------------------------
// Managed klines (Confirmed flag at rollover).
// ---------------------------------------------------------------------

// klineManager tracks the last candle start per subscription so it can mark
// the previous candle Confirmed when the interval rolls over.
type klineManager struct {
	symbol  string
	tf      spottypes.Timeframe
	handler func(*roottypes.KlineUpdate)

	mu        sync.Mutex
	lastStart int64
	last      roottypes.KlineUpdate
	hasLast   bool
}

func (k *klineManager) onReset() {
	k.mu.Lock()
	k.lastStart = 0
	k.hasLast = false
	k.mu.Unlock()
}

func (k *klineManager) onMessage(_, _ string, data []byte) {
	var w candlePushWire
	if codecUnmarshal(data, &w) != nil {
		return
	}
	var ku roottypes.KlineUpdate
	var ok bool
	ku, ok = w.toKlineUpdate(k.symbol, k.tf)
	if !ok {
		return
	}

	k.mu.Lock()
	var emitPrev bool
	var prev roottypes.KlineUpdate
	if k.hasLast && ku.StartMs > k.lastStart {
		prev = k.last
		prev.Confirmed = true
		emitPrev = true
	}
	k.last = ku
	k.lastStart = ku.StartMs
	k.hasLast = true
	k.mu.Unlock()

	if emitPrev {
		k.handler(&prev)
	}
	k.handler(&ku)
}
