/*
FILE: futures/client.go

DESCRIPTION:
Root sub-client for the KuCoin Futures profile. Holds a reference to the
parent kucoin.Client (REST, signer, logger, config) and exposes four domain
sub-clients — MarketData, Trading, Account, Stream.

Unlike Bitget MIX, KuCoin Futures does NOT take a productType/marginCoin
trio on every request: the settle currency is implied by the symbol, and
leverage is supplied per order. The only account-wide defaults the SDK
pins are an optional default margin mode and default leverage applied when
a place-order request leaves them unset.

CONTRACT:
  - Client is safe for concurrent use; sub-clients are read-only after
    construction.
  - All REST calls go through parent.REST() — shared connection pool.
*/

package futures

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// ClientSettings — optional account-wide defaults for the Futures profile.
// Empty fields fall back to SDK defaults:
//   - DefaultMarginMode "" → ISOLATED.
//   - DefaultLeverage   "" → unset; the place-order request must carry
//     a leverage, otherwise the call returns ErrorKindInvalidRequest.
type ClientSettings struct {
	DefaultMarginMode futurestypes.MarginMode
	DefaultLeverage   string
}

// Client — KuCoin Futures profile client.
type Client struct {
	parent *kucoin.Client

	defaultMarginMode futurestypes.MarginMode
	defaultLeverage   string

	trading    *TradingClient
	account    *AccountClient
	marketData *MarketDataClient
	stream     *StreamClient
}

// NewClient creates a Futures profile client with SDK defaults. Returns nil
// if parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	return NewClientWithSettings(parent, ClientSettings{})
}

// NewClientWithSettings creates a Futures profile client with explicit
// defaults. Empty fields fall back to the SDK defaults documented on
// ClientSettings. Returns nil if parent is nil.
func NewClientWithSettings(parent *kucoin.Client, s ClientSettings) *Client {
	if parent == nil {
		return nil
	}
	if s.DefaultMarginMode == "" {
		s.DefaultMarginMode = futurestypes.MarginIsolated
	}
	var c *Client = &Client{
		parent:            parent,
		defaultMarginMode: s.DefaultMarginMode,
		defaultLeverage:   s.DefaultLeverage,
	}
	c.trading = newTradingClient(c)
	c.account = newAccountClient(c)
	c.marketData = newMarketDataClient(c)
	c.stream = newStreamClient(c)
	return c
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

// DefaultMarginMode returns the resolved default margin mode.
func (c *Client) DefaultMarginMode() futurestypes.MarginMode { return c.defaultMarginMode }

// DefaultLeverage returns the resolved default leverage ("" when unset).
func (c *Client) DefaultLeverage() string { return c.defaultLeverage }

// MarketData returns the public market-data sub-client.
func (c *Client) MarketData() *MarketDataClient { return c.marketData }

// Trading returns the signed trading sub-client.
func (c *Client) Trading() *TradingClient { return c.trading }

// Account returns the signed account / position sub-client.
func (c *Client) Account() *AccountClient { return c.account }

// Stream returns the WebSocket subscription sub-client.
func (c *Client) Stream() *StreamClient { return c.stream }

// Internal shortcuts shared by sub-clients.
func (c *Client) logger() kucoin.Logger { return c.parent.Logger() }
func (c *Client) rest() *rest.Client    { return c.parent.REST() }
func (c *Client) config() kucoin.Config { return c.parent.Config() }
func (c *Client) signerEnabled() bool   { return c.parent.Signer().Enabled() }

// init registers the factory in the root package so kucoin.Client.Futures()
// lazily returns *futures.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/futures" triggers this init.
func init() {
	kucoin.RegisterFuturesFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
