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

// ---------------------------------------------------------------------
// /spotMarket/tradeOrders lifecycle frames — fill fields (v2.8).
// Frame shapes follow the documented V1 push (kucoin-universal-sdk
// OrderV1Event field list): matchSize/matchPrice/tradeId/liquidity exist on
// "match" frames only; other frames omit them or ship "".
// ---------------------------------------------------------------------

// TestOrderPush_MatchCarriesExecution — a "match" frame surfaces the
// execution (size/price/tradeId/liquidity), the lifecycle type and the raw
// nanosecond ts alongside the cumulative DealSize.
func TestOrderPush_MatchCarriesExecution(t *testing.T) {
	var raw = []byte(`{
		"symbol":"KCS-USDT","orderType":"limit","side":"sell",
		"orderId":"5efab07953bdea00089965fa","liquidity":"maker","type":"match",
		"feeType":"makerFee","orderTime":1593487482038606180,"size":"0.1",
		"filledSize":"0.04","price":"0.938","matchPrice":"0.96738","matchSize":"0.04",
		"tradeId":"5efab07a4ee4c7000a19d690","clientOid":"1593487481000313",
		"remainSize":"0.06","status":"match","ts":1593487482038606180
	}`)
	var o, err = DecodeOrderPush(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if o.EventType != "match" || o.TradeID != "5efab07a4ee4c7000a19d690" {
		t.Fatalf("EventType/TradeID = %q/%q", o.EventType, o.TradeID)
	}
	if !o.MatchSize.Equal(decimal.RequireFromString("0.04")) || !o.MatchPrice.Equal(decimal.RequireFromString("0.96738")) {
		t.Fatalf("MatchSize/MatchPrice = %s/%s", o.MatchSize, o.MatchPrice)
	}
	if o.Liquidity != "maker" {
		t.Fatalf("Liquidity = %q, want maker", o.Liquidity)
	}
	if !o.DealSize.Equal(decimal.RequireFromString("0.04")) || !o.Size.Equal(decimal.RequireFromString("0.1")) {
		t.Fatalf("DealSize/Size = %s/%s", o.DealSize, o.Size)
	}
	if o.TsNs != 1593487482038606180 || o.UpdatedAtMs != 1593487482038 {
		t.Fatalf("TsNs/UpdatedAtMs = %d/%d", o.TsNs, o.UpdatedAtMs)
	}
	if !o.IsActive || o.Status != "match" {
		t.Fatalf("IsActive/Status = %v/%q", o.IsActive, o.Status)
	}
}

// TestOrderPush_FilledWithoutMatchFields — the terminal "filled" frame has no
// execution fields: MatchSize/TradeID stay zero, DealSize is the cumulative.
func TestOrderPush_FilledWithoutMatchFields(t *testing.T) {
	var raw = []byte(`{
		"symbol":"KCS-USDT","orderType":"limit","side":"sell",
		"orderId":"5efab07953bdea00089965fa","type":"filled",
		"orderTime":1593487482038606180,"size":"0.1","filledSize":"0.1",
		"price":"0.938","clientOid":"1593487481000313","remainSize":"0",
		"status":"done","ts":1593487482038606180
	}`)
	var o, err = DecodeOrderPush(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if o.EventType != "filled" || o.Status != "done" || o.IsActive {
		t.Fatalf("EventType/Status/IsActive = %q/%q/%v", o.EventType, o.Status, o.IsActive)
	}
	if !o.MatchSize.IsZero() || !o.MatchPrice.IsZero() || o.TradeID != "" || o.Liquidity != "" {
		t.Fatalf("execution fields must be zero on filled: %+v", o)
	}
	if !o.DealSize.Equal(decimal.RequireFromString("0.1")) {
		t.Fatalf("DealSize = %s, want 0.1", o.DealSize)
	}
}

// TestOrderPush_EmptyMatchFieldsDecode — KuCoin ships "" for matchSize /
// matchPrice on non-match frames; the frame must still decode (flexDecimal).
func TestOrderPush_EmptyMatchFieldsDecode(t *testing.T) {
	var raw = []byte(`{
		"symbol":"KCS-USDT","orderType":"limit","side":"buy",
		"orderId":"5efab07953bdea00089965d2","type":"open","matchSize":"","matchPrice":"",
		"liquidity":"","orderTime":1593487481683297666,"size":"0.1","filledSize":"0",
		"price":"0.937","clientOid":"1593487481000906","remainSize":"0.1",
		"status":"open","ts":1593487481683297666
	}`)
	var o, err = DecodeOrderPush(raw)
	if err != nil {
		t.Fatalf("decode with empty matchSize/matchPrice failed: %v", err)
	}
	if o.EventType != "open" || !o.MatchSize.IsZero() || !o.MatchPrice.IsZero() {
		t.Fatalf("toOrderInfo = %+v", o)
	}
	// Quoted / bare numeric forms still decode through flexDecimal.
	var f flexDecimal
	if err = f.UnmarshalJSON([]byte(`"1.5"`)); err != nil || !f.Equal(decimal.RequireFromString("1.5")) {
		t.Fatalf("quoted: %v %s", err, f.Decimal)
	}
	if err = f.UnmarshalJSON([]byte(`2`)); err != nil || !f.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("bare: %v %s", err, f.Decimal)
	}
	if err = f.UnmarshalJSON([]byte(`null`)); err != nil || !f.IsZero() {
		t.Fatalf("null: %v %s", err, f.Decimal)
	}
}
