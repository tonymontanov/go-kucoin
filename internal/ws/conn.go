/*
FILE: internal/ws/conn.go

DESCRIPTION:
Managing wrapper over a single KuCoin WebSocket connection. At most two
such objects are created per domain client (futures.Client, …): one for
the public endpoint (public bullet token, market-data topics) and one for
the private endpoint (private bullet token, order/account topics).

RESPONSIBILITIES:
  - obtain a fresh bullet token on every (re)connect via TokenProvider;
  - dial <endpoint>?token=…&connectId=… and wait for the {"type":"welcome"}
    greeting;
  - heartbeat: JSON {"id":…,"type":"ping"} on the server-driven ping
    interval (TokenInfo.PingInterval, falling back to cfg.PingInterval);
  - subscribe / unsubscribe with a registry keyed by topic string that
    survives reconnects;
  - resubscribe every registered topic after each successful (re)connect,
    transparently to the caller;
  - dispatch incoming push frames to the per-topic handler;
  - reconnect with exponential backoff + jitter;
  - graceful shutdown via Close() or ctx cancellation.

DESIGN NOTES (DIFFERENCES VS. BITGET WS):
  - No in-band login: privacy is a property of the bullet token. The
    private endpoint simply uses a TokenProvider backed by the SIGNED
    /api/v1/bullet-private REST call.
  - Topics are a single string ("/contractMarket/level2:XBTUSDTM"), not a
    multi-field arg. The registry is keyed by that string.
  - Heartbeat is a JSON ping/pong, not a plain-text frame.
  - The token is single-use / short-lived, so it is re-fetched on every
    connection attempt rather than cached on the Conn.

CONCURRENCY:
  - mu guards subs/socket/closed/cancel.
  - writeMu guards the underlying gorilla/websocket writes (gorilla
    requires exclusive writes).
  - Background goroutines (read-loop + ping-loop) are started afresh on
    every connect and torn down on every disconnect; the supervisor runs
    in its own goroutine for the entire lifetime of Conn.

ERROR STRATEGY:
  - Token / dial / read errors end the current connection; supervise
    reconnects after backoff.
  - Application-level error frames are logged and counted, but the
    supervisor keeps the connection alive — a transient backend hiccup
    must not permanently kill the feed.
  - On Conn.Close() the supervisor exits cleanly without reconnecting.

HANDLER CONTRACT (HOT PATH):
  Subscription.Handler is invoked synchronously from the read-loop
  goroutine. It MUST be O(1) and non-blocking — push work onto your own
  queue/ring-buffer and return. A blocking handler stalls EVERY topic
  multiplexed on the same socket.
*/

package ws

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
	"github.com/tonymontanov/go-kucoin/v2/internal/kcerr"
	"github.com/tonymontanov/go-kucoin/v2/internal/kclog"
	"github.com/tonymontanov/go-kucoin/v2/internal/kcmet"
)

// ErrConnClosed is returned by operations performed on a closed Conn.
var ErrConnClosed = errors.New("ws: connection closed")

// TokenInfo is the connection material produced by a bullet-token REST
// call: the chosen WS endpoint, the one-shot token, and the server's
// requested ping interval.
type TokenInfo struct {
	// Endpoint — wss endpoint selected from the bullet instanceServers.
	Endpoint string
	// Token — one-shot bullet token appended to the dial URL.
	Token string
	// PingInterval — server-requested client ping cadence. When zero the
	// Conn falls back to Config.PingInterval.
	PingInterval time.Duration
}

// TokenProvider fetches a fresh bullet token + endpoint. It is invoked on
// EVERY (re)connect because KuCoin tokens are short-lived / single-use.
// The futures profile implements it on top of the REST client (public or
// signed private bullet path). ctx bounds the REST call.
type TokenProvider func(ctx context.Context) (TokenInfo, error)

