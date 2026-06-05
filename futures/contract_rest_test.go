/*
FILE: futures/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Futures REST surface. A mock HTTP server
returns recorded KuCoin response bodies (wrapped in the { code, data }
envelope); the tests drive the real sub-clients end-to-end through the SDK
transport and assert the typed output, exercising URL/path construction,
envelope parsing and the wire→type converters.

No network access: the SDK's REST base URL is pointed at httptest.
*/

package futures

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
)

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

// newMockRESTClient builds a futures.Client whose REST base points at a
// mock server serving recorded fixtures. withCreds enables signed calls.
func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recordedBodies) {
	t.Helper()
	var rec = &recordedBodies{body: map[string]string{}}

	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v1/contracts/active", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureContract+`]`)
	})
	mux.HandleFunc("/api/v1/ticker", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureTicker)
	})
	mux.HandleFunc("/api/v1/level2/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLevel2)
	})
	mux.HandleFunc("/api/v1/kline/query", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureKlines)
	})
	mux.HandleFunc("/api/v1/account-overview", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureAccountOverview)
	})
	mux.HandleFunc("/api/v1/positions", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixturePosition+`]`)
	})
	// /api/v1/orders and /api/v1/orders/{id} share a prefix.
	mux.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var b, _ = io.ReadAll(r.Body)
			rec.set("/api/v1/orders", string(b))
			writeEnv(w, `{"orderId":"234125150956625920","clientOid":"abc"}`)
			return
		}
		writeEnv(w, fixtureOrderPage)
	})
	mux.HandleFunc("/api/v1/orders/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			writeEnv(w, `{"cancelledOrderIds":["234125150956625920"]}`)
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
	return NewClientWithSettings(parent, ClientSettings{DefaultLeverage: "5"}), rec
}

