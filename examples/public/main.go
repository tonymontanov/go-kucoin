/*
FILE: examples/public/main.go

DESCRIPTION:
Public (no-auth) example for the KuCoin Futures profile: fetch a contract
spec and a REST order-book snapshot, then stream the managed level-2 order
book and the public trade tape for a few seconds.

RUN:

	go run ./examples/public
*/

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/futures"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

func main() {
	const symbol = "XBTUSDTM"

	var c, err = kucoin.NewClient(kucoin.DefaultConfig())
	if err != nil {
		log.Fatalf("new client: %v", err)
	}
	defer func() { _ = c.Close() }()

	var fc = c.Futures().(*futures.Client)

	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// REST: contract spec + a snapshot to show sizing context.
	var info, cErr = fc.MarketData().GetContract(ctx, symbol)
	if cErr != nil {
		log.Fatalf("contract: %v", cErr)
	}
	fmt.Printf("%s multiplier=%s tickSize=%s lotSize=%s\n",
		info.Symbol, info.Multiplier, info.TickSize, info.LotSize)

	// WS: managed order book + trades.
	var bookErr = fc.Stream().WatchOrderBook(ctx, symbol, func(ob *roottypes.OrderBookSnapshot) {
		if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
			return
		}
		fmt.Printf("book seq=%d bid=%s ask=%s\n", ob.Sequence, ob.Bids[0].Price, ob.Asks[0].Price)
	})
	if bookErr != nil {
		log.Fatalf("watch order book: %v", bookErr)
	}

	var tradeErr = fc.Stream().WatchTrades(ctx, symbol, func(tr *roottypes.TradeUpdate) {
		fmt.Printf("trade %s %s x %s\n", tr.Side, tr.Price, tr.Size)
	})
	if tradeErr != nil {
		log.Fatalf("watch trades: %v", tradeErr)
	}

	<-ctx.Done()
	_ = fc.Stream().Close()
}
