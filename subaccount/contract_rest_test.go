/*
FILE: subaccount/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Sub-Account management REST surface. A mock HTTP
server returns recorded KuCoin response bodies (wrapped in the { code, data }
envelope); the tests drive the real client end-to-end through the SDK transport
and assert the typed output — exercising path construction, envelope parsing,
the wire→type converters, request-body/query assembly, the signed-auth gate and
the flexInt64 createdAt handling (number vs string).
*/

package subaccount

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	subtypes "github.com/tonymontanov/go-kucoin/v2/subaccount/types"
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

	mux.HandleFunc("/api/v2/sub/user/created", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v2/sub/user/created", string(b), "")
		writeEnv(w, `{"uid":245730746,"subName":"subNameTest1","remarks":"TheRemark","access":"Spot"}`)
	})
	mux.HandleFunc("/api/v3/sub/user/margin/enable", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v3/sub/user/margin/enable", string(b), "")
		writeEnv(w, `{"uid":"245730746"}`)
	})
	mux.HandleFunc("/api/v3/sub/user/futures/enable", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v3/sub/user/futures/enable", string(b), "")
		writeEnv(w, `{"uid":"245730746"}`)
	})
	mux.HandleFunc("/api/v2/sub/user", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v2/sub/user", "", r.URL.RawQuery)
		// createdAt as a bare number here.
		writeEnv(w, `{"currentPage":1,"pageSize":10,"totalNum":1,"totalPage":1,"items":[{"userId":"635002438793b80001dcc8b3","uid":62356,"subName":"margin1","status":2,"type":0,"access":"Margin","remarks":"hi","createdAt":1666147844000}]}`)
	})
	mux.HandleFunc("/api/v1/sub-accounts/635002438793b80001dcc8b3", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v1/sub-accounts/id", "", r.URL.RawQuery)
		writeEnv(w, `{"subUserId":"635002438793b80001dcc8b3","subName":"margin1","mainAccounts":[{"currency":"USDT","balance":"8.21","available":"8.21","holds":"0","baseCurrency":"BTC","baseCurrencyPrice":"9979.95","baseAmount":"0.00082"}],"tradeAccounts":[],"marginAccounts":[]}`)
	})
	mux.HandleFunc("/api/v2/sub-accounts", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v2/sub-accounts", "", r.URL.RawQuery)
		writeEnv(w, `{"currentPage":1,"pageSize":10,"totalNum":1,"totalPage":1,"items":[{"subUserId":"635002438793b80001dcc8b3","subName":"margin1","mainAccounts":[{"currency":"USDT","balance":"8.21","available":"8.21","holds":"0","baseCurrency":"BTC","baseCurrencyPrice":"9979.95","baseAmount":"0.00082"}],"tradeAccounts":[],"marginAccounts":[]}]}`)
	})
	mux.HandleFunc("/api/v1/sub/api-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var b, _ = io.ReadAll(r.Body)
			rec.set("POST /api/v1/sub/api-key", string(b), "")
			// createdAt as a quoted string here → exercise flexInt64.
			writeEnv(w, `{"subName":"AAAAAAAAAA0007","remark":"remark","apiKey":"63032453e75087000182982a","apiSecret":"secret","passphrase":"bingo","permission":"General","ipWhitelist":"1.1.1.1","createdAt":"1661150868000"}`)
		case http.MethodGet:
			rec.set("GET /api/v1/sub/api-key", "", r.URL.RawQuery)
			writeEnv(w, `[{"subName":"AAAAAAAAAA0007","remark":"remark","apiKey":"63032453e75087000182982a","permission":"General","ipWhitelist":"1.1.1.1","createdAt":1661150868000}]`)
		case http.MethodDelete:
			rec.set("DELETE /api/v1/sub/api-key", "", r.URL.RawQuery)
			writeEnv(w, `{"subName":"AAAAAAAAAA0007","apiKey":"63032453e75087000182982a"}`)
		}
	})
	mux.HandleFunc("/api/v1/sub/api-key/update", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("POST /api/v1/sub/api-key/update", string(b), "")
		writeEnv(w, `{"subName":"AAAAAAAAAA0007","apiKey":"63032453e75087000182982a","permission":"General","ipWhitelist":"1.1.1.1,2.2.2.2"}`)
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

func TestContractREST_Create(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Create(context.Background(), subtypes.CreateRequest{
		SubName: "subNameTest1", Password: "q1234567", Access: "Spot", Remarks: "TheRemark",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.UID != 245730746 || res.SubName != "subNameTest1" || res.Access != "Spot" {
		t.Errorf("res = %+v", res)
	}
	var body = rec.getBody("POST /api/v2/sub/user/created")
	if !strings.Contains(body, `"subName":"subNameTest1"`) || !strings.Contains(body, `"remarks":"TheRemark"`) {
		t.Errorf("body = %s", body)
	}
}

func TestContractREST_CreateValidation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.Create(context.Background(), subtypes.CreateRequest{SubName: "x"})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_Permissions(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	if err := c.EnableMargin(context.Background(), "245730746"); err != nil {
		t.Fatalf("EnableMargin: %v", err)
	}
	if err := c.EnableFutures(context.Background(), "245730746"); err != nil {
		t.Fatalf("EnableFutures: %v", err)
	}
	if !strings.Contains(rec.getBody("POST /api/v3/sub/user/margin/enable"), `"uid":"245730746"`) {
		t.Errorf("margin body = %s", rec.getBody("POST /api/v3/sub/user/margin/enable"))
	}
}

func TestContractREST_GetSummaries(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var page, err = c.GetSummaries(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("GetSummaries: %v", err)
	}
	if page.TotalNum != 1 || len(page.Items) != 1 || page.Items[0].UID != 62356 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].Access != "Margin" || page.Items[0].CreatedAt != 1666147844000 {
		t.Errorf("item = %+v", page.Items[0])
	}
	if !strings.Contains(rec.getQuery("/api/v2/sub/user"), "currentPage=1") {
		t.Errorf("query = %s", rec.getQuery("/api/v2/sub/user"))
	}
}

func TestContractREST_GetBalance(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var a, err = c.GetBalance(context.Background(), "635002438793b80001dcc8b3", true)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if a.SubName != "margin1" || len(a.MainAccounts) != 1 {
		t.Fatalf("assets = %+v", a)
	}
	if a.MainAccounts[0].Currency != "USDT" || a.MainAccounts[0].Balance.String() != "8.21" {
		t.Errorf("leg = %+v", a.MainAccounts[0])
	}
	if !strings.Contains(rec.getQuery("/api/v1/sub-accounts/id"), "includeBaseAmount=true") {
		t.Errorf("query = %s", rec.getQuery("/api/v1/sub-accounts/id"))
	}
}

func TestContractREST_GetBalances(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var page, err = c.GetBalances(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	if page.TotalNum != 1 || len(page.Items) != 1 || page.Items[0].MainAccounts[0].BaseCurrency != "BTC" {
		t.Fatalf("page = %+v", page)
	}
}

func TestContractREST_CreateAPIKey(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var k, err = c.CreateAPIKey(context.Background(), subtypes.CreateAPIKeyRequest{
		SubName: "AAAAAAAAAA0007", Passphrase: "bingo", Remark: "remark", Permission: "General",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	// Secret + passphrase returned once; flexInt64 createdAt from a quoted string.
	if k.APISecret != "secret" || k.Passphrase != "bingo" || k.CreatedAt != 1661150868000 {
		t.Errorf("key = %+v", k)
	}
	if !strings.Contains(rec.getBody("POST /api/v1/sub/api-key"), `"remark":"remark"`) {
		t.Errorf("body = %s", rec.getBody("POST /api/v1/sub/api-key"))
	}
}

func TestContractREST_GetAPIKeys(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var keys, err = c.GetAPIKeys(context.Background(), "AAAAAAAAAA0007", "")
	if err != nil {
		t.Fatalf("GetAPIKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].APIKey != "63032453e75087000182982a" || keys[0].CreatedAt != 1661150868000 {
		t.Errorf("keys = %+v", keys)
	}
}

func TestContractREST_UpdateAPIKey(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var k, err = c.UpdateAPIKey(context.Background(), subtypes.UpdateAPIKeyRequest{
		SubName: "AAAAAAAAAA0007", APIKey: "63032453e75087000182982a", Passphrase: "bingo", IPWhitelist: "1.1.1.1,2.2.2.2",
	})
	if err != nil {
		t.Fatalf("UpdateAPIKey: %v", err)
	}
	if k.IPWhitelist != "1.1.1.1,2.2.2.2" {
		t.Errorf("key = %+v", k)
	}
	if !strings.Contains(rec.getBody("POST /api/v1/sub/api-key/update"), `"ipWhitelist":"1.1.1.1,2.2.2.2"`) {
		t.Errorf("body = %s", rec.getBody("POST /api/v1/sub/api-key/update"))
	}
}

func TestContractREST_DeleteAPIKey(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var k, err = c.DeleteAPIKey(context.Background(), "AAAAAAAAAA0007", "bingo", "63032453e75087000182982a")
	if err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	if k.APIKey != "63032453e75087000182982a" {
		t.Errorf("key = %+v", k)
	}
	var q = rec.getQuery("DELETE /api/v1/sub/api-key")
	if !strings.Contains(q, "apiKey=63032453e75087000182982a") || !strings.Contains(q, "passphrase=bingo") {
		t.Errorf("query = %s", q)
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var _, err = c.GetSummaries(context.Background(), 1, 10)
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
