/*
FILE: margin/trading_test.go

DESCRIPTION:
Unit tests for the HF-margin order-body assembly and validation: required
fields, limit vs market sizing rules, the clientOid default and the
cross/isolated isIsolated derivation.
*/

package margin

import (
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
)

// newClientForBody builds a margin client (no transport needed) with the given
// default trade type.
func newClientForBody(t *testing.T, tt margintypes.TradeType) *Client {
	t.Helper()
	var parent, err = kucoin.NewClient(kucoin.DefaultConfig())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClientWithSettings(parent, ClientSettings{DefaultTradeType: tt})
}

func TestBuildOrderBody_Validation(t *testing.T) {
	var c = newClientForBody(t, margintypes.TradeCross)
	var cases = []struct {
		name string
		req  margintypes.CreateOrderRequest
	}{
		{"noSymbol", margintypes.CreateOrderRequest{Side: margintypes.SideBuy, Type: margintypes.OrderLimit}},
		{"noSide", margintypes.CreateOrderRequest{Symbol: "BTC-USDT", Type: margintypes.OrderLimit}},
		{"noType", margintypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: margintypes.SideBuy}},
		{"limitNoPrice", margintypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderLimit, Size: requireDec("1")}},
		{"marketSizeAndFunds", margintypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderMarket, Size: requireDec("1"), Funds: requireDec("1")}},
		{"marketNeither", margintypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderMarket}},
	}
	var tc = cases[0]
	for _, tc = range cases {
		var _, err = c.buildOrderBody(tc.req)
		if err == nil || !kucoin.IsInvalidRequest(err) {
			t.Errorf("%s: expected invalid-request, got %v", tc.name, err)
		}
	}
}

func TestBuildOrderBody_ClientOidAndIsolated(t *testing.T) {
	var c = newClientForBody(t, margintypes.TradeIsolated)
	var b, err = c.buildOrderBody(margintypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: margintypes.SideBuy, Type: margintypes.OrderLimit,
		Price: requireDec("50000"), Size: requireDec("0.1"),
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if b.ClientOid == "" {
		t.Error("clientOid should be auto-generated")
	}
	if !b.IsIsolated {
		t.Error("isolated default should set IsIsolated")
	}
	if b.Price != "50000" || b.Size != "0.1" {
		t.Errorf("price/size = %s / %s", b.Price, b.Size)
	}

	// Explicit IsIsolated overrides a cross default.
	var c2 = newClientForBody(t, margintypes.TradeCross)
	var b2, _ = c2.buildOrderBody(margintypes.CreateOrderRequest{
		Symbol: "ETH-USDT", Side: margintypes.SideSell, Type: margintypes.OrderMarket,
		Size: requireDec("1"), IsIsolated: true, ClientOrderID: "fixed",
	})
	if !b2.IsIsolated || b2.ClientOid != "fixed" {
		t.Errorf("b2 = %+v", b2)
	}
}
