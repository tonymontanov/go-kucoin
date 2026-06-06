/*
FILE: margin/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Margin REST surface. A mock HTTP server returns
recorded KuCoin response bodies (wrapped in the { code, data } envelope); the
tests drive the real sub-clients end-to-end through the SDK transport and
assert the typed output — exercising path construction, envelope parsing, the
wire→type converters and HF-margin request-body assembly (isIsolated /
autoBorrow).

No network access: an explicit Config.REST.BaseURL (the httptest URL) is a
non-futures host, so the margin profile honours it as-is.
*/

package margin

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
	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
)

func requireDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

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

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recordedBodies) {
	t.Helper()
	var rec = &recordedBodies{body: map[string]string{}}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v3/margin/symbols", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"timestamp":1729665839353,"items":[`+fixtureCrossSymbol+`]}`)
	})
	mux.HandleFunc("/api/v1/isolated/symbols", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureIsolatedSymbol+`]`)
	})
	mux.HandleFunc("/api/v1/mark-price/", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureMarkPrice)
	})
	mux.HandleFunc("/api/v1/margin/config", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureMarginConfig)
	})
	mux.HandleFunc("/api/v3/margin/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureCrossAccount)
	})
	mux.HandleFunc("/api/v3/isolated/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureIsolatedAccount)
	})
	mux.HandleFunc("/api/v3/margin/currencies", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureCrossRiskLimit+`]`)
	})
	mux.HandleFunc("/api/v3/margin/borrow", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var b, _ = io.ReadAll(r.Body)
			rec.set("/api/v3/margin/borrow", string(b))
		}
		writeEnv(w, `{"orderNo":"borrow-1","actualSize":"10"}`)
	})
	mux.HandleFunc("/api/v3/margin/repay", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"orderNo":"repay-1","actualSize":"5"}`)
	})

	// Place (exact /order) and test (/order/test).
	mux.HandleFunc("/api/v3/hf/margin/order", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("/api/v3/hf/margin/order", string(b))
		writeEnv(w, `{"orderId":"671663e02188630007e21c9c","clientOid":"abc","borrowSize":"10.2","loanApplyId":"loan-1"}`)
	})
	mux.HandleFunc("/api/v3/hf/margin/order/", func(w http.ResponseWriter, r *http.Request) {
		// /order/test, /order/active/symbols
		if strings.HasSuffix(r.URL.Path, "/active/symbols") {
			writeEnv(w, `{"symbols":["BTC-USDT","ETH-USDT"]}`)
			return
		}
		var b, _ = io.ReadAll(r.Body)
		rec.set("/api/v3/hf/margin/order/test", string(b))
		writeEnv(w, `{"orderId":"test-1","clientOid":"abc"}`)
	})
	// Cancel-all (exact /orders) and the /orders/* subtree.
	mux.HandleFunc("/api/v3/hf/margin/orders", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `"success"`)
	})
	mux.HandleFunc("/api/v3/hf/margin/orders/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/active"):
			writeEnv(w, `[`+fixtureMarginOrder+`]`)
		case strings.HasSuffix(r.URL.Path, "/done"):
			writeEnv(w, `{"lastId":123,"items":[`+fixtureMarginOrder+`]}`)
		case r.Method == http.MethodDelete:
			writeEnv(w, `{"orderId":"671663e02188630007e21c9c"}`)
		default:
			writeEnv(w, fixtureMarginOrder)
		}
	})
	mux.HandleFunc("/api/v3/hf/margin/fills", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"lastId":9,"items":[`+fixtureMarginFill+`]}`)
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

