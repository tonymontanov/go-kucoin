/*
FILE: earn/client.go

DESCRIPTION:
Root client for the KuCoin Earn profile (v2.5 Phase C). Holds a reference to the
parent kucoin.Client (signer, logger, config) and a DEDICATED REST client bound
to the spot host (api.kucoin.com) — Earn lives on the spot host, while the root
REST client defaults to the futures host. The Earn surface is small and
cohesive, so the methods live directly on this client (see products.go /
orders.go) rather than behind sub-clients.

CONTRACT:
  - Client is safe for concurrent use; it holds no mutable state after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package earn

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// Client — KuCoin Earn profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string
}

// NewClient creates an Earn profile client. Returns nil if parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	if parent == nil {
		return nil
	}
	var base string = kucoin.SpotFamilyBaseURL(parent.Config())
	return &Client{
		parent:  parent,
		restCli: parent.NewSectionRESTClient(base),
		baseURL: base,
	}
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

// Internal shortcuts.
func (c *Client) rest() *rest.Client  { return c.restCli }
func (c *Client) signerEnabled() bool { return c.parent.Signer().Enabled() }

// init registers the factory in the root package so kucoin.Client.Earn()
// lazily returns *earn.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/earn" triggers this init.
func init() {
	kucoin.RegisterEarnFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
