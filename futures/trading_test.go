/*
FILE: futures/trading_test.go

DESCRIPTION:
White-box tests for the place-order body builder: required-field
validation, client default application (leverage / margin mode), clientOid
auto-generation, market-vs-limit price handling and stop-field mapping.
*/

package futures

import (
	"testing"

	"github.com/shopspring/decimal"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
)

func newTestClient(t *testing.T, s ClientSettings) *Client {
	t.Helper()
	var parent, err = kucoin.NewClient(kucoin.DefaultConfig())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClientWithSettings(parent, s)
}

func TestBuildOrderBody_Validation(t *testing.T) {
	var c = newTestClient(t, ClientSettings{DefaultLeverage: "5"})
	var cases = []struct {
		name string
		req  futurestypes.CreateOrderRequest
	}{
		{"no symbol", futurestypes.CreateOrderRequest{Side: futurestypes.SideBuy, Type: futurestypes.OrderLimit, Size: 1, Price: decimal.NewFromInt(1)}},
		{"no side", futurestypes.CreateOrderRequest{Symbol: "XBTUSDTM", Type: futurestypes.OrderLimit, Size: 1, Price: decimal.NewFromInt(1)}},
		{"no type", futurestypes.CreateOrderRequest{Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Size: 1, Price: decimal.NewFromInt(1)}},
		{"zero size", futurestypes.CreateOrderRequest{Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderLimit, Price: decimal.NewFromInt(1)}},
		{"limit no price", futurestypes.CreateOrderRequest{Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderLimit, Size: 1}},
	}
	var i int
	for i = 0; i < len(cases); i++ {
		var tc = cases[i]
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.buildOrderBody(tc.req); err == nil {
				t.Fatalf("expected validation error for %q", tc.name)
			}
		})
	}
}

func TestBuildOrderBody_LeverageRequired(t *testing.T) {
	var c = newTestClient(t, ClientSettings{}) // no default leverage
	var _, err = c.buildOrderBody(futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderMarket, Size: 1,
	})
	if err == nil {
		t.Fatal("expected leverage-required error")
	}
}

func TestBuildOrderBody_DefaultsApplied(t *testing.T) {
	var c = newTestClient(t, ClientSettings{DefaultLeverage: "7", DefaultMarginMode: futurestypes.MarginCross})
	var body, err = c.buildOrderBody(futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderLimit,
		Size: 3, Price: decimal.RequireFromString("50000.5"),
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.Leverage != "7" {
		t.Errorf("leverage = %q, want 7", body.Leverage)
	}
	if body.MarginMode != string(futurestypes.MarginCross) {
		t.Errorf("marginMode = %q, want CROSS", body.MarginMode)
	}
	if body.ClientOid == "" {
		t.Error("clientOid must be auto-generated")
	}
	if body.Price != "50000.5" {
		t.Errorf("price = %q, want 50000.5", body.Price)
	}
	if body.Size != 3 {
		t.Errorf("size = %d, want 3", body.Size)
	}
}

func TestBuildOrderBody_MarketOmitsPrice(t *testing.T) {
	var c = newTestClient(t, ClientSettings{DefaultLeverage: "5"})
	var body, err = c.buildOrderBody(futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideSell, Type: futurestypes.OrderMarket, Size: 2,
		Price: decimal.NewFromInt(999), // should be ignored for market
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.Price != "" {
		t.Errorf("market order price = %q, want empty", body.Price)
	}
}

func TestBuildOrderBody_StopMapping(t *testing.T) {
	var c = newTestClient(t, ClientSettings{DefaultLeverage: "5"})
	var body, err = c.buildOrderBody(futurestypes.CreateOrderRequest{
		Symbol: "XBTUSDTM", Side: futurestypes.SideBuy, Type: futurestypes.OrderLimit,
		Size: 1, Price: decimal.NewFromInt(50000), ClientOrderID: "mine-1",
		Stop: futurestypes.StopUp, StopPriceType: futurestypes.StopPriceMark, StopPrice: decimal.RequireFromString("51000"),
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.ClientOid != "mine-1" {
		t.Errorf("clientOid = %q, want mine-1 (caller-supplied)", body.ClientOid)
	}
	if body.Stop != "up" || body.StopPriceType != "MP" || body.StopPrice != "51000" {
		t.Errorf("stop fields = (%q,%q,%q)", body.Stop, body.StopPriceType, body.StopPrice)
	}
}
