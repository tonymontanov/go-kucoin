/*
FILE: earn/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Earn REST surface. A mock HTTP server returns
recorded KuCoin response bodies (wrapped in the { code, data } envelope); the
tests drive the real client end-to-end through the SDK transport and assert the
typed output — exercising path construction, envelope parsing, the wire→type
converters, request-body/query assembly and the signed-auth gate.
*/

package earn

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
	earntypes "github.com/tonymontanov/go-kucoin/v2/earn/types"
)

func requireDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

type recordedBodies struct {
	mu    sync.Mutex
	body  map[string]string
	query map[string]string
}

func (r *recordedBodies) set(path, body, query string) {
	r.mu.Lock()
	r.body[path] = body
	r.query[path] = query
	r.mu.Unlock()
}

func (r *recordedBodies) getBody(path string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body[path]
}

func (r *recordedBodies) getQuery(path string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query[path]
}

const (
	fixtureProduct = `{"id":"2172","currency":"BTC","category":"DEMAND","type":"DEMAND","precision":8,"productUpperLimit":"480","productRemainAmount":"132.36153083","userUpperLimit":"20","userLowerLimit":"0.01","redeemPeriod":0,"lockStartTime":1644807600000,"lockEndTime":null,"applyStartTime":1644807600000,"applyEndTime":null,"returnRate":"0.00047208","incomeCurrency":"BTC","earlyRedeemSupported":0,"status":"ONGOING","redeemType":"MANUAL","incomeReleaseType":"DAILY","interestDate":1729267200000,"duration":0,"newUserOnly":0}`
	fixtureHolding = `{"orderId":"2767291","productId":"2611","productCategory":"KCS_STAKING","productType":"DEMAND","currency":"KCS","incomeCurrency":"KCS","returnRate":"0.03471727","holdAmount":"1","redeemedAmount":"0","redeemingAmount":"1","lockStartTime":1701252000000,"lockEndTime":null,"purchaseTime":1729257513000,"redeemPeriod":3,"status":"REDEEMING","earlyRedeemSupported":0}`
)

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recordedBodies) {
	t.Helper()
	var rec = &recordedBodies{body: map[string]string{}, query: map[string]string{}}
	var mux = http.NewServeMux()

	var productHandler = func(w http.ResponseWriter, r *http.Request) {
		rec.set(r.URL.Path, "", r.URL.RawQuery)
		writeEnv(w, `[`+fixtureProduct+`]`)
	}
	mux.HandleFunc("/api/v1/earn/saving/products", productHandler)
	mux.HandleFunc("/api/v1/earn/promotion/products", productHandler)
	mux.HandleFunc("/api/v1/earn/staking/products", productHandler)
	mux.HandleFunc("/api/v1/earn/kcs-staking/products", productHandler)
	mux.HandleFunc("/api/v1/earn/eth-staking/products", productHandler)

	mux.HandleFunc("/api/v1/earn/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var b, _ = io.ReadAll(r.Body)
			rec.set("POST /api/v1/earn/orders", string(b), "")
			writeEnv(w, `{"orderId":"2767291","orderTxId":"6603694"}`)
		case http.MethodDelete:
			rec.set("DELETE /api/v1/earn/orders", "", r.URL.RawQuery)
			writeEnv(w, `{"orderTxId":"6603700","deliverTime":1729517805000,"status":"PENDING","amount":"1"}`)
		}
	})
	mux.HandleFunc("/api/v1/earn/redeem-preview", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"currency":"KCS","redeemAmount":"1","penaltyInterestAmount":"0","redeemPeriod":3,"deliverTime":1729518951000,"manualRedeemable":true,"redeemAll":false}`)
	})
	mux.HandleFunc("/api/v1/earn/hold-assets", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/earn/hold-assets", "", r.URL.RawQuery)
		writeEnv(w, `{"totalNum":1,"totalPage":1,"currentPage":1,"pageSize":15,"items":[`+fixtureHolding+`]}`)
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

func TestContractREST_GetSavingsProducts(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var got, err = c.GetSavingsProducts(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("GetSavingsProducts: %v", err)
	}
	if len(got) != 1 || got[0].ID != "2172" || got[0].Currency != "BTC" {
		t.Fatalf("products = %+v", got)
	}
	if got[0].ReturnRate.String() != "0.00047208" || got[0].Status != "ONGOING" {
		t.Errorf("product = %+v", got[0])
	}
	// lockEndTime is null on the wire → 0.
	if got[0].LockEndTime != 0 {
		t.Errorf("null lockEndTime not zeroed: %d", got[0].LockEndTime)
	}
	if !strings.Contains(rec.getQuery("/api/v1/earn/saving/products"), "currency=BTC") {
		t.Errorf("currency query missing: %s", rec.getQuery("/api/v1/earn/saving/products"))
	}
}

func TestContractREST_AllProductFamilies(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var ctx = context.Background()
	var checks = []func() ([]earntypes.Product, error){
		func() ([]earntypes.Product, error) { return c.GetPromotionProducts(ctx, "") },
		func() ([]earntypes.Product, error) { return c.GetStakingProducts(ctx, "") },
		func() ([]earntypes.Product, error) { return c.GetKCSStakingProducts(ctx, "") },
		func() ([]earntypes.Product, error) { return c.GetETHStakingProducts(ctx, "") },
	}
	var i int
	for i = 0; i < len(checks); i++ {
		var got, err = checks[i]()
		if err != nil || len(got) != 1 {
			t.Fatalf("family %d: got %+v err %v", i, got, err)
		}
	}
}

func TestContractREST_Purchase(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Purchase(context.Background(), earntypes.PurchaseRequest{
		ProductID: "2611", Amount: requireDec("1"),
	})
	if err != nil {
		t.Fatalf("Purchase: %v", err)
	}
	if res.OrderID != "2767291" || res.OrderTxID != "6603694" {
		t.Errorf("res = %+v", res)
	}
	var body = rec.getBody("POST /api/v1/earn/orders")
	if !strings.Contains(body, `"productId":"2611"`) || !strings.Contains(body, `"amount":"1"`) || !strings.Contains(body, `"accountType":"MAIN"`) {
		t.Errorf("body = %s", body)
	}
}

func TestContractREST_Purchase_Validation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.Purchase(context.Background(), earntypes.PurchaseRequest{Amount: requireDec("1")})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_Redeem(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Redeem(context.Background(), earntypes.RedeemRequest{
		OrderID: "2767291", Amount: requireDec("1"), FromAccountType: "MAIN",
	})
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if res.Status != "PENDING" || res.OrderTxID != "6603700" || res.Amount.String() != "1" {
		t.Errorf("res = %+v", res)
	}
	var q = rec.getQuery("DELETE /api/v1/earn/orders")
	if !strings.Contains(q, "orderId=2767291") || !strings.Contains(q, "amount=1") || !strings.Contains(q, "fromAccountType=MAIN") {
		t.Errorf("query = %s", q)
	}
}

func TestContractREST_RedeemPreview(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var p, err = c.RedeemPreview(context.Background(), "2767291", "MAIN")
	if err != nil {
		t.Fatalf("RedeemPreview: %v", err)
	}
	if !p.ManualRedeemable || p.RedeemPeriod != 3 || p.Currency != "KCS" {
		t.Errorf("preview = %+v", p)
	}
}

func TestContractREST_GetHoldings(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var page, err = c.GetHoldings(context.Background(), earntypes.HoldingQuery{Currency: "KCS", CurrentPage: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("GetHoldings: %v", err)
	}
	if page.TotalNum != 1 || len(page.Items) != 1 || page.Items[0].Status != "REDEEMING" {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].RedeemingAmount.String() != "1" {
		t.Errorf("holding = %+v", page.Items[0])
	}
	var q = rec.getQuery("/api/v1/earn/hold-assets")
	if !strings.Contains(q, "currency=KCS") || !strings.Contains(q, "pageSize=10") {
		t.Errorf("query = %s", q)
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	var _, err = c.GetSavingsProducts(context.Background(), "")
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
