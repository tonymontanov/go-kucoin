/*
FILE: futures/stream_private_test.go

DESCRIPTION:
White-box tests for private position-push decoding. The crux: KuCoin's
/contract/position channel emits frames without `currentQty` on non-position
subjects (markPriceChange / marginChange / ...). Those MUST NOT be read as a
flat (qty=0) position — otherwise a subscriber zeroes inventory on every mark
price tick. positionPushWire.CurrentQty is a pointer so callers can tell
"qty not reported" (nil) from "genuinely flat" (present 0).
*/

package futures

import (
	"testing"

	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
)

func TestPositionPush_MarkPriceChange_NoCurrentQty(t *testing.T) {
	// markPriceChange frame: NO currentQty field.
	var raw = []byte(`{
		"symbol":"PARTIUSDTM","markPrice":0.04661,"markValue":0.123,
		"unrealisedPnl":-0.01,"realLeverage":10.0,"maintMarginReq":0.005,
		"settleCurrency":"USDT","changeReason":"markPriceChange"
	}`)
	var w positionPushWire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if w.CurrentQty != nil {
		t.Fatalf("CurrentQty = %v, want nil (frame carries no currentQty)", w.CurrentQty)
	}
	// Converter must flag qty as UNKNOWN (not a flat position).
	var p = w.toPositionInfo("PARTIUSDTM")
	if p.CurrentQtyKnown {
		t.Error("CurrentQtyKnown = true, want false for mark-price frame")
	}
	if p.IsOpen {
		t.Error("IsOpen = true, want false when qty unknown")
	}
}

func TestPositionPush_PositionChange_CarriesQty(t *testing.T) {
	// positionChange frame: currentQty present (here negative → short).
	var raw = []byte(`{
		"symbol":"PARTIUSDTM","currentQty":-428,"avgEntryPrice":0.04662,
		"crossMode":true,"settleCurrency":"USDT","changeReason":"positionChange"
	}`)
	var w positionPushWire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if w.CurrentQty == nil {
		t.Fatal("CurrentQty = nil, want -428")
	}
	if w.CurrentQty.String() != "-428" {
		t.Fatalf("CurrentQty = %s, want -428", w.CurrentQty.String())
	}
	var p = w.toPositionInfo("PARTIUSDTM")
	if !p.CurrentQtyKnown {
		t.Error("CurrentQtyKnown = false, want true (currentQty present)")
	}
	if !p.IsOpen {
		t.Error("IsOpen = false, want true for qty=-428")
	}
	if !p.CrossMode {
		t.Error("CrossMode = false, want true")
	}
}

func TestPositionPush_FlatClose_PresentZero(t *testing.T) {
	// Genuine close: currentQty present with value 0 → IsOpen=false, emitted.
	var raw = []byte(`{
		"symbol":"PARTIUSDTM","currentQty":0,"avgEntryPrice":0,
		"changeReason":"positionChange"
	}`)
	var w positionPushWire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if w.CurrentQty == nil {
		t.Fatal("CurrentQty = nil, want present 0 (genuine flat)")
	}
	var p = w.toPositionInfo("PARTIUSDTM")
	if !p.CurrentQtyKnown {
		t.Error("CurrentQtyKnown = false, want true (currentQty present as 0)")
	}
	if p.IsOpen {
		t.Error("IsOpen = true, want false for qty=0")
	}
}

// ---------------------------------------------------------------------
// /contractMarket/tradeOrders lifecycle frames — fill fields (v2.8).
// Frame shapes follow the documented push (kucoin-universal-sdk OrderEvent
// field list): matchSize/matchPrice/tradeId/liquidity are set on "match"
// frames; other frames ship them as "". Sizes are in contracts.
// ---------------------------------------------------------------------

func TestOrderPush_MatchCarriesExecution(t *testing.T) {
	var raw = []byte(`{
		"orderId":"5cdfc138b21023a909e5ad55","symbol":"XBTUSDTM","type":"match",
		"status":"open","matchSize":"20","matchPrice":"3600","orderType":"limit",
		"side":"buy","price":"3600","size":"20000","remainSize":"19980",
		"filledSize":"20","canceledSize":"0","tradeId":"5ce24c16b210233c36eexxxx",
		"clientOid":"5ce24c16b210233c36ee321d","orderTime":1545914149935808589,
		"liquidity":"maker","ts":1545914149935808589
	}`)
	var o futurestypes.OrderInfo
	var err error
	o, err = DecodeOrderPush(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if o.EventType != "match" || o.TradeID != "5ce24c16b210233c36eexxxx" {
		t.Fatalf("EventType/TradeID = %q/%q", o.EventType, o.TradeID)
	}
	if !o.MatchSize.Equal(decimal.NewFromInt(20)) || !o.MatchPrice.Equal(decimal.NewFromInt(3600)) {
		t.Fatalf("MatchSize/MatchPrice = %s/%s", o.MatchSize, o.MatchPrice)
	}
	if o.Liquidity != futurestypes.LiquidityMaker {
		t.Fatalf("Liquidity = %q, want maker", o.Liquidity)
	}
	if !o.FilledSize.Equal(decimal.NewFromInt(20)) || !o.Size.Equal(decimal.NewFromInt(20000)) {
		t.Fatalf("FilledSize/Size = %s/%s", o.FilledSize, o.Size)
	}
	if o.TsNs != 1545914149935808589 || o.UpdatedAtMs != 1545914149935 {
		t.Fatalf("TsNs/UpdatedAtMs = %d/%d", o.TsNs, o.UpdatedAtMs)
	}
	if !o.IsActive || o.Status != "open" {
		t.Fatalf("IsActive/Status = %v/%q", o.IsActive, o.Status)
	}
}

// Non-match frames ship matchSize/matchPrice as "" — the frame must still
// decode (flexDecimal) and leave the execution fields zero.
func TestOrderPush_EmptyMatchFieldsDecode(t *testing.T) {
	var raw = []byte(`{
		"orderId":"5cdfc138b21023a909e5ad55","symbol":"XBTUSDTM","type":"canceled",
		"status":"done","matchSize":"","matchPrice":"","orderType":"limit",
		"side":"buy","price":"3600","size":"20000","remainSize":"0",
		"filledSize":"20","canceledSize":"19980","tradeId":"",
		"clientOid":"5ce24c16b210233c36ee321d","orderTime":1545914149935808589,
		"liquidity":"","ts":1545914150935808589
	}`)
	var o futurestypes.OrderInfo
	var err error
	o, err = DecodeOrderPush(raw)
	if err != nil {
		t.Fatalf("decode with empty matchSize/matchPrice failed: %v", err)
	}
	if o.EventType != "canceled" || o.Status != "done" || o.IsActive {
		t.Fatalf("EventType/Status/IsActive = %q/%q/%v", o.EventType, o.Status, o.IsActive)
	}
	if !o.MatchSize.IsZero() || !o.MatchPrice.IsZero() || o.TradeID != "" || o.Liquidity != "" {
		t.Fatalf("execution fields must be zero on canceled: %+v", o)
	}
	if !o.FilledSize.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("FilledSize = %s, want 20", o.FilledSize)
	}
	var f flexDecimal
	if err = f.UnmarshalJSON([]byte(`"1.5"`)); err != nil || !f.Equal(decimal.RequireFromString("1.5")) {
		t.Fatalf("quoted: %v %s", err, f.Decimal)
	}
	if err = f.UnmarshalJSON([]byte(`2`)); err != nil || !f.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("bare: %v %s", err, f.Decimal)
	}
}
