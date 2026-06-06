/*
FILE: convert/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Convert REST surface. A mock HTTP server returns
recorded KuCoin response bodies (wrapped in the { code, data } envelope); the
tests drive the real client end-to-end through the SDK transport and assert the
typed output — path construction, envelope parsing, wire→type converters,
body/query assembly, the public-vs-signed gate, the flexStr orderId (string vs
number) and nullable limit-order timestamps.
*/

package convert

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	convtypes "github.com/tonymontanov/go-kucoin/v2/convert/types"
)

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

type recorded struct {
	mu    sync.Mutex
	body  map[string]string
	query map[string]string
}

func (r *recorded) set(key, body, query string) {
	r.mu.Lock()
	r.body[key] = body
	r.query[key] = query
	r.mu.Unlock()
}

func (r *recorded) getBody(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body[key]
}

func (r *recorded) getQuery(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query[key]
}

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recorded) {
	t.Helper()
	var rec = &recorded{body: map[string]string{}, query: map[string]string{}}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v1/convert/symbol", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/symbol", "", r.URL.RawQuery)
		writeEnv(w, `{"fromCurrency":"BTC","toCurrency":"USDT","fromCurrencyMaxSize":"10","fromCurrencyMinSize":"0.000001","fromCurrencyStep":"0.00000001","toCurrencyMaxSize":"900000","toCurrencyMinSize":"0.01","toCurrencyStep":"0.00000001"}`)
	})
	mux.HandleFunc("/api/v1/convert/currencies", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/currencies", "", r.URL.RawQuery)
		writeEnv(w, `{"currencies":[{"currency":"HYPE","maxSize":"2000","minSize":"0.01","step":"0.00000001","tradeDirection":"ALL"}],"usdtCurrencyLimit":[{"currency":"SCR","maxSize":"10000","minSize":"0.1","step":"0.00000001"}]}`)
	})
	mux.HandleFunc("/api/v1/convert/quote", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/quote", "", r.URL.RawQuery)
		writeEnv(w, `{"quoteId":"d6dvjbt19cpheg28fueg","price":"65547.9811","fromCurrencySize":"0.00007628","toCurrencySize":"5","validUntill":1771829687707}`)
	})
	mux.HandleFunc("/api/v1/convert/order", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v1/convert/order", string(b), "")
		writeEnv(w, `{"clientOrderId":"client0001","orderId":"12565022"}`)
	})
	mux.HandleFunc("/api/v1/convert/order/detail", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/order/detail", "", r.URL.RawQuery)
		// orderId arrives as a BARE NUMBER here → exercise flexStr.
		writeEnv(w, `{"clientOrderId":"d0k3k4519cplvqk8um80","orderId":10721315,"price":"0.000009498","fromCurrency":"USDT","toCurrency":"BTC","fromCurrencySize":"5","toCurrencySize":"0.00004749","accountType":"TRADING","orderTime":1748939927000,"status":"SUCCESS"}`)
	})
	mux.HandleFunc("/api/v1/convert/order/history", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/order/history", "", r.URL.RawQuery)
		writeEnv(w, `{"currentPage":1,"pageSize":20,"totalNum":1,"totalPage":1,"items":[{"clientOrderId":"d0k3k4519cplvqk8um80","orderId":10721315,"price":"0.000009498","fromCurrency":"USDT","toCurrency":"BTC","fromCurrencySize":"5","toCurrencySize":"0.00004749","accountType":"TRADING","orderTime":1748939927000,"status":"SUCCESS"}]}`)
	})
	mux.HandleFunc("/api/v1/convert/limit/quote", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/limit/quote", "", r.URL.RawQuery)
		writeEnv(w, `{"price":"105019.9538","validUntill":1749023723018}`)
	})
	mux.HandleFunc("/api/v1/convert/limit/order", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v1/convert/limit/order", string(b), "")
		writeEnv(w, `{"clientOrderId":"8dbfc2f1-f37e-4346-83be-b63ad11dc0d9","orderId":"683ff823e66ccd0007ed881b"}`)
	})
	mux.HandleFunc("/api/v1/convert/limit/order/detail", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/limit/order/detail", "", r.URL.RawQuery)
		// orderId as STRING; filledTime + cancelType null.
		writeEnv(w, `{"clientOrderId":"8dbfc2f1-f37e-4346-83be-b63ad11dc0d9","orderId":"683ff823e66ccd0007ed881b","price":"0.000009524","fromCurrency":"USDT","toCurrency":"BTC","fromCurrencySize":"5","toCurrencySize":"0.00004762","accountType":"BOTH","orderTime":1749022755000,"status":"CANCELLED","expiryTime":1749627556000,"cancelTime":1749022900000,"filledTime":null,"cancelType":0}`)
	})
	mux.HandleFunc("/api/v1/convert/limit/orders", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/convert/limit/orders", "", r.URL.RawQuery)
		writeEnv(w, `{"currentPage":1,"pageSize":20,"totalNum":1,"totalPage":1,"items":[{"clientOrderId":"x","orderId":"683ff9f39f47d70006dacb0f","price":"0.000009522","fromCurrency":"USDT","toCurrency":"BTC","fromCurrencySize":"5","toCurrencySize":"0.00004761","accountType":"BOTH","orderTime":1749023219000,"status":"SUCCESS","expiryTime":1749628020000,"cancelTime":null,"filledTime":1749045700000,"cancelType":null}]}`)
	})
	mux.HandleFunc("/api/v1/convert/limit/order/cancel", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("DELETE /api/v1/convert/limit/order/cancel", string(b), "")
		writeEnv(w, `null`)
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

func TestContractREST_GetSymbol(t *testing.T) {
	// Public: works WITHOUT credentials.
	var c, rec = newMockRESTClient(t, false)
	var s, err = c.GetSymbol(context.Background(), "BTC", "USDT", "MARKET")
	if err != nil {
		t.Fatalf("GetSymbol: %v", err)
	}
	if s.FromCurrency != "BTC" || s.FromCurrencyMinSize.String() != "0.000001" {
		t.Errorf("symbol = %+v", s)
	}
	if !strings.Contains(rec.getQuery("/api/v1/convert/symbol"), "orderType=MARKET") {
		t.Errorf("query = %s", rec.getQuery("/api/v1/convert/symbol"))
	}
}

func TestContractREST_GetCurrencies(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var cur, err = c.GetCurrencies(context.Background())
	if err != nil {
		t.Fatalf("GetCurrencies: %v", err)
	}
	if len(cur.Currencies) != 1 || cur.Currencies[0].Currency != "HYPE" || cur.Currencies[0].TradeDirection != "ALL" {
		t.Fatalf("currencies = %+v", cur)
	}
	if len(cur.USDTCurrencyLimit) != 1 || cur.USDTCurrencyLimit[0].Currency != "SCR" {
		t.Errorf("usdt limit = %+v", cur.USDTCurrencyLimit)
	}
}

func TestContractREST_GetQuote(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var q, err = c.GetQuote(context.Background(), convtypes.QuoteRequest{
		FromCurrency: "BTC", ToCurrency: "USDT", ToCurrencySize: "5",
	})
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	if q.QuoteID != "d6dvjbt19cpheg28fueg" || q.ValidUntil != 1771829687707 {
		t.Errorf("quote = %+v", q)
	}
	if !strings.Contains(rec.getQuery("/api/v1/convert/quote"), "toCurrencySize=5") {
		t.Errorf("query = %s", rec.getQuery("/api/v1/convert/quote"))
	}
}

func TestContractREST_QuoteValidation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.GetQuote(context.Background(), convtypes.QuoteRequest{FromCurrency: "BTC", ToCurrency: "USDT"})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_PlaceMarketOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.PlaceMarketOrder(context.Background(), convtypes.PlaceMarketRequest{
		ClientOrderID: "client0001", QuoteID: "d6dvkg519cpheg28h490", AccountType: "BOTH",
	})
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}
	if res.OrderID != "12565022" || res.ClientOrderID != "client0001" {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(rec.getBody("POST /api/v1/convert/order"), `"accountType":"BOTH"`) {
		t.Errorf("body = %s", rec.getBody("POST /api/v1/convert/order"))
	}
}

