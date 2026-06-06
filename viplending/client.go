/*
FILE: viplending/client.go

DESCRIPTION:
Root client for the KuCoin VIP Lending (OTC loan) profile (v2.5 Phase C). Holds
a reference to the parent kucoin.Client and a DEDICATED REST client bound to the
spot host (api.kucoin.com). The surface is three read-only queries, so the
methods live directly on this client (see viplending.go).

CONTRACT:
  - Client is safe for concurrent use; it holds no mutable state after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package viplending

import (
	"context"
	"net/url"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// queryMeta is the rate-limit metadata stamped on calls.
var queryMeta = rest.RequestMeta{Category: "query"}

// Client — KuCoin VIP Lending profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string
}

// NewClient creates a VIP Lending profile client. Returns nil if parent is nil.
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

// errAuthRequired is returned by signed calls when no credentials are set.
func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "viplending."+method+": signed endpoint requires API credentials", nil)
}

// doGET issues a signed GET and decodes data into dest.
func (c *Client) doGET(ctx context.Context, path string, query map[string]string, dest any) error {
	if !c.parent.Signer().Enabled() {
		return errAuthRequired("GET " + path)
	}
	var opts rest.Options = rest.Options{Method: "GET", Path: path, Signed: true, Meta: queryMeta}
	if len(query) > 0 {
		var v url.Values = make(url.Values, len(query))
		var k, val string
		for k, val = range query {
			v[k] = []string{val}
		}
		opts.Query = v
	}
	var resp rest.Response
	var err error
	resp, _, err = c.restCli.Do(ctx, opts)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	return resp.UnmarshalData(dest)
}

// init registers the factory in the root package so kucoin.Client.VIPLending()
// lazily returns *viplending.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/viplending" triggers this init.
func init() {
	kucoin.RegisterVIPLendingFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
