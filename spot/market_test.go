/*
FILE: spot/market_test.go

DESCRIPTION:
White-box tests for the spot market-data wire decoders: the spot candle
column order ([time, open, CLOSE, high, low, …]), the level2 push change
flattening + sequence sort, and the kline-granularity mapping.
*/

package spot

import (
	"testing"

	"github.com/shopspring/decimal"

	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

func TestKlinesFromRows_SpotColumnOrder(t *testing.T) {
	// Spot row: [time(sec), open, close, high, low, volume, turnover].
	var raw = `[["1700000000","100","105","110","90","1234","100"]]`
	var rows = mustDecimalRows(t, raw)
	var candles = klinesFromRows(rows)
	if len(candles) != 1 {
		t.Fatalf("candles = %d", len(candles))
	}
	var c = candles[0]
	if c.OpenTimeMs != 1700000000000 {
		t.Errorf("openTimeMs = %d", c.OpenTimeMs)
	}
	if c.Open.String() != "100" || c.Close.String() != "105" || c.High.String() != "110" || c.Low.String() != "90" {
		t.Errorf("OHLC = %s/%s/%s/%s (want 100/105/110/90)", c.Open, c.High, c.Low, c.Close)
	}
}

func TestLevel2Push_ChangesSortedBySequence(t *testing.T) {
	// Interleaved ask/bid changes with out-of-array-order sequences must come
	// back sorted ascending by sequence so the contiguous engine accepts them.
	var data = []byte(`{
		"sequenceStart":100,"sequenceEnd":103,"symbol":"BTC-USDT","time":1700000000000,
		"changes":{"asks":[["50001","2","102"],["50002","0","100"]],"bids":[["50000","5","101"],["49999","3","103"]]}
	}`)
	var w level2PushWire
	if codecUnmarshal(data, &w) != nil {
		t.Fatal("decode level2 push")
	}
	var ch = w.changes()
	if len(ch) != 4 {
		t.Fatalf("changes = %d, want 4", len(ch))
	}
	var prev int64
	var i int
	for i = 0; i < len(ch); i++ {
		if ch[i].seq < prev {
			t.Fatalf("changes not sorted: seq %d after %d", ch[i].seq, prev)
		}
		prev = ch[i].seq
	}
	// Seq 100 is the ask at 50002 with size 0 (a removal); side must be sell.
	if ch[0].seq != 100 || ch[0].side != "sell" {
		t.Errorf("first change = %+v, want seq 100 sell", ch[0])
	}
	if w.timeMs() != 1700000000000 {
		t.Errorf("timeMs = %d", w.timeMs())
	}
}

func TestSpotGranularityMapping(t *testing.T) {
	var cases = map[spottypes.Timeframe]string{
		spottypes.Timeframe1m:  "1min",
		spottypes.Timeframe1h:  "1hour",
		spottypes.Timeframe1d:  "1day",
		spottypes.Timeframe1w:  "1week",
	}
	var tf spottypes.Timeframe
	var want string
	for tf, want = range cases {
		if got := spotGranularity[tf]; got != want {
			t.Errorf("granularity[%s] = %q, want %q", tf, got, want)
		}
	}
}

// mustDecimalRows decodes a JSON array-of-arrays of decimal strings/numbers.
func mustDecimalRows(t *testing.T, raw string) [][]decimal.Decimal {
	t.Helper()
	var rows [][]decimal.Decimal
	if err := codecUnmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode rows: %v", err)
	}
	return rows
}
