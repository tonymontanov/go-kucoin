/*
FILE: spot/client.go

DESCRIPTION:
Root sub-client for the KuCoin Spot profile. Holds a reference to the parent
kucoin.Client (signer, logger, config) and a DEDICATED REST client bound to
the spot host (api.kucoin.com), because the root REST client defaults to the
futures host. Exposes four domain sub-clients — MarketData, Trading,
Account, Stream.

CONTRACT:
  - Client is safe for concurrent use; sub-clients are read-only after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package spot

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

// ClientSettings — optional account-wide defaults for the Spot profile.
// Empty fields fall back to SDK defaults:
//   - DefaultTradeType "" → TRADE (spot account).
type ClientSettings struct {
	DefaultTradeType spottypes.TradeType
}

// Client — KuCoin Spot profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string

	defaultTradeType spottypes.TradeType

	trading    *TradingClient
	account    *AccountClient
	marketData *MarketDataClient
	stream     *StreamClient
}

// NewClient creates a Spot profile client with SDK defaults. Returns nil if
// parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	return NewClientWithSettings(parent, ClientSettings{})
}

// NewClientWithSettings creates a Spot profile client with explicit defaults.
// Empty fields fall back to the SDK defaults documented on ClientSettings.
// Returns nil if parent is nil.
func NewClientWithSettings(parent *kucoin.Client, s ClientSettings) *Client {
	if parent == nil {
		return nil
	}
	if s.DefaultTradeType == "" {
		s.DefaultTradeType = spottypes.TradeSpot
	}
	var base string = resolveSpotBaseURL(parent.Config())
	var c *Client = &Client{
		parent:           parent,
		restCli:          parent.NewSectionRESTClient(base),
		baseURL:          base,
		defaultTradeType: s.DefaultTradeType,
	}
	c.trading = newTradingClient(c)
	c.account = newAccountClient(c)
	c.marketData = newMarketDataClient(c)
	c.stream = newStreamClient(c)
	return c
}

// resolveSpotBaseURL picks the spot REST host. The root Config defaults its
// REST host to the FUTURES endpoint, so:
//   - a futures host (prod/sandbox) or empty resolves to the spot host
//     (sandbox when Demo is set);
//   - any OTHER explicit URL (e.g. a mock server or a deliberate override)
//     is honoured as-is so tests and advanced embedders keep control.
func resolveSpotBaseURL(cfg kucoin.Config) string {
	switch cfg.REST.BaseURL {
	case "", kucoin.DefaultFuturesRestBaseURL:
		if cfg.Demo {
			return kucoin.DefaultSpotSandboxRestBaseURL
		}
		return kucoin.DefaultSpotRestBaseURL
	case kucoin.DefaultFuturesSandboxRestBaseURL:
		return kucoin.DefaultSpotSandboxRestBaseURL
	default:
		return cfg.REST.BaseURL
	}
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

// DefaultTradeType returns the resolved default trade type.
func (c *Client) DefaultTradeType() spottypes.TradeType { return c.defaultTradeType }

// MarketData returns the public market-data sub-client.
func (c *Client) MarketData() *MarketDataClient { return c.marketData }

// Trading returns the signed trading sub-client.
func (c *Client) Trading() *TradingClient { return c.trading }

// Account returns the signed account sub-client.
func (c *Client) Account() *AccountClient { return c.account }

// Stream returns the WebSocket subscription sub-client.
func (c *Client) Stream() *StreamClient { return c.stream }

// Internal shortcuts shared by sub-clients.
func (c *Client) logger() kucoin.Logger { return c.parent.Logger() }
func (c *Client) rest() *rest.Client    { return c.restCli }
func (c *Client) config() kucoin.Config { return c.parent.Config() }
func (c *Client) signerEnabled() bool   { return c.parent.Signer().Enabled() }

// init registers the factory in the root package so kucoin.Client.Spot()
// lazily returns *spot.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/spot" triggers this init.
func init() {
	kucoin.RegisterSpotFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
