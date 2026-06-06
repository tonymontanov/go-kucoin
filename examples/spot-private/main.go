/*
FILE: examples/spot-private/main.go

DESCRIPTION:
Signed example for the KuCoin Spot profile: read the trade-account balance,
place a far-from-market post-only limit order, query it, then cancel it.
Credentials are read from the environment:

	KUCOIN_API_KEY, KUCOIN_API_SECRET, KUCOIN_API_PASSPHRASE

RUN:

	KUCOIN_API_KEY=... KUCOIN_API_SECRET=... KUCOIN_API_PASSPHRASE=... \
	    go run ./examples/spot-private
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
	"github.com/tonymontanov/go-kucoin/v2/spot"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

func main() {
	const symbol = "BTC-USDT"

	var cfg = kucoin.DefaultConfig()
	cfg.APIKey = os.Getenv("KUCOIN_API_KEY")
	cfg.SecretKey = os.Getenv("KUCOIN_API_SECRET")
	cfg.Passphrase = os.Getenv("KUCOIN_API_PASSPHRASE")
	if cfg.APIKey == "" || cfg.SecretKey == "" || cfg.Passphrase == "" {
		log.Fatal("set KUCOIN_API_KEY / KUCOIN_API_SECRET / KUCOIN_API_PASSPHRASE")
	}

	var c, err = kucoin.NewClient(cfg)
	if err != nil {
		log.Fatalf("new client: %v", err)
	}
	defer func() { _ = c.Close() }()

	var sc = c.Spot().(*spot.Client)
	var ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Trade-account balance (USDT).
	var bal, balErr = sc.Account().GetBalance(ctx, "USDT")
	if balErr != nil {
		log.Fatalf("balance: %v", balErr)
	}
	fmt.Printf("USDT available=%s holds=%s\n", bal.AvailableBalance, bal.LockedBalance)

	// Place a far-out post-only limit buy (0.001 BTC @ 10000).
	var ack, placeErr = sc.Trading().PlaceOrder(ctx, spottypes.CreateOrderRequest{
		Symbol:   symbol,
		Side:     spottypes.SideBuy,
		Type:     spottypes.OrderLimit,
		Size:     decimal.RequireFromString("0.001"),
		Price:    decimal.RequireFromString("10000"),
		PostOnly: true,
	})
	if placeErr != nil {
		log.Fatalf("place: %v", placeErr)
	}
	fmt.Printf("placed order=%s clientOid=%s\n", ack.OrderID, ack.ClientOrderID)

	// Query then cancel it.
	var ord, getErr = sc.Trading().GetOrder(ctx, ack.OrderID)
	if getErr == nil {
		fmt.Printf("order active=%v dealSize=%s\n", ord.IsActive, ord.DealSize)
	}

	var cancelled, cancelErr = sc.Trading().CancelOrder(ctx, ack.OrderID)
	if cancelErr != nil {
		log.Fatalf("cancel: %v", cancelErr)
	}
	fmt.Printf("cancelled=%v\n", cancelled)
}
