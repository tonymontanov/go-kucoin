/*
FILE: internal/ws/conn_test.go

DESCRIPTION:
Connection-level tests for the KuCoin WS transport against an in-process
mock server (httptest + gorilla/websocket upgrader). Exercises the full
happy path — bullet token → dial → welcome → subscribe → ack → push —
plus heartbeat and reconnect/resubscribe.
*/

package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tonymontanov/go-kucoin/v2/internal/kclog"
)

// mockServer is a minimal KuCoin-like WS endpoint for tests.
type mockServer struct {
	srv      *httptest.Server
	upgrader websocket.Upgrader
	// connections counts how many sockets were accepted (reconnect proof).
	connections int64
	// pings counts ping frames observed across all connections.
	pings int64
	// dropAfterPush, when nonzero, makes the server close the socket right
	// after pushing the first data frame — forcing the client to reconnect.
	dropAfterPush int64
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	var m = &mockServer{}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

// wsURL returns the ws:// dial URL for the mock server.
func (m *mockServer) wsURL() string {
	return "ws" + strings.TrimPrefix(m.srv.URL, "http")
}

func (m *mockServer) handle(w http.ResponseWriter, r *http.Request) {
	var c *websocket.Conn
	var err error
	c, err = m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.Close() }()
	atomic.AddInt64(&m.connections, 1)

	// Welcome immediately, echoing the connectId from the query.
	var connectID string = r.URL.Query().Get("connectId")
	_ = c.WriteMessage(websocket.TextMessage, []byte(`{"id":"`+connectID+`","type":"welcome"}`))

	for {
		var raw []byte
		_, raw, err = c.ReadMessage()
		if err != nil {
			return
		}
		var env struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Topic string `json:"topic"`
		}
		if json.Unmarshal(raw, &env) != nil {
			continue
		}
		switch env.Type {
		case "ping":
			atomic.AddInt64(&m.pings, 1)
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"id":"`+env.ID+`","type":"pong"}`))
		case "subscribe":
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"id":"`+env.ID+`","type":"ack"}`))
			// Push one data frame on the subscribed topic.
			_ = c.WriteMessage(websocket.TextMessage,
				[]byte(`{"type":"message","topic":"`+env.Topic+`","subject":"level2","data":{"seq":42}}`))
			if atomic.LoadInt64(&m.dropAfterPush) != 0 {
				return
			}
		}
	}
}

// tokenProvider returns a TokenProvider pointing at the mock server.
func (m *mockServer) tokenProvider(ping time.Duration) TokenProvider {
	return func(_ context.Context) (TokenInfo, error) {
		return TokenInfo{Endpoint: m.wsURL(), Token: "test-token", PingInterval: ping}, nil
	}
}

func testConfig(tp TokenProvider) Config {
	return Config{
		TokenProvider:           tp,
		HandshakeTimeout:        2 * time.Second,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            time.Second,
		PingInterval:            50 * time.Millisecond,
		ReconnectInitialBackoff: 20 * time.Millisecond,
		ReconnectMaxBackoff:     100 * time.Millisecond,
		ReconnectJitter:         0,
		ReadBufferSize:          4096,
		WriteBufferSize:         4096,
	}
}