func TestContractREST_PlaceLimitOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.PlaceLimitOrder(context.Background(), convtypes.PlaceLimitRequest{
		ClientOrderID: "8dbfc2f1-f37e-4346-83be-b63ad11dc0d9",
		FromCurrency:  "USDT", ToCurrency: "BTC",
		FromCurrencySize: "5", ToCurrencySize: "0.00004761",
	})
	if err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	if res.OrderID != "683ff823e66ccd0007ed881b" {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(rec.getBody("POST /api/v1/convert/limit/order"), `"toCurrencySize":"0.00004761"`) {
		t.Errorf("body = %s", rec.getBody("POST /api/v1/convert/limit/order"))
	}
}

func TestContractREST_PlaceLimitOrderValidation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.PlaceLimitOrder(context.Background(), convtypes.PlaceLimitRequest{
		ClientOrderID: "x", FromCurrency: "USDT", ToCurrency: "BTC", FromCurrencySize: "5",
	})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_GetOrder_FlexStrNumber(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var o, err = c.GetOrder(context.Background(), "10721315", "")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	// orderId came back as a number on the wire → normalised to string.
	if o.OrderID != "10721315" || o.Status != "SUCCESS" || o.Price.String() != "0.000009498" {
		t.Errorf("order = %+v", o)
	}
	if !strings.Contains(rec.getQuery("/api/v1/convert/order/detail"), "orderId=10721315") {
		t.Errorf("query = %s", rec.getQuery("/api/v1/convert/order/detail"))
	}
}

