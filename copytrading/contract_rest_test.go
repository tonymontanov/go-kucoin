/*
FILE: copytrading/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the futures Copy-Trading REST surface. The mock HTTP
server stands in for the futures host (parent.REST() is bound to cfg.REST.BaseURL
= the mock). Exercises path/body/query assembly, the bare string/bool data
shapes, the position payload, the signed-auth gate and validation.
*/

package copytrading

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	cttypes "github.com/tonymontanov/go-kucoin/v2/copytrading/types"
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

func (r *recorded) set(k, b, q string) { r.mu.Lock(); r.body[k] = b; r.query[k] = q; r.mu.Unlock() }
func (r *recorded) getBody(k string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body[k]
}
func (r *recorded) getQuery(k string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query[k]
}

const fixturePosition = `{"id":"400000000001993893","symbol":"XBTUSDTM","autoDeposit":true,"maintMarginReq":"0.004","riskLimit":100000,"realLeverage":"7.20","crossMode":false,"marginMode":"ISOLATED","positionSide":"LONG","leverage":"7.20","delevPercentage":0.0,"openingTimestamp":1760667597842,"currentTimestamp":1760667659286,"currentQty":1,"currentCost":"108.4412","currentComm":"0.06506472","unrealisedCost":"108.4412","isOpen":true,"markPrice":"108459.69","markValue":"108.4596900000","posCost":"108.4412","posMargin":"15.036766663","posMaint":"0.4989145740","maintMargin":"15.0552566630","realisedPnl":"-0.0650647200","unrealisedPnl":"0.0184900000","unrealisedPnlPcnt":"0.0002","unrealisedRoePcnt":"0.0020","avgEntryPrice":"108441.20","liquidationPrice":"93836.08","bankruptPrice":"93404.44","settleCurrency":"USDT"}`

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recorded) {
	t.Helper()
	var rec = &recorded{body: map[string]string{}, query: map[string]string{}}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v1/copy-trade/futures/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var b, _ = io.ReadAll(r.Body)
			rec.set("POST /orders", string(b), "")
			writeEnv(w, `{"orderId":"368516378027724800","clientOid":"ea391ca47fb2448095b6d1aaf968baf8"}`)
		case http.MethodDelete:
			rec.set("DELETE /orders", "", r.URL.RawQuery)
			writeEnv(w, `{"cancelledOrderIds":["368516378027724800"]}`)
		}
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/orders/test", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /orders/test", string(b), "")
		writeEnv(w, `{"orderId":"t1","clientOid":"c1"}`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/st-orders", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /st-orders", string(b), "")
		writeEnv(w, `{"orderId":"368518618981363712","clientOid":"fa928420a7a0411cb50835e14d123943"}`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/orders/client-order", func(w http.ResponseWriter, r *http.Request) {
		rec.set("DELETE /client-order", "", r.URL.RawQuery)
		writeEnv(w, `{"clientOid":"c1"}`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/get-max-open-size", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/get-max-open-size", "", r.URL.RawQuery)
		writeEnv(w, `{"symbol":"XBTUSDTM","maxBuyOpenSize":"1000000","maxSellOpenSize":"51"}`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/position/margin/max-withdraw-margin", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/max-withdraw-margin", "", r.URL.RawQuery)
		writeEnv(w, `"15.0367"`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/position/margin/deposit-margin", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /deposit-margin", string(b), "")
		writeEnv(w, fixturePosition)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/position/margin/withdraw-margin", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /withdraw-margin", string(b), "")
		writeEnv(w, `"3"`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/position/risk-limit-level/change", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /risk-limit", string(b), "")
		writeEnv(w, `true`)
	})
	mux.HandleFunc("/api/v1/copy-trade/futures/position/margin/auto-deposit-status", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /auto-deposit", string(b), "")
		writeEnv(w, `true`)
	})

	var srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	var cfg = kucoin.DefaultConfig()
	cfg.REST.BaseURL = srv.URL
	if withCreds {
		cfg.APIKey, cfg.SecretKey, cfg.Passphrase = "k", "s", "p"
	}
	var parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClient(parent), rec
}

