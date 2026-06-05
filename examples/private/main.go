/*
FILE: examples/private/main.go

DESCRIPTION:
Signed example for the KuCoin Futures profile: read the account balance and
open positions, place a far-from-market post-only limit order, query it, then
cancel it. Credentials are read from the environment:

	KUCOIN_API_KEY, KUCOIN_API_SECRET, KUCOIN_API_PASSPHRASE

Set KUCOIN_DEMO=1 to target the sandbox host.

RUN:

	KUCOIN_API_KEY=... KUCOIN_API_SECRET=... KUCOIN_API_PASSPHRASE=... \
	    go run ./examples/private
*/

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shopspring/decimal"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/futures"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
)

func main() {
	const symbol = "XBTUSDTM"

	var cfg = kucoin.DefaultConfig()
	cfg.APIKey = os.Getenv("KUCOIN_API_KEY")
	cfg.SecretKey = os.Getenv("KUCOIN_API_SECRET")
	cfg.Passphrase = os.Getenv("KUCOIN_API_PASSPHRASE")
	cfg.Demo = os.Getenv("KUCOIN_DEMO") == "1"
	if cfg.APIKey == "" || cfg.SecretKey == "" || cfg.Passphrase == "" {
		log.Fatal("set KUCOIN_API_KEY / KUCOIN_API_SECRET / KUCOIN_API_PASSPHRASE")
	}

	var c, err = kucoin.NewClient(cfg)
	if err != nil {
		log.Fatalf("new client: %v", err)
	}
	defer func() { _ = c.Close() }()

	var fc = c.Futures().(*futures.Client)
	var ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Balance + positions.
	var bal, balErr = fc.Account().GetBalance(ctx, "USDT")
	if balErr != nil {
		log.Fatalf("balance: %v", balErr)
	}
	fmt.Printf("equity=%s available=%s\n", bal.TotalEquity, bal.AvailableBalance)

	// Place a far-out post-only limit buy (1 contract, leverage 5).
	var ack, placeErr = fc.Trading().PlaceOrder(ctx, futurestypes.CreateOrderRequest{
		Symbol:   symbol,
		Side:     futurestypes.SideBuy,
		Type:     futurestypes.OrderLimit,
		Size:     1,
		Price:    decimal.RequireFromString("10000"),
		Leverage: "5",
		PostOnly: true,
	})
	if placeErr != nil {
		log.Fatalf("place: %v", placeErr)
	}
	fmt.Printf("placed order=%s clientOid=%s\n", ack.OrderID, ack.ClientOrderID)

	// Query then cancel it.
	var ord, getErr = fc.Trading().GetOrder(ctx, ack.OrderID)
	if getErr == nil {
		fmt.Printf("order status=%s filled=%s\n", ord.Status, ord.FilledSize)
	}

	var cancelled, cancelErr = fc.Trading().CancelOrder(ctx, ack.OrderID)
	if cancelErr != nil {
		log.Fatalf("cancel: %v", cancelErr)
	}
	fmt.Printf("cancelled=%v\n", cancelled)
}
