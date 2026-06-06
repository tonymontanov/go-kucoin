/*
FILE: margin/client.go

DESCRIPTION:
Root sub-client for the KuCoin Margin profile (v2.5). Holds a reference to the
parent kucoin.Client (signer, logger, config) and a DEDICATED REST client
bound to the spot host (api.kucoin.com) — margin trades on the spot matching
engine, while the root REST client defaults to the futures host. Exposes the
domain sub-clients: MarketData, Trading, Borrow, Account, RiskLimit, Stream.

CONTRACT:
  - Client is safe for concurrent use; sub-clients are read-only after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package margin

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	margintypes "github.com/tonymontanov/go-kucoin/v2/margin/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// ClientSettings — optional account-wide defaults for the Margin profile.
// Empty fields fall back to SDK defaults:
//   - DefaultTradeType "" → MARGIN_TRADE (cross margin).
type ClientSettings struct {
	DefaultTradeType margintypes.TradeType
}

// Client — KuCoin Margin profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string

	defaultTradeType margintypes.TradeType

	trading    *TradingClient
	account    *AccountClient
	marketData *MarketDataClient
	borrow     *BorrowClient
	riskLimit  *RiskLimitClient
	stream     *StreamClient
}

// NewClient creates a Margin profile client with SDK defaults. Returns nil if
// parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	return NewClientWithSettings(parent, ClientSettings{})
}

// NewClientWithSettings creates a Margin profile client with explicit
// defaults. Empty fields fall back to the SDK defaults documented on
// ClientSettings. Returns nil if parent is nil.
func NewClientWithSettings(parent *kucoin.Client, s ClientSettings) *Client {
	if parent == nil {
		return nil
	}
	if s.DefaultTradeType == "" {
		s.DefaultTradeType = margintypes.TradeCross
	}
	var base string = kucoin.SpotFamilyBaseURL(parent.Config())
	var c *Client = &Client{
		parent:           parent,
		restCli:          parent.NewSectionRESTClient(base),
		baseURL:          base,
		defaultTradeType: s.DefaultTradeType,
	}
	c.trading = newTradingClient(c)
	c.account = newAccountClient(c)
	c.marketData = newMarketDataClient(c)
	c.borrow = newBorrowClient(c)
	c.riskLimit = newRiskLimitClient(c)
	c.stream = newStreamClient(c)
	return c
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

// DefaultTradeType returns the resolved default trade type (cross/isolated).
func (c *Client) DefaultTradeType() margintypes.TradeType { return c.defaultTradeType }

// MarketData returns the public market-data sub-client.
func (c *Client) MarketData() *MarketDataClient { return c.marketData }

// Trading returns the signed HF-margin trading sub-client.
func (c *Client) Trading() *TradingClient { return c.trading }

// Borrow returns the signed debit (borrow/repay/interest) sub-client.
func (c *Client) Borrow() *BorrowClient { return c.borrow }

// Account returns the signed cross/isolated margin account sub-client.
func (c *Client) Account() *AccountClient { return c.account }

// RiskLimit returns the signed risk-limit / borrow-config sub-client.
func (c *Client) RiskLimit() *RiskLimitClient { return c.riskLimit }

// Stream returns the WebSocket subscription sub-client (private margin).
func (c *Client) Stream() *StreamClient { return c.stream }

// Internal shortcuts shared by sub-clients.
func (c *Client) logger() kucoin.Logger { return c.parent.Logger() }
func (c *Client) rest() *rest.Client    { return c.restCli }
func (c *Client) config() kucoin.Config { return c.parent.Config() }
func (c *Client) signerEnabled() bool   { return c.parent.Signer().Enabled() }

// init registers the factory in the root package so kucoin.Client.Margin()
// lazily returns *margin.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/margin" triggers this init.
func init() {
	kucoin.RegisterMarginFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