func TestContractREST_PlaceOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.PlaceOrder(context.Background(), cttypes.OrderRequest{
		ClientOid: "ea391ca47fb2448095b6d1aaf968baf8", Symbol: "XBTUSDTM",
		MarginMode: "ISOLATED", Leverage: "12", PositionSide: "LONG",
		Side: "buy", Type: "market", Size: "1", ReduceOnly: false,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.OrderID != "368516378027724800" {
		t.Errorf("res = %+v", res)
	}
	var b = rec.getBody("POST /orders")
	if !strings.Contains(b, `"positionSide":"LONG"`) || !strings.Contains(b, `"size":"1"`) {
		t.Errorf("body = %s", b)
	}
}

func TestContractREST_PlaceOrderValidation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	// limit without price
	var _, err = c.PlaceOrder(context.Background(), cttypes.OrderRequest{
		ClientOid: "x", Symbol: "XBTUSDTM", Side: "buy", Type: "limit", Size: "1",
	})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_PlaceTPSLOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.PlaceTPSLOrder(context.Background(), cttypes.TPSLOrderRequest{
		OrderRequest: cttypes.OrderRequest{
			ClientOid: "fa928420a7a0411cb50835e14d123943", Symbol: "XBTUSDTM",
			MarginMode: "ISOLATED", Leverage: "12", PositionSide: "LONG",
			Side: "buy", Type: "market", Size: "1",
		},
		StopPriceType: "TP", TriggerStopUpPrice: "111000", TriggerStopDownPrice: "100000",
	})
	if err != nil {
		t.Fatalf("PlaceTPSLOrder: %v", err)
	}
	if res.OrderID != "368518618981363712" {
		t.Errorf("res = %+v", res)
	}
	var b = rec.getBody("POST /st-orders")
	if !strings.Contains(b, `"stopPriceType":"TP"`) || !strings.Contains(b, `"triggerStopUpPrice":"111000"`) {
		t.Errorf("body = %s", b)
	}
}

func TestContractREST_CancelOrder(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.CancelOrder(context.Background(), "368516378027724800")
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if len(res.CancelledOrderIDs) != 1 || res.CancelledOrderIDs[0] != "368516378027724800" {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(rec.getQuery("DELETE /orders"), "orderId=368516378027724800") {
		t.Errorf("query = %s", rec.getQuery("DELETE /orders"))
	}
}

func TestContractREST_CancelByClientOid(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var coid, err = c.CancelOrderByClientOid(context.Background(), "c1", "XBTUSDTM")
	if err != nil {
		t.Fatalf("CancelOrderByClientOid: %v", err)
	}
	if coid != "c1" {
		t.Errorf("coid = %s", coid)
	}
	var q = rec.getQuery("DELETE /client-order")
	if !strings.Contains(q, "clientOid=c1") || !strings.Contains(q, "symbol=XBTUSDTM") {
		t.Errorf("query = %s", q)
	}
}

func TestContractREST_GetMaxOpenSize(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var m, err = c.GetMaxOpenSize(context.Background(), "XBTUSDTM", "108000", "12")
	if err != nil {
		t.Fatalf("GetMaxOpenSize: %v", err)
	}
	if m.MaxBuyOpenSize.String() != "1000000" || m.MaxSellOpenSize.String() != "51" {
		t.Errorf("m = %+v", m)
	}
	if !strings.Contains(rec.getQuery("/get-max-open-size"), "leverage=12") {
		t.Errorf("query = %s", rec.getQuery("/get-max-open-size"))
	}
}

func TestContractREST_MarginOps(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var ctx = context.Background()

	var mw, err = c.GetMaxWithdrawMargin(ctx, "XBTUSDTM", "LONG")
	if err != nil || mw.String() != "15.0367" {
		t.Fatalf("GetMaxWithdrawMargin = %v err %v", mw, err)
	}

	var pos, perr = c.AddIsolatedMargin(ctx, cttypes.AddMarginRequest{Symbol: "XBTUSDTM", Margin: "3", BizNo: "112233", PositionSide: "LONG"})
	if perr != nil {
		t.Fatalf("AddIsolatedMargin: %v", perr)
	}
	if pos.MarginMode != "ISOLATED" || pos.AvgEntryPrice.String() != "108441.2" || pos.RiskLimit != 100000 {
		t.Errorf("pos = %+v", pos)
	}
	if !strings.Contains(rec.getBody("POST /deposit-margin"), `"bizNo":"112233"`) {
		t.Errorf("body = %s", rec.getBody("POST /deposit-margin"))
	}

	var rm, rerr = c.RemoveIsolatedMargin(ctx, "XBTUSDTM", "3", "LONG")
	if rerr != nil || rm.String() != "3" {
		t.Fatalf("RemoveIsolatedMargin = %v err %v", rm, rerr)
	}

	var ok, lerr = c.ModifyRiskLimitLevel(ctx, "XBTUSDTM", 2)
	if lerr != nil || !ok {
		t.Fatalf("ModifyRiskLimitLevel = %v err %v", ok, lerr)
	}
	if !strings.Contains(rec.getBody("POST /risk-limit"), `"level":2`) {
		t.Errorf("body = %s", rec.getBody("POST /risk-limit"))
	}

	var ad, aerr = c.SetAutoDepositStatus(ctx, "XBTUSDTM", true, "LONG")
	if aerr != nil || !ad {
		t.Fatalf("SetAutoDepositStatus = %v err %v", ad, aerr)
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var _, err = c.GetMaxOpenSize(context.Background(), "XBTUSDTM", "1", "1")
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
