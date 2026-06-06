/*
FILE: affiliate/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Affiliate REST surface (mock HTTP server,
end-to-end through the SDK transport).
*/

package affiliate

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"context"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	afftypes "github.com/tonymontanov/go-kucoin/v2/affiliate/types"
)

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

type recorded struct {
	mu    sync.Mutex
	query map[string]string
}

func (r *recorded) set(p, q string) { r.mu.Lock(); r.query[p] = q; r.mu.Unlock() }
func (r *recorded) get(p string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query[p]
}

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recorded) {
	t.Helper()
	var rec = &recorded{query: map[string]string{}}
	var mux = http.NewServeMux()
	mux.HandleFunc("/api/v2/affiliate/queryMyCommission", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v2/affiliate/queryMyCommission", r.URL.RawQuery)
		writeEnv(w, `[{"siteType":"global","rebateType":1,"payoutTime":1609516800000,"periodStartTime":1609430400000,"periodEndTime":1609516800000,"status":1,"takerVolume":"20000.02","makerVolume":"40000.04","commission":"5.05","currency":"USDT"}]`)
	})
	mux.HandleFunc("/api/v2/affiliate/inviter/statistics", func(w http.ResponseWriter, r *http.Request) {
		rec.set("/api/v2/affiliate/inviter/statistics", r.URL.RawQuery)
		writeEnv(w, `[{"m1Uid":"2400xxx499","rcode":"rrxx4A","m2Uid":"9xx6","amount":"8000.16","rebate":"800.16","cashBack":"100.16","offset":"66271f01400000"}]`)
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

func TestContractREST_GetCommission(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var got, err = c.GetCommission(context.Background(), afftypes.CommissionQuery{
		RebateFrom: 1609430400000, RebateTo: 1609516800000, Page: 1, PageSize: 10, SiteType: "global",
	})
	if err != nil {
		t.Fatalf("GetCommission: %v", err)
	}
	if len(got) != 1 || got[0].Currency != "USDT" || got[0].Commission.String() != "5.05" {
		t.Fatalf("commission = %+v", got)
	}
	var q = rec.get("/api/v2/affiliate/queryMyCommission")
	if !strings.Contains(q, "rebateStartAt=1609430400000") || !strings.Contains(q, "siteType=global") {
		t.Errorf("query = %s", q)
	}
}

func TestContractREST_GetInviterRebate(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var got, err = c.GetInviterRebate(context.Background(), "20250601", "", 0)
	if err != nil {
		t.Fatalf("GetInviterRebate: %v", err)
	}
	if len(got) != 1 || got[0].M1UID != "2400xxx499" || got[0].Rebate.String() != "800.16" {
		t.Fatalf("rebate = %+v", got)
	}
	if !strings.Contains(rec.get("/api/v2/affiliate/inviter/statistics"), "date=20250601") {
		t.Errorf("query = %s", rec.get("/api/v2/affiliate/inviter/statistics"))
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var _, err = c.GetCommission(context.Background(), afftypes.CommissionQuery{})
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