func TestContractREST_GetLimitOrder_Nullable(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var o, err = c.GetLimitOrder(context.Background(), "", "8dbfc2f1-f37e-4346-83be-b63ad11dc0d9")
	if err != nil {
		t.Fatalf("GetLimitOrder: %v", err)
	}
	if o.OrderID != "683ff823e66ccd0007ed881b" || o.Status != "CANCELLED" {
		t.Fatalf("order = %+v", o)
	}
	// filledTime was null → 0; cancelTime present.
	if o.FilledTime != 0 || o.CancelTime != 1749022900000 {
		t.Errorf("times = filled %d cancel %d", o.FilledTime, o.CancelTime)
	}
}

func TestContractREST_Histories(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ctx = context.Background()
	var mp, err = c.GetOrderHistory(ctx, convtypes.HistoryQuery{Page: 1, PageSize: 20, Status: "SUCCESS"})
	if err != nil || mp.TotalNum != 1 || len(mp.Items) != 1 || mp.Items[0].OrderID != "10721315" {
		t.Fatalf("order history = %+v err %v", mp, err)
	}
	var lp, lerr = c.GetLimitOrders(ctx, convtypes.HistoryQuery{Page: 1})
	if lerr != nil || lp.TotalNum != 1 || len(lp.Items) != 1 || lp.Items[0].FilledTime != 1749045700000 {
		t.Fatalf("limit orders = %+v err %v", lp, lerr)
	}
}

func TestContractREST_CancelLimitOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	if err := c.CancelLimitOrder(context.Background(), "683ff823e66ccd0007ed881b"); err != nil {
		t.Fatalf("CancelLimitOrder: %v", err)
	}
	if !strings.Contains(rec.getBody("DELETE /api/v1/convert/limit/order/cancel"), `"clientOrderId":"683ff823e66ccd0007ed881b"`) {
		t.Errorf("body = %s", rec.getBody("DELETE /api/v1/convert/limit/order/cancel"))
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	// A SIGNED endpoint must fail without credentials.
	var _, err = c.GetQuote(context.Background(), convtypes.QuoteRequest{FromCurrency: "BTC", ToCurrency: "USDT", ToCurrencySize: "5"})
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
