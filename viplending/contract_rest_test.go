/*
FILE: viplending/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the VIP Lending (OTC loan) REST surface. A mock HTTP
server returns recorded KuCoin response bodies (wrapped in the { code, data }
envelope); the tests drive the real client end-to-end and assert the typed
output — path construction, envelope parsing, wire→type converters and the
signed-auth gate.
*/

package viplending

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
)

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

const (
	fixtureDiscount = `[{"currency":"BTC","usdtLevels":[{"left":0,"right":20000000,"discountRate":"1.00000000"},{"left":20000000,"right":50000000,"discountRate":"0.95000000"}]}]`
	fixtureLoan     = `{"parentUid":"1260004199","orders":[{"orderId":"671a","principal":"100","interest":"0","currency":"USDT"}],"ltv":{"transferLtv":"0.6000","onlyClosePosLtv":"0.7500","delayedLiquidationLtv":"0.7500","instantLiquidationLtv":"0.8000","currentLtv":"0.1111"},"totalMarginAmount":"900.00000000","transferMarginAmount":"166.66666666","margins":[{"marginCcy":"USDT","marginQty":"1000.00000000","marginFactor":"0.9000000000"}]}`
	fixtureAccounts = `[{"uid":"1260004199","marginCcy":"USDT","marginQty":"900","marginFactor":"0.9000000000","accountType":"TRADE","isParent":true}]`
)

func newMockRESTClient(t *testing.T, withCreds bool) *Client {
	t.Helper()
	var mux = http.NewServeMux()
	mux.HandleFunc("/api/v1/otc-loan/discount-rate-configs", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureDiscount)
	})
	mux.HandleFunc("/api/v1/otc-loan/loan", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureLoan)
	})
	mux.HandleFunc("/api/v1/otc-loan/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureAccounts)
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
	return NewClient(parent)
}

func TestContractREST_GetCollateralConfigs(t *testing.T) {
	var c = newMockRESTClient(t, true)
	var got, err = c.GetCollateralConfigs(context.Background())
	if err != nil {
		t.Fatalf("GetCollateralConfigs: %v", err)
	}
	if len(got) != 1 || got[0].Currency != "BTC" || len(got[0].UsdtLevels) != 2 {
		t.Fatalf("configs = %+v", got)
	}
	if got[0].UsdtLevels[1].Right != 50000000 || got[0].UsdtLevels[1].DiscountRate.String() != "0.95" {
		t.Errorf("level = %+v", got[0].UsdtLevels[1])
	}
}

func TestContractREST_GetLoanInfo(t *testing.T) {
	var c = newMockRESTClient(t, true)
	var info, err = c.GetLoanInfo(context.Background())
	if err != nil {
		t.Fatalf("GetLoanInfo: %v", err)
	}
	if info.ParentUID != "1260004199" || len(info.Orders) != 1 || len(info.Margins) != 1 {
		t.Fatalf("info = %+v", info)
	}
	if info.Ltv.CurrentLtv.String() != "0.1111" || info.Orders[0].Principal.String() != "100" {
		t.Errorf("info = %+v", info)
	}
	if info.Margins[0].MarginCcy != "USDT" || info.TotalMarginAmount.String() != "900" {
		t.Errorf("margins = %+v total = %s", info.Margins[0], info.TotalMarginAmount)
	}
}

func TestContractREST_GetAccounts(t *testing.T) {
	var c = newMockRESTClient(t, true)
	var got, err = c.GetAccounts(context.Background())
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(got) != 1 || got[0].UID != "1260004199" || !got[0].IsParent {
		t.Fatalf("accounts = %+v", got)
	}
	if got[0].MarginCcy != "USDT" || got[0].MarginFactor.String() != "0.9" {
		t.Errorf("account = %+v", got[0])
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c = newMockRESTClient(t, false) // no creds
	var _, err = c.GetAccounts(context.Background())
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
