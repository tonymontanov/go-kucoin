/*
FILE: margin/stream_private_test.go

DESCRIPTION:
Unit tests for the private margin WS push decoding. Covers flexInt64 (KuCoin
ships order timestamps as bare numbers OR quoted strings across the
spot/margin private channel) and the order push → OrderInfo converter,
including the margin tradeType.
*/

package margin

import (
	"testing"

	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
)

func TestOrderPushWire_FlexInt64BothShapes(t *testing.T) {
	var cases = []struct {
		name string
		json string
	}{
		{"bareNumbers", `{"orderId":"o1","symbol":"BTC-USDT","orderType":"limit","type":"match","status":"open","side":"buy","tradeType":"MARGIN_TRADE","price":"50000","size":"0.1","filledSize":"0.02","funds":"0","clientOid":"c1","orderTime":1700000000000000000,"ts":1700000000500000000}`},
		{"quotedStrings", `{"orderId":"o1","symbol":"BTC-USDT","orderType":"limit","type":"match","status":"open","side":"buy","tradeType":"MARGIN_TRADE","price":"50000","size":"0.1","filledSize":"0.02","funds":"0","clientOid":"c1","orderTime":"1700000000000000000","ts":"1700000000500000000"}`},
	}
	var tc = cases[0]
	for _, tc = range cases {
		var w orderPushWire
		if err := codecUnmarshal([]byte(tc.json), &w); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		var o margintypes.OrderInfo = w.toOrderInfo()
		if o.OrderID != "o1" || o.Symbol != "BTC-USDT" {
			t.Fatalf("%s: order = %+v", tc.name, o)
		}
		if o.TradeType != margintypes.TradeCross {
			t.Errorf("%s: tradeType = %s", tc.name, o.TradeType)
		}
		if o.DealSize.String() != "0.02" {
			t.Errorf("%s: dealSize = %s", tc.name, o.DealSize)
		}
		if !o.IsActive { // status "open" → active
			t.Errorf("%s: expected active", tc.name)
		}
		// orderTime/ts are nanoseconds → converted to ms.
		if o.CreatedAtMs != 1700000000000 || o.UpdatedAtMs != 1700000000500 {
			t.Errorf("%s: ts ms = %d / %d", tc.name, o.CreatedAtMs, o.UpdatedAtMs)
		}
	}
}
