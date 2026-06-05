/*
FILE: futures/contract_ws_test.go

DESCRIPTION:
Offline contract tests for the public WS dispatch path. A single mock server
plays three roles: it serves the public bullet token (pointing the WS
endpoint back at itself), serves the REST level2 snapshot used to seed the
managed order book, and upgrades /ws to a WebSocket that greets with
"welcome", acks subscriptions and pushes recorded frames per topic.

The tests drive the real StreamClient end-to-end and assert the typed
structs handed to the callbacks — covering bullet-token plumbing, the
welcome/subscribe handshake, frame dispatch and the wire→type converters,
including the managed order-book seed + sequence apply.
*/

package futures

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// newMockStreamServer returns an httptest server that serves bullet + REST
// snapshot + a WS endpoint, and a futures.Client wired to it.
func newMockStreamServer(t *testing.T) (*Client, context.CancelFunc) {
	t.Helper()
	var upgrader = websocket.Upgrader{}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v1/bullet-public", func(w http.ResponseWriter, r *http.Request) {
		var endpoint = "ws://" + r.Host + "/ws"
		writeEnv(w, `{"token":"tkn","instanceServers":[{"endpoint":"`+endpoint+`","pingInterval":10000,"pingTimeout":5000,"encrypt":false,"protocol":"websocket"}]}`)
	})
	mux.HandleFunc("/api/v1/level2/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLevel2) // sequence 1000
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		var conn, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"c","type":"welcome"}`))
		for {
			var _, raw, rerr = conn.ReadMessage()
			if rerr != nil {
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
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"`+env.ID+`","type":"pong"}`))
			case "subscribe":
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"`+env.ID+`","type":"ack"}`))
				pushForTopic(conn, env.Topic)
			}
		}
	})

	var srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var cfg = kucoin.DefaultConfig()
	cfg.REST.BaseURL = srv.URL
	var parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var _, cancel = context.WithCancel(context.Background())
	return NewClient(parent), cancel
}

// pushForTopic writes a recorded push frame matching the subscribed topic.
func pushForTopic(conn *websocket.Conn, topic string) {
	switch {
	case strings.HasPrefix(topic, "/contractMarket/level2:"):
		// New best bid at 50000.5 (snapshot best bid was 50000).
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"level2","data":{"sequence":1001,"change":"50000.5,buy,9","timestamp":1700000000111}}`))
	case strings.HasPrefix(topic, "/contractMarket/ticker:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"ticker","data":{"sequence":2,"side":"buy","size":4,"price":"50000.5","bestBidSize":11,"bestBidPrice":"50000.0","bestAskPrice":"50001.0","bestAskSize":6,"tradeId":"tt","ts":1700000000000000000}}`))
	case strings.HasPrefix(topic, "/contractMarket/execution:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"match","data":{"sequence":3,"side":"sell","size":7,"price":"50001.0","tradeId":"e1","ts":1700000000000000000}}`))
	case strings.HasPrefix(topic, "/contractMarket/candle:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"candle.stick","data":{"symbol":"XBTUSDTM","candles":[1700000000,100,110,90,105,1234,5678],"time":1700000000000000000}}`))
	case strings.HasPrefix(topic, "/contract/instrument:"):
		// A funding.rate frame first (must be ignored), then mark.index.price.
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"funding.rate","data":{"granularity":60000,"fundingRate":-0.0001,"timestamp":1700000000000}}`))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"mark.index.price","data":{"granularity":1000,"indexPrice":"50010.0","markPrice":"50012.5","timestamp":1700000000123}}`))
	}
}