// Subscription describes a single KuCoin topic subscription. The caller
// (domain stream package) constructs it once and passes it to Subscribe;
// the same Subscription is reused on every reconnect via its Reset hook.
type Subscription struct {
	// Topic — full KuCoin topic string, e.g.
	// "/contractMarket/level2:XBTUSDTM". Required.
	Topic string
	// PrivateChannel — true for private topics (order/account); sets the
	// "privateChannel" flag on the subscribe op. Requires a private token.
	PrivateChannel bool
	// Handler is invoked for every push frame on this topic. Args:
	//   - topic   : the wire topic (lets one handler serve a topic family);
	//   - subject : the push subject (e.g. "level2", "ticker", "trade.l3match");
	//   - data    : the raw bytes of the "data" field (may be nil).
	// MUST be O(1) and non-blocking — see the HANDLER CONTRACT above.
	Handler func(topic, subject string, data []byte)
	// Reset is called once before every (re)subscribe. Used by the
	// orderbook engine to drop local state so the next snapshot is treated
	// as the new authoritative book. May be nil.
	Reset func()
}

// Config — parameters for a single KuCoin WS connection. Populated from the
// public root config via a field-by-field copy, plus the TokenProvider the
// domain profile wires.
type Config struct {
	// TokenProvider — REQUIRED. Supplies endpoint + token on each connect.
	TokenProvider TokenProvider
	// HandshakeTimeout — TLS+HTTP upgrade timeout. Also bounds the wait for
	// the welcome frame.
	HandshakeTimeout time.Duration
	// ReadTimeout — read deadline used to detect a silent server. Should be
	// >= 2 * ping interval so a single missed pong does not reconnect.
	ReadTimeout time.Duration
	// WriteTimeout — write deadline.
	WriteTimeout time.Duration
	// PingInterval — fallback client ping cadence used only when the bullet
	// response does not carry a server interval.
	PingInterval time.Duration
	// ReconnectInitialBackoff — first sleep after a connection failure.
	ReconnectInitialBackoff time.Duration
	// ReconnectMaxBackoff — cap for the exponential backoff.
	ReconnectMaxBackoff time.Duration
	// ReconnectJitter — random multiplier [1-j, 1+j] applied to backoff.
	// 0 disables jitter.
	ReconnectJitter float64
	// ReadBufferSize / WriteBufferSize — gorilla/websocket buffer sizes.
	ReadBufferSize  int
	WriteBufferSize int
}

// Conn — managing wrapper over a single KuCoin WS connection.
type Conn struct {
	cfg     Config
	logger  kclog.Logger
	metrics kcmet.CounterFactory

	mu     sync.RWMutex
	subs   map[string]*Subscription
	socket *websocket.Conn
	closed bool
	cancel context.CancelFunc

	writeMu sync.Mutex

	startOnce sync.Once

	cReceived  kcmet.Counter
	cDropped   kcmet.Counter
	cReconn    kcmet.Counter
	cSub       kcmet.Counter
	cPingErr   kcmet.Counter
	cTokenFail kcmet.Counter
}

// idSeq backs nextID; combined with the wall clock it yields ids unique
// within a process without pulling in a uuid dependency.
var idSeq uint64

// nextID returns a process-unique frame/connection id.
func nextID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" +
		strconv.FormatUint(atomic.AddUint64(&idSeq, 1), 10)
}

// NewConn creates a Conn. No network activity occurs until Start (or the
// first Subscribe) is called. log/mf may be nil.
func NewConn(cfg Config, log kclog.Logger, mf kcmet.CounterFactory) *Conn {
	if log == nil {
		log = kclog.Noop()
	}
	if mf == nil {
		mf = kcmet.Noop()
	}
	return &Conn{
		cfg:        cfg,
		logger:     log,
		metrics:    mf,
		subs:       make(map[string]*Subscription, 16),
		cReceived:  mf.Counter("kucoin_ws_messages_received_total"),
		cDropped:   mf.Counter("kucoin_ws_messages_dropped_total"),
		cReconn:    mf.Counter("kucoin_ws_reconnects_total"),
		cSub:       mf.Counter("kucoin_ws_subscriptions_total"),
		cPingErr:   mf.Counter("kucoin_ws_ping_failed_total"),
		cTokenFail: mf.Counter("kucoin_ws_token_failed_total"),
	}
}