func TestContractREST_GetCrossSymbols(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var got, err = c.MarketData().GetCrossSymbols(context.Background(), "")
	if err != nil {
		t.Fatalf("GetCrossSymbols: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "BTC-USDT" {
		t.Fatalf("got %+v", got)
	}
	if got[0].BaseIncrement.String() != "0.00000001" || !got[0].EnableTrading {
		t.Errorf("symbol = %+v", got[0])
	}
}

func TestContractREST_GetMarkPrice(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var mp, err = c.MarketData().GetMarkPrice(context.Background(), "BTC-USDT")
	if err != nil {
		t.Fatalf("GetMarkPrice: %v", err)
	}
	if mp.Value.String() != "50000.1" || mp.TsMs != 1729665839353 {
		t.Errorf("markPrice = %+v", mp)
	}
}

func TestContractREST_GetMarginConfig(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var cfg, err = c.MarketData().GetMarginConfig(context.Background())
	if err != nil {
		t.Fatalf("GetMarginConfig: %v", err)
	}
	if cfg.MaxLeverage != 5 || cfg.LiqDebtRatio.String() != "0.97" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestContractREST_PlaceOrder_CrossDefault(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var ack, err = c.Trading().PlaceOrder(context.Background(), margintypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderLimit,
		Size: requireDec("0.1"), Price: requireDec("50000"), AutoBorrow: true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if ack.OrderID != "671663e02188630007e21c9c" || ack.BorrowSize.String() != "10.2" || ack.LoanApplyID != "loan-1" {
		t.Errorf("ack = %+v", ack)
	}
	var body = rec.get("/api/v3/hf/margin/order")
	if strings.Contains(body, `"isIsolated"`) {
		t.Errorf("cross default must omit isIsolated: %s", body)
	}
	if !strings.Contains(body, `"autoBorrow":true`) {
		t.Errorf("body missing autoBorrow: %s", body)
	}
	if !strings.Contains(body, `"price":"50000"`) || !strings.Contains(body, `"size":"0.1"`) {
		t.Errorf("body missing price/size: %s", body)
	}
}

func TestContractREST_PlaceOrder_IsolatedDefault(t *testing.T) {
	var parent, rec = newMockRESTClient(t, true)
	// Build an isolated-default client over the same parent transport.
	var c = NewClientWithSettings(parent.Parent(), ClientSettings{DefaultTradeType: margintypes.TradeIsolated})
	var _, err = c.Trading().PlaceOrder(context.Background(), margintypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: margintypes.SideSell, Type: margintypes.OrderLimit,
		Size: requireDec("0.1"), Price: requireDec("50000"),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	var body = rec.get("/api/v3/hf/margin/order")
	if !strings.Contains(body, `"isIsolated":true`) {
		t.Errorf("isolated default must set isIsolated: %s", body)
	}
}

func TestContractREST_CancelOrder(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var id, err = c.Trading().CancelOrder(context.Background(), "BTC-USDT", "671663e02188630007e21c9c")
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if id != "671663e02188630007e21c9c" {
		t.Errorf("id = %s", id)
	}
}

func TestContractREST_CancelOrder_RequiresSymbol(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.Trading().CancelOrder(context.Background(), "", "x")
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_GetOpenOrders(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var orders, err = c.Trading().GetOpenOrders(context.Background(), "BTC-USDT", "")
	if err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if len(orders) != 1 || orders[0].Symbol != "BTC-USDT" {
		t.Fatalf("orders = %+v", orders)
	}
	if !orders[0].IsActive || orders[0].TradeType != margintypes.TradeCross {
		t.Errorf("order = %+v", orders[0])
	}
}

func TestContractREST_GetCrossAccount(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var acc, err = c.Account().GetCrossAccount(context.Background(), "USDT")
	if err != nil {
		t.Fatalf("GetCrossAccount: %v", err)
	}
	if acc.Status != "EFFECTIVE" || len(acc.Accounts) != 1 {
		t.Fatalf("account = %+v", acc)
	}
	if acc.Accounts[0].MaxBorrowSize.String() != "163" || !acc.Accounts[0].BorrowEnabled {
		t.Errorf("asset = %+v", acc.Accounts[0])
	}
}

func TestContractREST_GetIsolatedAccount(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var acc, err = c.Account().GetIsolatedAccount(context.Background(), "BTC-USDT", "USDT")
	if err != nil {
		t.Fatalf("GetIsolatedAccount: %v", err)
	}
	if len(acc.Assets) != 1 || acc.Assets[0].Symbol != "BTC-USDT" {
		t.Fatalf("account = %+v", acc)
	}
	if acc.Assets[0].QuoteAsset.Liability.String() != "0.00038891" {
		t.Errorf("quote leg = %+v", acc.Assets[0].QuoteAsset)
	}
}

func TestContractREST_Borrow(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Borrow().Borrow(context.Background(), BorrowParams{Currency: "USDT", Size: requireDec("10")})
	if err != nil {
		t.Fatalf("Borrow: %v", err)
	}
	if res.OrderNo != "borrow-1" || res.ActualSize.String() != "10" {
		t.Errorf("res = %+v", res)
	}
	var body = rec.get("/api/v3/margin/borrow")
	if !strings.Contains(body, `"timeInForce":"IOC"`) {
		t.Errorf("borrow default TIF should be IOC: %s", body)
	}
}

func TestContractREST_Repay(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var res, err = c.Borrow().Repay(context.Background(), RepayParams{Currency: "USDT", Size: requireDec("5")})
	if err != nil {
		t.Fatalf("Repay: %v", err)
	}
	if res.OrderNo != "repay-1" || res.ActualSize.String() != "5" {
		t.Errorf("res = %+v", res)
	}
}

func TestContractREST_GetCrossRiskLimit(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var rl, err = c.RiskLimit().GetCrossRiskLimit(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("GetCrossRiskLimit: %v", err)
	}
	if len(rl) != 1 || rl[0].Currency != "BTC" || rl[0].BorrowMaxAmount.String() != "75.15" {
		t.Fatalf("riskLimit = %+v", rl)
	}
}

func TestContractREST_PlaceOrder_NoCredsFailsFast(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var _, err = c.Trading().PlaceOrder(context.Background(), margintypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderMarket, Funds: requireDec("100"),
	})
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

// ---- Recorded fixtures (trimmed real KuCoin margin shapes) --------------

const fixtureCrossSymbol = `{
	"symbol":"BTC-USDT","name":"BTC-USDT","enableTrading":true,"market":"USDS",
	"baseCurrency":"BTC","quoteCurrency":"USDT","feeCurrency":"USDT",
	"baseIncrement":"0.00000001","baseMinSize":"0.00001","baseMaxSize":"10000000000",
	"quoteIncrement":"0.000001","quoteMinSize":"0.1","quoteMaxSize":"99999999",
	"priceIncrement":"0.1","priceLimitRate":"0.1","minFunds":"0.1"
}`

const fixtureIsolatedSymbol = `{
	"symbol":"BTC-USDT","symbolName":"BTC-USDT","baseCurrency":"BTC","quoteCurrency":"USDT",
	"maxLeverage":10,"flDebtRatio":"0.97","tradeEnable":true,"autoRenewMaxDebtRatio":"0.95",
	"baseBorrowEnable":true,"quoteBorrowEnable":true,"baseTransferInEnable":true,"quoteTransferInEnable":true
}`

const fixtureMarkPrice = `{"symbol":"BTC-USDT","granularity":5000,"timePoint":1729665839353,"value":"50000.1"}`

const fixtureMarginConfig = `{
	"currencyList":["BTC","USDT","ETH"],"maxLeverage":5,"warningDebtRatio":"0.95","liqDebtRatio":"0.97"
}`

const fixtureCrossAccount = `{
	"totalAssetOfQuoteCurrency":"40.86","totalLiabilityOfQuoteCurrency":"0","debtRatio":"0","status":"EFFECTIVE",
	"accounts":[{
		"currency":"USDT","total":"38.68","available":"20.01","hold":"18.66",
		"liability":"0","liabilityPrincipal":"0","liabilityInterest":"0",
		"maxBorrowSize":"163","borrowEnabled":true,"transferInEnabled":true
	}]
}`

const fixtureIsolatedAccount = `{
	"totalAssetOfQuoteCurrency":"4.97","totalLiabilityOfQuoteCurrency":"0.00038891","timestamp":1747303659773,
	"assets":[{
		"symbol":"BTC-USDT","status":"EFFECTIVE","debtRatio":"0",
		"baseAsset":{"currency":"BTC","borrowEnabled":true,"transferInEnabled":true,"liability":"0","liabilityPrincipal":"0","liabilityInterest":"0","total":"0","available":"0","hold":"0","maxBorrowSize":"0"},
		"quoteAsset":{"currency":"USDT","borrowEnabled":true,"transferInEnabled":true,"liability":"0.00038891","liabilityPrincipal":"0.00038888","liabilityInterest":"0.00000003","total":"4.97","available":"4.97","hold":"0","maxBorrowSize":"44"}
	}]
}`

const fixtureCrossRiskLimit = `{
	"timestamp":1729678659275,"currency":"BTC","borrowMaxAmount":"75.15","buyMaxAmount":"217.12",
	"holdMaxAmount":"217.12","borrowCoefficient":"1","marginCoefficient":"1","precision":8,
	"borrowMinAmount":"0.001","borrowMinUnit":"0.0001","borrowEnabled":true
}`

const fixtureMarginOrder = `{
	"id":"671663e02188630007e21c9c","clientOid":"abc","symbol":"BTC-USDT","type":"limit","side":"buy",
	"price":"50000","size":"0.1","funds":"0","dealFunds":"50","dealSize":"0.001","fee":"0.05","feeCurrency":"USDT",
	"timeInForce":"GTC","tradeType":"MARGIN_TRADE","postOnly":true,"hidden":false,"iceberg":false,
	"visibleSize":"0","cancelAfter":0,"stp":"","active":true,"cancelExist":false,
	"createdAt":1700000000000,"lastUpdatedAt":1700000000500
}`

const fixtureMarginFill = `{
	"tradeId":"t1","orderId":"671663e02188630007e21c9c","symbol":"BTC-USDT","side":"buy","liquidity":"maker",
	"forceTaker":false,"price":"50000","size":"0.001","funds":"50","fee":"0.05","feeRate":"0.001","feeCurrency":"USDT",
	"type":"limit","tradeType":"MARGIN_TRADE","createdAt":1700000000000
}`
