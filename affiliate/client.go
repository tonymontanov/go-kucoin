/*
FILE: affiliate/client.go

DESCRIPTION:
Root client for the KuCoin Affiliate profile (v2.5 Phase F). Holds a reference
to the parent kucoin.Client and a DEDICATED REST client bound to the spot host
(api.kucoin.com). The surface is two read-only reports, so the methods live
directly on this client (see affiliate.go).
*/

package affiliate

import (
	"context"
	"net/url"
	"strconv"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

var queryMeta = rest.RequestMeta{Category: "query"}

// Client — KuCoin Affiliate profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string
}

// NewClient creates an Affiliate profile client. Returns nil if parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	if parent == nil {
		return nil
	}
	var base string = kucoin.SpotFamilyBaseURL(parent.Config())
	return &Client{parent: parent, restCli: parent.NewSectionRESTClient(base), baseURL: base}
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "affiliate."+method+": signed endpoint requires API credentials", nil)
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

func itoa(v int) string     { return strconv.Itoa(v) }
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// init registers the factory so kucoin.Client.Affiliate() lazily returns
// *affiliate.Client.
func init() {
	kucoin.RegisterAffiliateFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
