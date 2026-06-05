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

import "testing"

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
