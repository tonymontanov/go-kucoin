/*
FILE: futures/market_test.go

DESCRIPTION:
White-box tests for the market-data wire decoding: JSON tag correctness,
decimal normalisation of mixed number/string fields, nanosecond→millisecond
timestamp conversion, and the kline / order-book row mapping.
*/

package futures

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestContractWire_Decode(t *testing.T) {
	var raw = []byte(`{
		"symbol":"XBTUSDTM","rootSymbol":"USDT","type":"FFWCSX",
		"baseCurrency":"XBT","quoteCurrency":"USDT","settleCurrency":"USDT",
		"status":"Open","isInverse":false,
		"multiplier":0.001,"lotSize":1,"tickSize":0.1,"indexPriceTickSize":0.01,
		"maxOrderQty":1000000,"maxPrice":1000000.0,
		"makerFeeRate":0.0002,"takerFeeRate":0.0006,
		"markPrice":50000.1,"indexPrice":50000.0,
		"openInterest":"4955514","volumeOf24h":6788.072,"turnoverOf24h":5.98e8
	}`)
	var w contractWire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var info = w.toSymbolInfo()
	if info.Symbol != "XBTUSDTM" || info.SettleCurrency != "USDT" {
		t.Fatalf("symbol/settle = %q/%q", info.Symbol, info.SettleCurrency)
	}
	if info.Multiplier.String() != "0.001" {
		t.Errorf("multiplier = %s, want 0.001", info.Multiplier)
	}
	if info.OpenInterest.String() != "4955514" {
		t.Errorf("openInterest = %s, want 4955514 (string field)", info.OpenInterest)
	}
	if info.TickSize.String() != "0.1" {
		t.Errorf("tickSize = %s, want 0.1", info.TickSize)
	}
}

func TestTickerWire_NsToMs(t *testing.T) {
	var raw = []byte(`{"sequence":1,"symbol":"XBTUSDTM","side":"sell","size":3,
		"price":"50000.5","bestBidPrice":"50000.0","bestBidSize":10,
		"bestAskPrice":"50001.0","bestAskSize":7,"tradeId":"t1","ts":1700000000000000000}`)
	var w tickerWire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var tk = w.toTicker()
	if tk.TsMs != 1700000000000 {
		t.Errorf("TsMs = %d, want 1700000000000 (ns→ms)", tk.TsMs)
	}
	if tk.LastPrice.String() != "50000.5" || tk.BestAskPrice.String() != "50001" {
		t.Errorf("prices = %s / %s", tk.LastPrice, tk.BestAskPrice)
	}
	if tk.Side != "sell" {
		t.Errorf("side = %q", tk.Side)
	}
}

func TestLevel2Wire_ToSnapshot(t *testing.T) {
	var raw = []byte(`{"sequence":100,"symbol":"XBTUSDTM",
		"bids":[[50000.0,5],[49999.0,3]],"asks":[[50001.0,2],[50002.0,8]],"ts":1700000000000}`)
	var w level2Wire
	if err := codecUnmarshal(raw, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var snap = w.toSnapshot("XBTUSDTM")
	if snap.Sequence != 100 {
		t.Errorf("sequence = %d, want 100", snap.Sequence)
	}
	if len(snap.Bids) != 2 || len(snap.Asks) != 2 {
		t.Fatalf("levels = %d bids / %d asks", len(snap.Bids), len(snap.Asks))
	}
	if snap.Bids[0].Price.String() != "50000" || snap.Asks[0].Size.String() != "2" {
		t.Errorf("bid0=%s ask0size=%s", snap.Bids[0].Price, snap.Asks[0].Size)
	}
}

func TestKlinesFromRows(t *testing.T) {
	var raw = []byte(`[[1700000000000,100,110,90,105,1234],[1700000060000,105,120,104,118,999]]`)
	var rows [][]decimal.Decimal
	if err := codecUnmarshal(raw, &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var candles = klinesFromRows(rows)
	if len(candles) != 2 {
		t.Fatalf("candles = %d, want 2", len(candles))
	}
	if candles[0].OpenTimeMs != 1700000000000 {
		t.Errorf("openTime = %d", candles[0].OpenTimeMs)
	}
	if candles[1].Close.String() != "118" {
		t.Errorf("close[1] = %s, want 118", candles[1].Close)
	}
}