// Start launches the background supervisor (idempotent). It returns
// immediately; the supervisor exits when ctx is cancelled or Close is
// called.
func (c *Conn) Start(ctx context.Context) {
	c.startOnce.Do(func() {
		var supCtx context.Context
		supCtx, c.cancel = context.WithCancel(ctx)
		go c.supervise(supCtx)
	})
}

// Subscribe registers a subscription and, if the socket is up, sends the
// subscribe op immediately. Otherwise the subscription waits in the
// registry and is sent automatically on the next successful (re)connect.
func (c *Conn) Subscribe(sub *Subscription) error {
	if sub == nil || sub.Topic == "" || sub.Handler == nil {
		return kcerr.New(kcerr.ErrorKindInvalidRequest, "", "ws: invalid subscription", nil)
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnClosed
	}
	c.subs[sub.Topic] = sub
	var socket *websocket.Conn = c.socket
	c.mu.Unlock()
	c.cSub.Inc()

	if socket == nil {
		return nil
	}
	return c.sendOp(socket, typeSubscribe, sub.Topic, sub.PrivateChannel)
}

// Unsubscribe removes the topic from the registry. If the socket is up, an
// unsubscribe op is sent. Idempotent — unknown topics return nil.
func (c *Conn) Unsubscribe(topic string) error {
	c.mu.Lock()
	var sub *Subscription = c.subs[topic]
	delete(c.subs, topic)
	var socket *websocket.Conn = c.socket
	c.mu.Unlock()
	if socket == nil || sub == nil {
		return nil
	}
	return c.sendOp(socket, typeUnsubscribe, topic, sub.PrivateChannel)
}

// Close stops the supervisor and the underlying socket. Idempotent.
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	var s *websocket.Conn = c.socket
	c.socket = nil
	c.mu.Unlock()

	if s != nil {
		_ = s.Close()
	}
	return nil
}

// supervise is the connect → run → backoff loop. Exits on ctx.Done.
func (c *Conn) supervise(ctx context.Context) {
	var backoff time.Duration = c.cfg.ReconnectInitialBackoff
	var attempt int
	for {
		if ctx.Err() != nil {
			return
		}
		var err error = c.connectAndRun(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.logger.Warn("ws: connection error, will reconnect",
				kclog.Int("attempt", int64(attempt)),
				kclog.Err(err),
			)
		}
		c.cReconn.Inc()
		attempt++

		var sleep time.Duration = applyJitter(backoff, c.cfg.ReconnectJitter)
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}
		backoff = nextBackoff(backoff, c.cfg.ReconnectMaxBackoff)
	}
}

