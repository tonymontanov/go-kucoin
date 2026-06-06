/*
FILE: spot/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Spot REST surface. A mock HTTP server returns
recorded KuCoin response bodies (wrapped in the { code, data } envelope); the
tests drive the real sub-clients end-to-end through the SDK transport and
assert the typed output, exercising URL/path construction, envelope parsing,
the wire→type converters and request-body assembly.

No network access: an explicit Config.REST.BaseURL (the httptest URL) is a
non-futures host, so the spot profile honours it as-is.
*/

package spot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

// requireDec is a tiny test helper for decimal literals.
func requireDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// writeEnv writes a KuCoin success envelope wrapping the given data JSON.
func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

// recordedBodies holds the captured request bodies per path for assertions.
type recordedBodies struct {
	mu   sync.Mutex
	body map[string]string
}

func (r *recordedBodies) set(path, body string) {
	r.mu.Lock()
	r.body[path] = body
	r.mu.Unlock()
}

func (r *recordedBodies) get(path string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body[path]
}

// newMockRESTClient builds a spot.Client whose REST base points at a mock
// server serving recorded fixtures. withCreds enables signed calls.
func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recordedBodies) {
	t.Helper()
	var rec = &recordedBodies{body: map[string]string{}}

	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v2/symbols", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureSymbol+`]`)
	})
	mux.HandleFunc("/api/v2/symbols/", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureSymbol)
	})
	mux.HandleFunc("/api/v1/market/orderbook/level1", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLevel1)
	})
	mux.HandleFunc("/api/v1/market/orderbook/level2_100", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLevel2)
	})
	mux.HandleFunc("/api/v3/market/orderbook/level2", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLevel2Full)
	})
	mux.HandleFunc("/api/v1/market/stats", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureStats)
	})
	mux.HandleFunc("/api/v1/market/candles", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureKlines)
	})
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureAccount+`]`)
	})
	// /api/v1/orders and /api/v1/orders/{id} share a prefix.
	mux.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var b, _ = io.ReadAll(r.Body)
			rec.set("/api/v1/orders", string(b))
			writeEnv(w, `{"orderId":"5bd6e9286d99522a52e458de"}`)
			return
		}
		writeEnv(w, fixtureOrderPage)
	})
	mux.HandleFunc("/api/v1/orders/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			writeEnv(w, `{"cancelledOrderIds":["5bd6e9286d99522a52e458de"]}`)
		default:
			writeEnv(w, fixtureOrder)
		}
	})

	var srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var cfg = kucoin.DefaultConfig()
	cfg.REST.BaseURL = srv.URL
	if withCreds {
		cfg.APIKey = "k"
		cfg.SecretKey = "s"
		cfg.Passphrase = "p"
	}
	var parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClient(parent), rec
}

func TestContractREST_GetSymbols(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var got, err = c.MarketData().GetSymbols(context.Background())
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "BTC-USDT" {
		t.Fatalf("got %+v", got)
	}
	if got[0].BaseIncrement.String() != "0.00000001" {
		t.Errorf("baseIncrement = %s", got[0].BaseIncrement)
	}
	if !got[0].EnableTrading {
		t.Error("enableTrading should be true")
	}
}

func TestContractREST_GetTicker(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var tk, err = c.MarketData().GetTicker(context.Background(), "BTC-USDT")
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if tk.TsMs != 1700000000000 {
		t.Errorf("TsMs = %d (spot ticker is already ms)", tk.TsMs)
	}
	if tk.BestBidPrice.String() != "50000" {
		t.Errorf("bestBid = %s", tk.BestBidPrice)
	}
}

func TestContractREST_GetOrderBook(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var ob, err = c.MarketData().GetOrderBook(context.Background(), "BTC-USDT")
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if ob.Sequence != 1000 {
		t.Errorf("sequence = %d", ob.Sequence)
	}
	if len(ob.Bids) != 2 || len(ob.Asks) != 2 {
		t.Fatalf("levels = %d/%d", len(ob.Bids), len(ob.Asks))
	}
	if ob.Bids[0].Price.String() != "50000" {
		t.Errorf("bid0 = %s", ob.Bids[0].Price)
	}
}

func TestContractREST_GetOrderBookFull(t *testing.T) {
	var c, _ = newMockRESTClient(t, true) // signed: full depth needs creds
	var ob, err = c.MarketData().GetOrderBookFull(context.Background(), "BTC-USDT")
	if err != nil {
		t.Fatalf("GetOrderBookFull: %v", err)
	}
	if ob.Sequence != 1005 {
		t.Errorf("sequence = %d", ob.Sequence)
	}
	if len(ob.Bids) != 3 || len(ob.Asks) != 3 {
		t.Fatalf("levels = %d/%d (want full-depth fixture 3/3)", len(ob.Bids), len(ob.Asks))
	}
}