func TestContractWS_OrderBookManaged(t *testing.T) {
	var c, cancel = newMockStreamServer(t)
	defer cancel()
	defer func() { _ = c.Stream().Close() }()

	var ctx, cctx = context.WithTimeout(context.Background(), 5*time.Second)
	defer cctx()

	var got = make(chan *roottypes.OrderBookSnapshot, 8)
	var err = c.Stream().WatchOrderBook(ctx, "XBTUSDTM", func(ob *roottypes.OrderBookSnapshot) {
		got <- ob
	})
	if err != nil {
		t.Fatalf("WatchOrderBook: %v", err)
	}

	// Wait for a snapshot reflecting the applied change (best bid 50000.5).
	var deadline = time.After(5 * time.Second)
	for {
		select {
		case ob := <-got:
			if len(ob.Bids) > 0 && ob.Bids[0].Price.String() == "50000.5" && ob.Sequence == 1001 {
				return
			}
		case <-deadline:
			t.Fatal("did not receive a reconciled snapshot with the applied change")
		}
	}
}

func TestContractWS_Ticker(t *testing.T) {
	var c, cancel = newMockStreamServer(t)
	defer cancel()
	defer func() { _ = c.Stream().Close() }()

	var ctx, cctx = context.WithTimeout(context.Background(), 5*time.Second)
	defer cctx()

	var got = make(chan *futurestypes.MarketTicker, 1)
	if err := c.Stream().WatchTicker(ctx, "XBTUSDTM", func(tk *futurestypes.MarketTicker) {
		select {
		case got <- tk:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchTicker: %v", err)
	}

	select {
	case tk := <-got:
		if tk.Symbol != "XBTUSDTM" || tk.BestBidPrice.String() != "50000" || tk.TsMs != 1700000000000 {
			t.Fatalf("ticker = %+v", tk)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ticker push")
	}
}

func TestContractWS_Trades(t *testing.T) {
	var c, cancel = newMockStreamServer(t)
	defer cancel()
	defer func() { _ = c.Stream().Close() }()

	var ctx, cctx = context.WithTimeout(context.Background(), 5*time.Second)
	defer cctx()

	var got = make(chan *roottypes.TradeUpdate, 1)
	if err := c.Stream().WatchTrades(ctx, "XBTUSDTM", func(tr *roottypes.TradeUpdate) {
		select {
		case got <- tr:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchTrades: %v", err)
	}

	select {
	case tr := <-got:
		if tr.Side != "sell" || tr.Price.String() != "50001" || tr.TsMs != 1700000000000 {
			t.Fatalf("trade = %+v", tr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no trade push")
	}
}

func TestContractWS_Instrument(t *testing.T) {
	var c, cancel = newMockStreamServer(t)
	defer cancel()
	defer func() { _ = c.Stream().Close() }()

	var ctx, cctx = context.WithTimeout(context.Background(), 5*time.Second)
	defer cctx()

	var got = make(chan *futurestypes.MarkPrice, 1)
	if err := c.Stream().WatchInstrument(ctx, "XBTUSDTM", func(mp *futurestypes.MarkPrice) {
		select {
		case got <- mp:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchInstrument: %v", err)
	}

	select {
	case mp := <-got:
		// The funding.rate frame must have been filtered out; we only get
		// the mark.index.price one.
		if mp.Symbol != "XBTUSDTM" || mp.Value.String() != "50012.5" || mp.IndexPrice.String() != "50010" || mp.TimePointMs != 1700000000123 {
			t.Fatalf("mark price = %+v", mp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no instrument push")
	}
}

func TestContractWS_Klines(t *testing.T) {
	var c, cancel = newMockStreamServer(t)
	defer cancel()
	defer func() { _ = c.Stream().Close() }()

	var ctx, cctx = context.WithTimeout(context.Background(), 5*time.Second)
	defer cctx()

	var got = make(chan *roottypes.KlineUpdate, 1)
	if err := c.Stream().WatchKlines(ctx, "XBTUSDTM", futurestypes.Timeframe1m, func(ku *roottypes.KlineUpdate) {
		select {
		case got <- ku:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchKlines: %v", err)
	}

	select {
	case ku := <-got:
		if ku.StartMs != 1700000000000 || ku.Close.String() != "105" {
			t.Fatalf("kline = %+v", ku)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no kline push")
	}
}
