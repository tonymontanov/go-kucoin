/*
FILE: internal/kccommon/orderbook/engine_test.go

DESCRIPTION:
Unit tests for the sequence-based KuCoin Futures level2 engine: snapshot
seeding, contiguous apply, stale drop, gap detection, level removal, and
the change-string parser.
*/

package orderbook

import (
	"testing"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal {
	var v, err = decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

func seed(t *testing.T) *Engine {
	t.Helper()
	var e = NewEngine("XBTUSDTM", 200)
	// asks ascending, bids descending.
	e.ApplySnapshot(
		[]Level{{Price: d("100"), Size: d("1")}, {Price: d("101"), Size: d("2")}},
		[]Level{{Price: d("99"), Size: d("3")}, {Price: d("98"), Size: d("4")}},
		10, 1700000000000,
	)
	return e
}

func TestEngine_DirtyBeforeSnapshot(t *testing.T) {
	var e = NewEngine("XBTUSDTM", 200)
	if !e.IsDirty() {
		t.Fatal("new engine must be dirty")
	}
	if err := e.ApplyChange(SideBuy, d("99.5"), d("1"), 11, 0); err != ErrDirty {
		t.Fatalf("err = %v, want ErrDirty", err)
	}
}

func TestEngine_ApplyContiguous(t *testing.T) {
	var e = seed(t)
	// seq 11 = snapshot.seq+1 → insert a new bid at 99.5 (best bid).
	if err := e.ApplyChange(SideBuy, d("99.5"), d("5"), 11, 1700000000001); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var snap = e.Snapshot()
	if len(snap.Bids) != 3 {
		t.Fatalf("bids len = %d, want 3", len(snap.Bids))
	}
	if !snap.Bids[0].Price.Equal(d("99.5")) {
		t.Fatalf("best bid = %s, want 99.5", snap.Bids[0].Price)
	}
	if snap.Sequence != 11 {
		t.Fatalf("sequence = %d, want 11", snap.Sequence)
	}
}

func TestEngine_StaleDropped(t *testing.T) {
	var e = seed(t)
	// seq 10 <= lastSequence(10) → silently dropped, no error, no change.
	if err := e.ApplyChange(SideBuy, d("99"), d("0"), 10, 0); err != nil {
		t.Fatalf("stale apply err = %v, want nil", err)
	}
	if e.LastSequence() != 10 {
		t.Fatalf("sequence advanced on stale change: %d", e.LastSequence())
	}
	if len(e.Snapshot().Bids) != 2 {
		t.Fatal("stale change must not mutate the book")
	}
}

func TestEngine_GapTriggersResync(t *testing.T) {
	var e = seed(t)
	// seq 13 skips 11,12 → gap.
	if err := e.ApplyChange(SideSell, d("102"), d("1"), 13, 0); err != ErrGap {
		t.Fatalf("err = %v, want ErrGap", err)
	}
	if !e.IsDirty() {
		t.Fatal("engine must be dirty after a gap")
	}
	// Subsequent change is dropped as dirty until a fresh snapshot.
	if err := e.ApplyChange(SideSell, d("102"), d("1"), 14, 0); err != ErrDirty {
		t.Fatalf("err = %v, want ErrDirty", err)
	}
}

func TestEngine_RemoveLevel(t *testing.T) {
	var e = seed(t)
	// Remove the best ask (100) via size 0.
	if err := e.ApplyChange(SideSell, d("100"), d("0"), 11, 0); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var snap = e.Snapshot()
	if len(snap.Asks) != 1 || !snap.Asks[0].Price.Equal(d("101")) {
		t.Fatalf("asks = %+v, want only [101]", snap.Asks)
	}
}

func TestEngine_ReplaceLevelSize(t *testing.T) {
	var e = seed(t)
	if err := e.ApplyChange(SideSell, d("100"), d("9"), 11, 0); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var snap = e.Snapshot()
	if !snap.Asks[0].Size.Equal(d("9")) {
		t.Fatalf("best ask size = %s, want 9", snap.Asks[0].Size)
	}
}

func TestEngine_ResnapshotClearsDirty(t *testing.T) {
	var e = seed(t)
	_ = e.ApplyChange(SideSell, d("102"), d("1"), 13, 0) // gap → dirty
	if !e.IsDirty() {
		t.Fatal("precondition: dirty after gap")
	}
	e.ApplySnapshot(
		[]Level{{Price: d("100"), Size: d("1")}},
		[]Level{{Price: d("99"), Size: d("1")}},
		20, 0,
	)
	if e.IsDirty() {
		t.Fatal("engine must be clean after re-snapshot")
	}
	if e.LastSequence() != 20 {
		t.Fatalf("sequence = %d, want 20", e.LastSequence())
	}
}

func TestParseChange(t *testing.T) {
	var side string
	var price, size decimal.Decimal
	var err error
	side, price, size, err = ParseChange("5000.0,sell,83")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if side != SideSell || !price.Equal(d("5000.0")) || !size.Equal(d("83")) {
		t.Fatalf("got side=%s price=%s size=%s", side, price, size)
	}
}

func TestParseChange_Malformed(t *testing.T) {
	if _, _, _, err := ParseChange("5000.0,sell"); err != ErrBadChange {
		t.Fatalf("err = %v, want ErrBadChange", err)
	}
}

func TestLevelsFromPairs(t *testing.T) {
	var levels, err = LevelsFromPairs([][]decimal.Decimal{
		{d("100"), d("1")},
		{d("101"), d("2")},
	})
	if err != nil {
		t.Fatalf("LevelsFromPairs: %v", err)
	}
	if len(levels) != 2 || !levels[1].Price.Equal(d("101")) {
		t.Fatalf("got %+v", levels)
	}
}
