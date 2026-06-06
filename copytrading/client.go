/*
FILE: copytrading/client.go

DESCRIPTION:
Root client for the KuCoin futures Copy-Trading profile (v2.5 Phase F). Holds a
reference to the parent kucoin.Client and uses the parent's FUTURES-bound REST
client (api-futures.kucoin.com), exactly like the futures profile. The surface
is a flat set of lead-trader endpoints (see copytrading.go).
*/

package copytrading

import (
	"context"
	"net/url"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// tradeMeta / queryMeta stamp rate-limit categories on calls.
var (
	tradeMeta = rest.RequestMeta{Category: "trade"}
	queryMeta = rest.RequestMeta{Category: "query"}
)

// Client — KuCoin futures Copy-Trading profile client.
type Client struct {
	parent *kucoin.Client
}

// NewClient creates a Copy-Trading profile client. Returns nil if parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	if parent == nil {
		return nil
	}
	return &Client{parent: parent}
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

func (c *Client) signerEnabled() bool { return c.parent.Signer().Enabled() }

func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "copytrading."+method+": "+msg, nil)
}

func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "copytrading."+method+": signed endpoint requires API credentials", nil)
}

func (c *Client) doGET(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "GET", path, query, nil, queryMeta, dest)
}

func (c *Client) doPOST(ctx context.Context, path string, body any, dest any) error {
	return c.do(ctx, "POST", path, nil, body, tradeMeta, dest)
}

func (c *Client) doDELETE(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "DELETE", path, query, nil, tradeMeta, dest)
}

// do is the shared signed REST invocation core, bound to the futures host.
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any, meta rest.RequestMeta, dest any) error {
	if !c.signerEnabled() {
		return errAuthRequired(method + " " + path)
	}
	var opts rest.Options = rest.Options{Method: method, Path: path, Body: body, Signed: true, Meta: meta}
	if len(query) > 0 {
		opts.Query = toValues(query)
	}
	var resp rest.Response
	var err error
	resp, _, err = c.parent.REST().Do(ctx, opts)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	return resp.UnmarshalData(dest)
}

func toValues(m map[string]string) url.Values {
	var v url.Values = make(url.Values, len(m))
	var k, val string
	for k, val = range m {
		v[k] = []string{val}
	}
	return v
}

// init registers the factory so kucoin.Client.CopyTrading() lazily returns
// *copytrading.Client.
func init() {
	kucoin.RegisterCopyTradingFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