// connectAndRun owns one full connection lifecycle: fetch token → dial →
// await welcome → resubscribe → read-loop + ping-loop.
func (c *Conn) connectAndRun(ctx context.Context) error {
	if c.cfg.TokenProvider == nil {
		return errors.New("ws: TokenProvider is nil")
	}

	var tokenCtx context.Context
	var tokenCancel context.CancelFunc
	tokenCtx, tokenCancel = context.WithTimeout(ctx, c.cfg.HandshakeTimeout)
	var info TokenInfo
	var err error
	info, err = c.cfg.TokenProvider(tokenCtx)
	tokenCancel()
	if err != nil {
		c.cTokenFail.Inc()
		return fmt.Errorf("bullet token: %w", err)
	}

	var dialURL string
	dialURL, err = buildDialURL(info.Endpoint, info.Token)
	if err != nil {
		return err
	}

	var dialer *websocket.Dialer = &websocket.Dialer{
		HandshakeTimeout: c.cfg.HandshakeTimeout,
		ReadBufferSize:   c.cfg.ReadBufferSize,
		WriteBufferSize:  c.cfg.WriteBufferSize,
	}
	var socket *websocket.Conn
	socket, _, err = dialer.DialContext(ctx, dialURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.logger.Info("ws: connected", kclog.Str("endpoint", info.Endpoint))

	defer func() {
		c.mu.Lock()
		if c.socket == socket {
			c.socket = nil
		}
		c.mu.Unlock()
		_ = socket.Close()
	}()

	if err = c.awaitWelcome(socket); err != nil {
		return fmt.Errorf("welcome: %w", err)
	}

	// Publish the socket and snapshot subscriptions, calling Reset BEFORE
	// the socket is visible so a stale push cannot race the engine reset.
	c.mu.Lock()
	c.socket = socket
	var subsCopy []*Subscription = make([]*Subscription, 0, len(c.subs))
	var s *Subscription
	for _, s = range c.subs {
		if s.Reset != nil {
			s.Reset()
		}
		subsCopy = append(subsCopy, s)
	}
	c.mu.Unlock()

	// Resubscribe everything that survived the previous disconnect. KuCoin
	// has no batch subscribe op — one frame per topic.
	var i int
	for i = 0; i < len(subsCopy); i++ {
		if err = c.sendOp(socket, typeSubscribe, subsCopy[i].Topic, subsCopy[i].PrivateChannel); err != nil {
			c.logger.Warn("ws: resubscribe failed",
				kclog.Str("topic", subsCopy[i].Topic), kclog.Err(err))
		}
	}

	var pingInterval time.Duration = info.PingInterval
	if pingInterval <= 0 {
		pingInterval = c.cfg.PingInterval
	}

	var loopCtx context.Context
	var loopCancel context.CancelFunc
	loopCtx, loopCancel = context.WithCancel(ctx)
	defer loopCancel()

	var wg sync.WaitGroup
	wg.Add(2)
	var readErr error
	go func() {
		defer wg.Done()
		defer loopCancel()
		readErr = c.readLoop(loopCtx, socket)
	}()
	go func() {
		defer wg.Done()
		c.pingLoop(loopCtx, socket, pingInterval)
	}()
	wg.Wait()

	if readErr != nil {
		return readErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// awaitWelcome blocks until the server's {"type":"welcome"} frame arrives
// or the handshake deadline elapses. KuCoin sends it immediately on a
// healthy socket; anything else (pong/message) before welcome is skipped.
func (c *Conn) awaitWelcome(socket *websocket.Conn) error {
	var deadline time.Time = time.Now().Add(c.cfg.HandshakeTimeout)
	_ = socket.SetReadDeadline(deadline)
	defer func() { _ = socket.SetReadDeadline(time.Time{}) }()

	var i int
	for i = 0; i < 10; i++ {
		var msgType int
		var raw []byte
		var err error
		msgType, raw, err = socket.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.TextMessage {
			continue
		}
		var env Envelope
		if err = codec.Unmarshal(raw, &env); err != nil {
			c.logger.Debug("ws: unparseable frame before welcome", kclog.Err(err))
			continue
		}
		switch env.Type {
		case typeWelcome:
			c.logger.Debug("ws: welcome", kclog.Str("id", env.ID))
			return nil
		case typeError:
			return fmt.Errorf("server error before welcome: code=%s", env.Code.String())
		default:
			continue
		}
	}
	return errors.New("ws: welcome not received")
}

// readLoop reads frames and dispatches push frames to subscription
// handlers. Returns the read error so supervise can decide on reconnect.
func (c *Conn) readLoop(ctx context.Context, socket *websocket.Conn) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		_ = socket.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		var msgType int
		var raw []byte
		var err error
		msgType, raw, err = socket.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.TextMessage {
			continue
		}
		c.cReceived.Inc()

		var env Envelope
		if err = codec.Unmarshal(raw, &env); err != nil {
			c.cDropped.Inc()
			c.logger.Warn("ws: failed to parse envelope", kclog.Err(err))
			continue
		}

		if env.IsPush() {
			c.mu.RLock()
			var sub *Subscription = c.subs[env.Topic]
			c.mu.RUnlock()
			if sub == nil {
				c.cDropped.Inc()
				c.logger.Debug("ws: push for unknown topic", kclog.Str("topic", env.Topic))
				continue
			}
			sub.Handler(env.Topic, env.Subject, env.Data)
			continue
		}

		c.handleControl(&env)
	}
}

// handleControl logs ack / pong / error / welcome frames that reach the
// read-loop. Subscribe acks are informational — the supervisor does not
// gate on them.
func (c *Conn) handleControl(env *Envelope) {
	switch env.Type {
	case typeAck:
		c.logger.Debug("ws: ack", kclog.Str("id", env.ID))
	case typePong:
		// Heartbeat reply — nothing to do; the read deadline already
		// advanced when this frame arrived.
	case typeWelcome:
		c.logger.Debug("ws: welcome (mid-stream)", kclog.Str("id", env.ID))
	case typeError:
		c.logger.Warn("ws: server error",
			kclog.Str("id", env.ID),
			kclog.Str("code", env.Code.String()),
		)
	default:
		c.logger.Debug("ws: control", kclog.Str("type", env.Type))
	}
}

// pingLoop sends a JSON {"type":"ping"} frame on a ticker. Exits on the
// first write error; the read-loop will fail too and supervise reconnects.
func (c *Conn) pingLoop(ctx context.Context, socket *websocket.Conn, interval time.Duration) {
	if interval <= 0 {
		return
	}
	var ticker *time.Ticker = time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var op outboundOp = outboundOp{ID: nextID(), Type: typePing}
			var raw []byte
			var err error
			raw, err = codec.Marshal(op)
			if err != nil {
				return
			}
			if err = c.writeFrame(socket, raw); err != nil {
				c.cPingErr.Inc()
				c.logger.Debug("ws: ping write failed", kclog.Err(err))
				return
			}
		}
	}
}