func TestContractREST_GetContracts(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var got, err = c.MarketData().GetContracts(context.Background())
	if err != nil {
		t.Fatalf("GetContracts: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "XBTUSDTM" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Multiplier.String() != "0.001" {
		t.Errorf("multiplier = %s", got[0].Multiplier)
	}
	if got[0].OpenInterest.String() != "4955514" {
		t.Errorf("openInterest = %s", got[0].OpenInterest)
	}
}

func TestContractREST_GetTicker(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var tk, err = c.MarketData().GetTicker(context.Background(), "XBTUSDTM")
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if tk.TsMs != 1700000000000 {
		t.Errorf("TsMs = %d (want ns→ms)", tk.TsMs)
	}
	if tk.BestBidPrice.String() != "50000" {
		t.Errorf("bestBid = %s", tk.BestBidPrice)
	}
}

func TestContractREST_GetOrderBook(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var ob, err = c.MarketData().GetOrderBook(context.Background(), "XBTUSDTM")
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if ob.Sequence != 1000 {
		t.Errorf("sequence = %d", ob.Sequence)
	}
	if len(ob.Bids) != 2 || len(ob.Asks) != 2 {
		t.Fatalf("levels = %d/%d", len(ob.Bids), len(ob.Asks))
	}
}

func TestContractREST_GetKlines(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var candles, err = c.MarketData().GetKlines(context.Background(), "XBTUSDTM", futurestypes.Timeframe1m, 0, 0)
	if err != nil {
		t.Fatalf("GetKlines: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("candles = %d", len(candles))
	}
	if candles[0].OpenTimeMs != 1700000000000 || candles[0].Close.String() != "105" {
		t.Errorf("candle0 = %+v", candles[0])
	}
}

func TestContractREST_PlaceOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var ack, err = c.Trading().PlaceOrder(context.Background(), futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderMarket, Size: 2,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if ack.OrderID != "234125150956625920" {
		t.Errorf("orderId = %s", ack.OrderID)
	}
	// The wire body must carry the defaulted leverage and the size.
	var body = rec.get("/api/v1/orders")
	if !strings.Contains(body, `"leverage":"5"`) {
		t.Errorf("body missing default leverage: %s", body)
	}
	if !strings.Contains(body, `"size":2`) {
		t.Errorf("body missing size: %s", body)
	}
}

func TestContractREST_PlaceOrder_NoCredsFailsFast(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	var _, err = c.Trading().PlaceOrder(context.Background(), futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderMarket, Size: 1, Leverage: "5",
	})
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestContractREST_CancelOrder(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ids, err = c.Trading().CancelOrder(context.Background(), "234125150956625920")
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if len(ids) != 1 || ids[0] != "234125150956625920" {
		t.Errorf("ids = %v", ids)
	}
}

func TestContractREST_GetOrder(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ord, err = c.Trading().GetOrder(context.Background(), "234125150956625920")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if ord.Symbol != "XBTUSDTM" || ord.Side != futurestypes.SideBuy {
		t.Errorf("order = %+v", ord)
	}
	if ord.FilledSize.String() != "1" {
		t.Errorf("filledSize = %s", ord.FilledSize)
	}
}

func TestContractREST_GetBalanceAndPositions(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var bal, err = c.Account().GetBalance(context.Background(), "USDT")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.MarginCoin != "USDT" || bal.AvailableBalance.String() != "900.5" {
		t.Errorf("balance = %+v", bal)
	}

	var pos, perr = c.Account().GetPositions(context.Background())
	if perr != nil {
		t.Fatalf("GetPositions: %v", perr)
	}
	if len(pos) != 1 || !pos[0].IsOpen || pos[0].CurrentQty.String() != "3" {
		t.Errorf("positions = %+v", pos)
	}
}

// ---- Recorded fixtures (trimmed real KuCoin shapes) ---------------------

const fixtureContract = `{
	"symbol":"XBTUSDTM","rootSymbol":"USDT","type":"FFWCSX",
	"baseCurrency":"XBT","quoteCurrency":"USDT","settleCurrency":"USDT",
	"status":"Open","isInverse":false,
	"multiplier":0.001,"lotSize":1,"tickSize":0.1,"indexPriceTickSize":0.01,
	"maxOrderQty":1000000,"maxPrice":1000000.0,
	"makerFeeRate":0.0002,"takerFeeRate":0.0006,
	"markPrice":50000.1,"indexPrice":50000.0,
	"openInterest":"4955514","volumeOf24h":6788.072,"turnoverOf24h":5.98e8
}`

const fixtureTicker = `{
	"sequence":1700000000,"symbol":"XBTUSDTM","side":"sell","size":3,
	"price":"50000.5","bestBidPrice":"50000.0","bestBidSize":10,
	"bestAskPrice":"50001.0","bestAskSize":7,"tradeId":"t1","ts":1700000000000000000
}`

const fixtureLevel2 = `{
	"sequence":1000,"symbol":"XBTUSDTM",
	"bids":[[50000.0,5],[49999.0,3]],"asks":[[50001.0,2],[50002.0,8]],"ts":1700000000000
}`

const fixtureKlines = `[
	[1700000000000,100,110,90,105,1234],
	[1700000060000,105,120,104,118,999]
]`

const fixtureAccountOverview = `{
	"accountEquity":1000.0,"unrealisedPNL":12.5,"marginBalance":987.5,
	"positionMargin":50.0,"orderMargin":37.0,"frozenFunds":0.0,
	"availableBalance":900.5,"currency":"USDT"
}`

const fixturePosition = `{
	"symbol":"XBTUSDTM","settleCurrency":"USDT","isOpen":true,"crossMode":false,
	"currentQty":3,"avgEntryPrice":50000.0,"markPrice":50010.0,"markValue":150.03,
	"liquidationPrice":40000.0,"bankruptPrice":39500.0,"realLeverage":5.0,
	"posMargin":30.0,"posCost":150.0,"maintMarginReq":0.004,"riskLimit":250000,
	"unrealisedPnl":0.03,"unrealisedPnlPcnt":0.0002,"realisedPnl":0.0,
	"openingTimestamp":1700000000000
}`

const fixtureOrder = `{
	"id":"234125150956625920","clientOid":"abc","symbol":"XBTUSDTM",
	"type":"limit","side":"buy","price":"50000","size":2,"value":"100",
	"dealValue":"50","dealSize":1,"leverage":"5","timeInForce":"GTC",
	"marginMode":"ISOLATED","postOnly":true,"hidden":false,"iceberg":false,
	"reduceOnly":false,"closeOrder":false,"status":"open","isActive":true,
	"cancelExist":false,"settleCurrency":"USDT","createdAt":1700000000000,
	"updatedAt":1700000001000
}`

const fixtureOrderPage = `{
	"currentPage":1,"pageSize":50,"totalNum":1,"totalPage":1,
	"items":[` + fixtureOrder + `]
}`