func TestConn_SubscribeReceivesPush(t *testing.T) {
	var m = newMockServer(t)
	var conn = NewConn(testConfig(m.tokenProvider(0)), nil, nil)
	defer func() { _ = conn.Close() }()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	conn.Start(ctx)

	var got = make(chan string, 1)
	var sub = &Subscription{
		Topic: "/contractMarket/level2:XBTUSDTM",
		Handler: func(topic, subject string, data []byte) {
			select {
			case got <- subject + "|" + string(data):
			default:
			}
		},
	}
	if err := conn.Subscribe(sub); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case v := <-got:
		var want string = `level2|{"seq":42}`
		if v != want {
			t.Fatalf("push = %q, want %q", v, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for push frame")
	}
}

func TestConn_PingPong(t *testing.T) {
	var m = newMockServer(t)
	// Fast server-driven ping via the token interval.
	var conn = NewConn(testConfig(m.tokenProvider(30*time.Millisecond)), nil, nil)
	defer func() { _ = conn.Close() }()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	conn.Start(ctx)

	// A subscribe forces the socket up promptly.
	_ = conn.Subscribe(&Subscription{
		Topic:   "/contractMarket/level2:XBTUSDTM",
		Handler: func(string, string, []byte) {},
	})

	// Wait long enough for several ping cycles.
	time.Sleep(250 * time.Millisecond)
	if atomic.LoadInt64(&m.pings) == 0 {
		t.Fatal("expected at least one ping frame at the server")
	}
}

func TestConn_SubscribeAfterClose(t *testing.T) {
	var m = newMockServer(t)
	var conn = NewConn(testConfig(m.tokenProvider(0)), nil, nil)
	conn.Start(context.Background())
	_ = conn.Close()

	var err = conn.Subscribe(&Subscription{
		Topic:   "/contractMarket/ticker:XBTUSDTM",
		Handler: func(string, string, []byte) {},
	})
	if err != ErrConnClosed {
		t.Fatalf("err = %v, want ErrConnClosed", err)
	}
}

func TestConn_ResubscribeOnReconnect(t *testing.T) {
	var m = newMockServer(t)
	atomic.StoreInt64(&m.dropAfterPush, 1)
	var conn = NewConn(testConfig(m.tokenProvider(0)), nil, nil)
	defer func() { _ = conn.Close() }()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	conn.Start(ctx)

	var pushes = make(chan struct{}, 16)
	var resetCount int64
	var sub = &Subscription{
		Topic:   "/contractMarket/level2:XBTUSDTM",
		Handler: func(string, string, []byte) { pushes <- struct{}{} },
		Reset:   func() { atomic.AddInt64(&resetCount, 1) },
	}
	if err := conn.Subscribe(sub); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// The server drops after each push, so the transport must reconnect and
	// resubscribe to produce a second push. Wait for two pushes.
	var i int
	for i = 0; i < 2; i++ {
		select {
		case <-pushes:
		case <-time.After(5 * time.Second):
			t.Fatalf("only got %d push(es); reconnect/resubscribe failed", i)
		}
	}

	if atomic.LoadInt64(&m.connections) < 2 {
		t.Fatalf("connections = %d, want >= 2 (reconnect)", atomic.LoadInt64(&m.connections))
	}
	// Reset must run before each (re)subscribe so the orderbook engine
	// discards stale state.
	if atomic.LoadInt64(&resetCount) < 2 {
		t.Fatalf("resetCount = %d, want >= 2", atomic.LoadInt64(&resetCount))
	}
}

// TestConn_ReadyBeforeFirstPushOnEachConnect locks the Ready contract: the
// hook fires once per (re)connect, after the subscribe op was sent, and
// strictly before the first push of that connection reaches the handler.
// The mock drops the socket after every push, so the observed sequence must
// alternate ready → push → ready → push.
func TestConn_ReadyBeforeFirstPushOnEachConnect(t *testing.T) {
	var m = newMockServer(t)
	atomic.StoreInt64(&m.dropAfterPush, 1)
	var conn = NewConn(testConfig(m.tokenProvider(0)), nil, nil)
	defer func() { _ = conn.Close() }()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	conn.Start(ctx)

	var mu sync.Mutex
	var seq []string
	var pushes = make(chan struct{}, 16)
	var sub = &Subscription{
		Topic: "/spotMarket/tradeOrders",
		Handler: func(string, string, []byte) {
			mu.Lock()
			seq = append(seq, "push")
			mu.Unlock()
			pushes <- struct{}{}
		},
		Ready: func() {
			mu.Lock()
			seq = append(seq, "ready")
			mu.Unlock()
		},
	}
	if err := conn.Subscribe(sub); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	var i int
	for i = 0; i < 2; i++ {
		select {
		case <-pushes:
		case <-time.After(5 * time.Second):
			t.Fatalf("only got %d push(es); reconnect/resubscribe failed", i)
		}
	}

	mu.Lock()
	var got []string = append([]string(nil), seq...)
	mu.Unlock()
	if len(got) < 4 || got[0] != "ready" {
		t.Fatalf("sequence = %v, want ready first and at least ready,push,ready,push", got)
	}
	var readies int
	for i = 0; i < len(got); i++ {
		if got[i] == "ready" {
			readies++
		}
		if got[i] == "push" && got[i-1] != "ready" {
			t.Fatalf("push at %d not preceded by ready: %v", i, got)
		}
	}
	if readies < 2 {
		t.Fatalf("ready fired %d time(s), want >= 2 (once per connection)", readies)
	}
}

// recordingLogger — kclog.Logger that counts lines per level and keeps the
// topics of warn lines.
type recordingLogger struct {
	mu     sync.Mutex
	debugs int
	warns  []string
}

func (l *recordingLogger) Debug(string, ...kclog.Field) { l.mu.Lock(); l.debugs++; l.mu.Unlock() }
func (l *recordingLogger) Info(string, ...kclog.Field)  {}
func (l *recordingLogger) Error(string, ...kclog.Field) {}
func (l *recordingLogger) Warn(_ string, f ...kclog.Field) {
	var topic string
	var i int
	for i = 0; i < len(f); i++ {
		if f[i].Key == "topic" {
			topic = f[i].Str
		}
	}
	l.mu.Lock()
	l.warns = append(l.warns, topic)
	l.mu.Unlock()
}

// v2.8.1: a push for a topic without a subscription is logged at Warn the
// first time per topic (a protocol surprise worth a human's eye) and at Debug
// afterwards (no flooding); the memory of topics is bounded.
func TestConn_UnknownTopicWarnOncePerTopic(t *testing.T) {
	var logger *recordingLogger = &recordingLogger{}
	var c *Conn = NewConn(testConfig(nil), logger, nil)
	c.noteUnknownTopic("/contractMarket/tradeOrders:XBTUSDTM")
	c.noteUnknownTopic("/contractMarket/tradeOrders:XBTUSDTM")
	c.noteUnknownTopic("/contractMarket/tradeOrders:ETHUSDTM")
	logger.mu.Lock()
	var warns []string = append([]string(nil), logger.warns...)
	var debugs int = logger.debugs
	logger.mu.Unlock()
	if len(warns) != 2 || warns[0] != "/contractMarket/tradeOrders:XBTUSDTM" || warns[1] != "/contractMarket/tradeOrders:ETHUSDTM" {
		t.Fatalf("want one warn per distinct topic, got %v", warns)
	}
	if debugs != 1 {
		t.Fatalf("repeat of a known topic must be debug: debugs=%d", debugs)
	}
	var i int
	for i = 0; i < unknownTopicsCap+5; i++ {
		c.noteUnknownTopic("/x/" + strconv.Itoa(i))
	}
	c.mu.RLock()
	var remembered int = len(c.unknownTopics)
	c.mu.RUnlock()
	if remembered != unknownTopicsCap {
		t.Fatalf("unknown-topic memory must stay bounded: %d", remembered)
	}
}
