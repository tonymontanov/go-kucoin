/*
FILE: spot/trading_test.go

DESCRIPTION:
White-box tests for the spot place-order body builder: required-field
validation, limit vs market sizing rules (size/funds), trade-type and
clientOid defaults, and time-in-force / post-only mapping.
*/

package spot

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
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
	var c = newTestClient(t, ClientSettings{})
	var cases = []struct {
		name string
		req  spottypes.CreateOrderRequest
	}{
		{"no symbol", spottypes.CreateOrderRequest{Side: spottypes.SideBuy, Type: spottypes.OrderLimit, Size: decimal.NewFromInt(1), Price: decimal.NewFromInt(1)}},
		{"no side", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Type: spottypes.OrderLimit, Size: decimal.NewFromInt(1), Price: decimal.NewFromInt(1)}},
		{"no type", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: spottypes.SideBuy, Size: decimal.NewFromInt(1), Price: decimal.NewFromInt(1)}},
		{"limit no price", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderLimit, Size: decimal.NewFromInt(1)}},
		{"limit no size", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderLimit, Price: decimal.NewFromInt(1)}},
		{"market no size/funds", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderMarket}},
		{"market both size+funds", spottypes.CreateOrderRequest{Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderMarket, Size: decimal.NewFromInt(1), Funds: decimal.NewFromInt(10)}},
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

func TestBuildOrderBody_LimitDefaults(t *testing.T) {
	var c = newTestClient(t, ClientSettings{})
	var body, err = c.buildOrderBody(spottypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderLimit,
		Size: decimal.RequireFromString("0.5"), Price: decimal.RequireFromString("50000.1"),
		TimeInForce: spottypes.GTC, PostOnly: true,
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.TradeType != string(spottypes.TradeSpot) {
		t.Errorf("tradeType = %q, want TRADE", body.TradeType)
	}
	if body.ClientOid == "" {
		t.Error("clientOid must be auto-generated")
	}
	if body.Price != "50000.1" || body.Size != "0.5" {
		t.Errorf("price/size = %q/%q", body.Price, body.Size)
	}
	if !body.PostOnly || body.TimeInForce != "GTC" {
		t.Errorf("postOnly/tif = %v/%q", body.PostOnly, body.TimeInForce)
	}
	if body.Funds != "" {
		t.Errorf("limit order funds = %q, want empty", body.Funds)
	}
}

func TestBuildOrderBody_MarketByFunds(t *testing.T) {
	var c = newTestClient(t, ClientSettings{})
	var body, err = c.buildOrderBody(spottypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: spottypes.SideBuy, Type: spottypes.OrderMarket,
		Funds: decimal.RequireFromString("100"),
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.Funds != "100" {
		t.Errorf("funds = %q, want 100", body.Funds)
	}
	if body.Size != "" || body.Price != "" {
		t.Errorf("market-by-funds must omit size/price, got size=%q price=%q", body.Size, body.Price)
	}
}

func TestBuildOrderBody_MarketBySize(t *testing.T) {
	var c = newTestClient(t, ClientSettings{DefaultTradeType: spottypes.TradeMargin})
	var body, err = c.buildOrderBody(spottypes.CreateOrderRequest{
		Symbol: "BTC-USDT", Side: spottypes.SideSell, Type: spottypes.OrderMarket,
		Size: decimal.RequireFromString("0.25"),
	})
	if err != nil {
		t.Fatalf("buildOrderBody: %v", err)
	}
	if body.Size != "0.25" || body.Funds != "" {
		t.Errorf("market-by-size = size %q funds %q", body.Size, body.Funds)
	}
	if body.TradeType != string(spottypes.TradeMargin) {
		t.Errorf("tradeType = %q, want MARGIN_TRADE (client default)", body.TradeType)
	}
}

// TestDecodeBatchRows_NestedDataObject locks the regression that flooded the
// book: KuCoin returns the multi-order rows nested under {"data":[...]} and
// the SDK previously decoded the envelope payload as a bare array, failing
// with "expect [ or n, but found {" even though the orders were accepted.
func TestDecodeBatchRows_NestedDataObject(t *testing.T) {
	var raw = json.RawMessage(`{"data":[
		{"symbol":"PARTI-USDT","orderId":"abc123","clientOid":"c1","status":"success"},
		{"symbol":"PARTI-USDT","clientOid":"c2","status":"fail","failMsg":"balance insufficient"}
	]}`)
	var rows, err = decodeBatchRows(raw)
	if err != nil {
		t.Fatalf("decodeBatchRows(nested): %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].orderID() != "abc123" || rows[0].Status != "success" {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].FailMsg != "balance insufficient" || rows[1].Status != "fail" {
		t.Errorf("row1 = %+v", rows[1])
	}
}

// TestDecodeBatchRows_BareArray keeps tolerance for the legacy/sandbox shape
// where the payload is a bare [ ... ] array.
func TestDecodeBatchRows_BareArray(t *testing.T) {
	var raw = json.RawMessage(`[{"id":"legacy1","clientOid":"c1"}]`)
	var rows, err = decodeBatchRows(raw)
	if err != nil {
		t.Fatalf("decodeBatchRows(bare): %v", err)
	}
	if len(rows) != 1 || rows[0].orderID() != "legacy1" {
		t.Fatalf("rows = %+v", rows)
	}
}

// TestDecodeBatchRows_Empty tolerates null / empty payloads.
func TestDecodeBatchRows_Empty(t *testing.T) {
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(``), json.RawMessage(`  `)} {
		var rows, err = decodeBatchRows(raw)
		if err != nil {
			t.Fatalf("decodeBatchRows(%q): %v", string(raw), err)
		}
		if len(rows) != 0 {
			t.Fatalf("rows = %d, want 0 for %q", len(rows), string(raw))
		}
	}
}
