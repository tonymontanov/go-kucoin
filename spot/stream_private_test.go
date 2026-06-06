/*
FILE: spot/stream_private_test.go

DESCRIPTION:
White-box regression tests for the spot private push decoders. The headline
case: KuCoin ships /account/balance `time` as a QUOTED string, which a plain
int64 field rejected — failing the whole decode so the balance push was
silently dropped (the "inventory websocket not working" symptom). flexInt64
must accept both quoted-string and bare-number timestamps.
*/

package spot

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
)

// TestBalancePush_QuotedTimeDecodes locks the fix: a balance push with a
// quoted "time" must decode and surface the currency + total so the desk's
// per-currency router can emit the position update.
func TestBalancePush_QuotedTimeDecodes(t *testing.T) {
	// Real KuCoin shape (see Account Balance Change docs): time is QUOTED.
	var raw = []byte(`{
		"accountId":"548674591753",
		"currency":"PARTI",
		"total":"58.84",
		"available":"40.53",
		"availableChange":"18.31",
		"hold":"18.31",
		"holdChange":"18.31",
		"relationContext":{"symbol":"PARTI-USDT","orderId":"x","tradeId":"y"},
		"relationEvent":"trade.hold",
		"relationEventId":"354689988084000",
		"time":"1730269283892"
	}`)
	var w balancePushWire
	if err := codec.Unmarshal(raw, &w); err != nil {
		t.Fatalf("balance push decode failed (quoted time regression): %v", err)
	}
	if w.Currency != "PARTI" {
		t.Errorf("currency = %q, want PARTI", w.Currency)
	}
	if !w.Total.Equal(decimal.RequireFromString("58.84")) {
		t.Errorf("total = %s, want 58.84", w.Total)
	}
	if w.Time.int64() != 1730269283892 {
		t.Errorf("time = %d, want 1730269283892", w.Time.int64())
	}
	var b = w.toBalance()
	if b.MarginCoin != "PARTI" || !b.TotalEquity.Equal(decimal.RequireFromString("58.84")) {
		t.Errorf("toBalance = %+v", b)
	}
}

// TestOrderPush_BareNumberTimeDecodes keeps the order push (bare-number
// orderTime/ts) working through the same flexInt64 type.
func TestOrderPush_BareNumberTimeDecodes(t *testing.T) {
	var raw = []byte(`{
		"orderId":"abc","symbol":"PARTI-USDT","orderType":"limit","type":"match",
		"status":"open","side":"buy","price":"0.0508","size":"100","filledSize":"0",
		"clientOid":"c1","orderTime":1593487481683297666,"ts":1593487481683297666
	}`)
	var w orderPushWire
	if err := codec.Unmarshal(raw, &w); err != nil {
		t.Fatalf("order push decode failed: %v", err)
	}
	if w.OrderTime.int64() != 1593487481683297666 {
		t.Errorf("orderTime = %d", w.OrderTime.int64())
	}
	var o = w.toOrderInfo()
	if o.OrderID != "abc" || !o.IsActive {
		t.Errorf("toOrderInfo = %+v", o)
	}
}

// TestFlexInt64_Forms exercises the tolerant integer across both shapes plus
// null/empty.
func TestFlexInt64_Forms(t *testing.T) {
	var cases = []struct {
		in   string
		want int64
	}{
		{`"123"`, 123},
		{`456`, 456},
		{`null`, 0},
		{`""`, 0},
	}
	for _, c := range cases {
		var f flexInt64
		if err := f.UnmarshalJSON([]byte(c.in)); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", c.in, err)
		}
		if f.int64() != c.want {
			t.Errorf("UnmarshalJSON(%s) = %d, want %d", c.in, f.int64(), c.want)
		}
	}
}
