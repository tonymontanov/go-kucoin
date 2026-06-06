/*
FILE: convert/client.go

DESCRIPTION:
Root client for the KuCoin Convert profile (v2.5 Phase E). Holds a reference to
the parent kucoin.Client and a DEDICATED REST client bound to the spot host
(api.kucoin.com). The surface is small and cohesive, so the methods live
directly on this client (see convert.go).

CONTRACT:
  - Client is safe for concurrent use; it holds no mutable state after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package convert

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// Client — KuCoin Convert profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string
}

// NewClient creates a Convert profile client. Returns nil if parent is nil.
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

func (c *Client) rest() *rest.Client  { return c.restCli }
func (c *Client) signerEnabled() bool { return c.parent.Signer().Enabled() }

// init registers the factory so kucoin.Client.Convert() lazily returns
// *convert.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/convert" triggers this init.
func init() {
	kucoin.RegisterConvertFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
