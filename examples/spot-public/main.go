/*
FILE: examples/spot-public/main.go

DESCRIPTION:
Public (no-auth) example for the KuCoin Spot profile: fetch a symbol spec and
a level1 ticker, then stream the managed level-2 order book and the public
trade tape for a few seconds.

RUN:

	go run ./examples/spot-public
*/

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/spot"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

func main() {
	const symbol = "BTC-USDT"

	var c, err = kucoin.NewClient(kucoin.DefaultConfig())
	if err != nil {
		log.Fatalf("new client: %v", err)
	}
	defer func() { _ = c.Close() }()

	var sc = c.Spot().(*spot.Client)

	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// REST: symbol spec + level1 ticker.
	var info, sErr = sc.MarketData().GetSymbol(ctx, symbol)
	if sErr != nil {
		log.Fatalf("symbol: %v", sErr)
	}
	fmt.Printf("%s baseIncr=%s priceIncr=%s minFunds=%s\n",
		info.Symbol, info.BaseIncrement, info.PriceIncrement, info.MinFunds)

	var tk, tErr = sc.MarketData().GetTicker(ctx, symbol)
	if tErr == nil {
		fmt.Printf("ticker last=%s bid=%s ask=%s\n", tk.LastPrice, tk.BestBidPrice, tk.BestAskPrice)
	}

	// WS: managed order book + trades.
	var bookErr = sc.Stream().WatchOrderBook(ctx, symbol, func(ob *roottypes.OrderBookSnapshot) {
		if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
			return
		}
		fmt.Printf("book seq=%d bid=%s ask=%s\n", ob.Sequence, ob.Bids[0].Price, ob.Asks[0].Price)
	})
	if bookErr != nil {
		log.Fatalf("watch order book: %v", bookErr)
	}

	var tradeErr = sc.Stream().WatchTrades(ctx, symbol, func(tr *roottypes.TradeUpdate) {
		fmt.Printf("trade %s %s x %s\n", tr.Side, tr.Price, tr.Size)
	})
	if tradeErr != nil {
		log.Fatalf("watch trades: %v", tradeErr)
	}

	<-ctx.Done()
	_ = sc.Stream().Close()
}