// sendOp marshals a subscribe/unsubscribe op for a single topic and writes
// it. response:true asks KuCoin to confirm with an ack frame.
func (c *Conn) sendOp(socket *websocket.Conn, opType, topic string, private bool) error {
	var msg outboundOp = outboundOp{
		ID:             nextID(),
		Type:           opType,
		Topic:          topic,
		PrivateChannel: private,
		Response:       true,
	}
	var raw []byte
	var err error
	raw, err = codec.Marshal(msg)
	if err != nil {
		return err
	}
	return c.writeFrame(socket, raw)
}

// writeFrame is a thread-safe text-frame write. gorilla/websocket requires
// exclusive writes — the dedicated mutex keeps ping/sub from colliding.
func (c *Conn) writeFrame(socket *websocket.Conn, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = socket.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	return socket.WriteMessage(websocket.TextMessage, data)
}

// buildDialURL appends the token and a fresh connectId to the bullet
// endpoint. KuCoin requires both query params on the dial URL.
func buildDialURL(endpoint, token string) (string, error) {
	if endpoint == "" {
		return "", kcerr.New(kcerr.ErrorKindInvalidRequest, "", "ws: empty bullet endpoint", nil)
	}
	var u *url.URL
	var err error
	u, err = url.Parse(endpoint)
	if err != nil {
		return "", kcerr.New(kcerr.ErrorKindInvalidRequest, "", "ws: invalid bullet endpoint", err)
	}
	var q url.Values = u.Query()
	q.Set("token", token)
	q.Set("connectId", nextID())
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// nextBackoff doubles cur, capping at max.
func nextBackoff(cur, max time.Duration) time.Duration {
	cur *= 2
	if cur > max {
		cur = max
	}
	return cur
}

// applyJitter multiplies d by a random factor in [1-j, 1+j].
func applyJitter(d time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return d
	}
	var f float64 = 1.0 + (rand.Float64()*2.0-1.0)*jitter
	return time.Duration(float64(d) * f)
}