func TestContractREST_GetOrderBookFull_NoCredsFailsFast(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	var _, err = c.MarketData().GetOrderBookFull(context.Background(), "BTC-USDT")
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestContractREST_GetKlines(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var candles, err = c.MarketData().GetKlines(context.Background(), "BTC-USDT", spottypes.Timeframe1m, 0, 0)
	if err != nil {
		t.Fatalf("GetKlines: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("candles = %d", len(candles))
	}
	// Spot row order is [time(sec), open, CLOSE, high, low, volume, turnover].
	if candles[0].OpenTimeMs != 1700000000000 || candles[0].Close.String() != "105" {
		t.Errorf("candle0 = %+v", candles[0])
	}
	if candles[0].High.String() != "110" || candles[0].Low.String() != "90" {
		t.Errorf("candle0 high/low = %s/%s", candles[0].High, candles[0].Low)
	}
}

func TestContractREST_PlaceOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var ack, err = c.Trading().PlaceOrder(context.Background(), spottypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderLimit,
		Size: requireDec("0.1"), Price: requireDec("50000"),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if ack.OrderID != "5bd6e9286d99522a52e458de" {
		t.Errorf("orderId = %s", ack.OrderID)
	}
	var body = rec.get("/api/v1/orders")
	if !strings.Contains(body, `"tradeType":"TRADE"`) {
		t.Errorf("body missing default tradeType: %s", body)
	}
	if !strings.Contains(body, `"size":"0.1"`) {
		t.Errorf("body missing size: %s", body)
	}
	if !strings.Contains(body, `"price":"50000"`) {
		t.Errorf("body missing price: %s", body)
	}
}

func TestContractREST_PlaceOrder_NoCredsFailsFast(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	var _, err = c.Trading().PlaceOrder(context.Background(), spottypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderMarket, Funds: requireDec("100"),
	})
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestContractREST_CancelOrder(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ids, err = c.Trading().CancelOrder(context.Background(), "5bd6e9286d99522a52e458de")
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if len(ids) != 1 || ids[0] != "5bd6e9286d99522a52e458de" {
		t.Errorf("ids = %v", ids)
	}
}

func TestContractREST_GetOrder(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ord, err = c.Trading().GetOrder(context.Background(), "5bd6e9286d99522a52e458de")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if ord.Symbol != "BTC-USDT" || ord.Side != spottypes.SideBuy {
		t.Errorf("order = %+v", ord)
	}
	if ord.DealSize.String() != "0.001" {
		t.Errorf("dealSize = %s", ord.DealSize)
	}
	if !ord.IsActive {
		t.Error("order should be active")
	}
}

func TestContractREST_GetBalance(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var bal, err = c.Account().GetBalance(context.Background(), "USDT")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.MarginCoin != "USDT" || bal.AvailableBalance.String() != "900.5" {
		t.Errorf("balance = %+v", bal)
	}
	if bal.LockedBalance.String() != "100" {
		t.Errorf("locked = %s", bal.LockedBalance)
	}
}

// ---- Recorded fixtures (trimmed real KuCoin spot shapes) ----------------

const fixtureSymbol = `{
	"symbol":"BTC-USDT","name":"BTC-USDT","baseCurrency":"BTC","quoteCurrency":"USDT",
	"feeCurrency":"USDT","market":"USDS",
	"baseMinSize":"0.00001","quoteMinSize":"0.1","baseMaxSize":"10000000000","quoteMaxSize":"99999999",
	"baseIncrement":"0.00000001","quoteIncrement":"0.000001","priceIncrement":"0.1",
	"priceLimitRate":"0.1","minFunds":"0.1","isMarginEnabled":true,"enableTrading":true
}`

const fixtureLevel1 = `{
	"sequence":"1700000000","price":"50000.5","size":"0.1",
	"bestBid":"50000.0","bestBidSize":"1.2","bestAsk":"50001.0","bestAskSize":"0.8",
	"time":1700000000000
}`

const fixtureLevel2 = `{
	"sequence":"1000","time":1700000000000,
	"bids":[["50000.0","5"],["49999.0","3"]],"asks":[["50001.0","2"],["50002.0","8"]]
}`

const fixtureLevel2Full = `{
	"sequence":"1005","time":1700000000005,
	"bids":[["50000.0","5"],["49999.0","3"],["49998.0","7"]],
	"asks":[["50001.0","2"],["50002.0","8"],["50003.0","4"]]
}`

const fixtureStats = `{
	"time":1700000000000,"symbol":"BTC-USDT","buy":"50000","sell":"50001",
	"changeRate":"0.01","changePrice":"500","high":"51000","low":"49000",
	"vol":"1234","volValue":"61000000","last":"50000.5","averagePrice":"50000",
	"takerFeeRate":"0.001","makerFeeRate":"0.001"
}`

const fixtureKlines = `[
	["1700000000","100","105","110","90","1234","100"],
	["1700000060","105","118","120","104","999","90"]
]`

const fixtureAccount = `{
	"id":"a1","currency":"USDT","type":"trade","balance":"1000.5","available":"900.5","holds":"100.0"
}`

const fixtureOrder = `{
	"id":"5bd6e9286d99522a52e458de","clientOid":"abc","symbol":"BTC-USDT","opType":"DEAL",
	"type":"limit","side":"buy","price":"50000","size":"0.1","funds":"0",
	"dealFunds":"50","dealSize":"0.001","fee":"0.05","feeCurrency":"USDT",
	"timeInForce":"GTC","postOnly":true,"hidden":false,"iceberg":false,
	"visibleSize":"0","cancelAfter":0,"stp":"","tradeType":"TRADE",
	"isActive":true,"cancelExist":false,"createdAt":1700000000000
}`

const fixtureOrderPage = `{
	"currentPage":1,"pageSize":50,"totalNum":1,"totalPage":1,
	"items":[` + fixtureOrder + `]
}`
