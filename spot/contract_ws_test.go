/*
FILE: spot/contract_ws_test.go

DESCRIPTION:
Offline contract tests for the public spot WS dispatch path. A single mock
server plays three roles: it serves the public bullet token (pointing the WS
endpoint back at itself), serves the REST level2_100 snapshot used to seed
the managed order book, and upgrades /ws to a WebSocket that greets with
"welcome", acks subscriptions and pushes recorded frames per topic.

The tests drive the real StreamClient end-to-end and assert the typed structs
handed to the callbacks — covering bullet plumbing, the welcome/subscribe
handshake, frame dispatch and the wire→type converters, including the managed
order-book seed + multi-change sequence apply.
*/

package spot

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
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// newMockStreamServer returns an httptest server that serves bullet + REST
// snapshot + a WS endpoint, and a spot.Client wired to it.
func newMockStreamServer(t *testing.T) *Client {
	t.Helper()
	var upgrader = websocket.Upgrader{}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v1/bullet-public", func(w http.ResponseWriter, r *http.Request) {
		var endpoint = "ws://" + r.Host + "/ws"
		writeEnv(w, `{"token":"tkn","instanceServers":[{"endpoint":"`+endpoint+`","pingInterval":10000,"pingTimeout":5000,"encrypt":false,"protocol":"websocket"}]}`)
	})
	mux.HandleFunc("/api/v1/market/orderbook/level2_100", func(w http.ResponseWriter, r *http.Request) {
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
	return NewClient(parent)
}

// pushForTopic writes a recorded push frame matching the subscribed topic.
func pushForTopic(conn *websocket.Conn, topic string) {
	switch {
	case strings.HasPrefix(topic, "/market/level2:"):
		// New best bid at 50000.5 (snapshot best bid was 50000), seq 1001 =
		// snapshot seq + 1. Interleave an ask change to exercise the sort.
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"trade.l2update","data":{"sequenceStart":1001,"sequenceEnd":1002,"symbol":"BTC-USDT","time":1700000000111,"changes":{"asks":[["50002.0","0","1002"]],"bids":[["50000.5","9","1001"]]}}}`))
	case strings.HasPrefix(topic, "/market/ticker:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"trade.ticker","data":{"sequence":"2","price":"50000.5","size":"4","bestBid":"50000.0","bestBidSize":"11","bestAsk":"50001.0","bestAskSize":"6","time":1700000000000}}`))
	case strings.HasPrefix(topic, "/market/match:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"trade.l3match","data":{"sequence":"3","symbol":"BTC-USDT","side":"sell","size":"0.7","price":"50001.0","tradeId":"e1","time":1700000000000000000}}`))
	case strings.HasPrefix(topic, "/market/candles:"):
		_ = conn.WriteMessage(websocket.TextMessage, []byte(
			`{"type":"message","topic":"`+topic+`","subject":"trade.candles.update","data":{"symbol":"BTC-USDT","candles":["1700000000","100","105","110","90","1234","5678"],"time":1700000000000000000}}`))
	}
}

func TestContractWS_OrderBookManaged(t *testing.T) {
	var c = newMockStreamServer(t)
	defer func() { _ = c.Stream().Close() }()

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got = make(chan *roottypes.OrderBookSnapshot, 8)
	var err = c.Stream().WatchOrderBook(ctx, "BTC-USDT", func(ob *roottypes.OrderBookSnapshot) {
		got <- ob
	})
	if err != nil {
		t.Fatalf("WatchOrderBook: %v", err)
	}

	var deadline = time.After(5 * time.Second)
	for {
		select {
		case ob := <-got:
			// Best bid should be the applied 50000.5 and the ask removal of
			// 50002.0 should leave 50001.0 as best ask. Sequence advances to
			// 1002 (last applied change).
			if len(ob.Bids) > 0 && ob.Bids[0].Price.String() == "50000.5" && ob.Sequence == 1002 {
				if len(ob.Asks) > 0 && ob.Asks[0].Price.String() != "50001" {
					t.Fatalf("best ask = %s, want 50001 after removal", ob.Asks[0].Price)
				}
				return
			}
		case <-deadline:
			t.Fatal("did not receive a reconciled snapshot with the applied changes")
		}
	}
}

func TestContractWS_Ticker(t *testing.T) {
	var c = newMockStreamServer(t)
	defer func() { _ = c.Stream().Close() }()

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got = make(chan *spottypes.MarketTicker, 1)
	if err := c.Stream().WatchTicker(ctx, "BTC-USDT", func(tk *spottypes.MarketTicker) {
		select {
		case got <- tk:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchTicker: %v", err)
	}

	select {
	case tk := <-got:
		if tk.Symbol != "BTC-USDT" || tk.BestBidPrice.String() != "50000" || tk.TsMs != 1700000000000 {
			t.Fatalf("ticker = %+v", tk)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ticker push")
	}
}

func TestContractWS_Trades(t *testing.T) {
	var c = newMockStreamServer(t)
	defer func() { _ = c.Stream().Close() }()

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got = make(chan *roottypes.TradeUpdate, 1)
	if err := c.Stream().WatchTrades(ctx, "BTC-USDT", func(tr *roottypes.TradeUpdate) {
		select {
		case got <- tr:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchTrades: %v", err)
	}

	select {
	case tr := <-got:
		// Spot match time is ns → converted to ms.
		if tr.Side != "sell" || tr.Price.String() != "50001" || tr.TsMs != 1700000000000 {
			t.Fatalf("trade = %+v", tr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no trade push")
	}
}

func TestContractWS_Klines(t *testing.T) {
	var c = newMockStreamServer(t)
	defer func() { _ = c.Stream().Close() }()

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got = make(chan *roottypes.KlineUpdate, 1)
	if err := c.Stream().WatchKlines(ctx, "BTC-USDT", spottypes.Timeframe1m, func(ku *roottypes.KlineUpdate) {
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
