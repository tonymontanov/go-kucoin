/*
FILE: earn/helpers.go

DESCRIPTION:
Shared helpers for the Earn profile: validation/auth error constructors, REST
invocation wrappers that decode the KuCoin envelope's data field, and small
conversion helpers. Mirrors the spot/margin/account profile helper surface.
*/

package earn

import (
	"context"
	"net/url"
	"strconv"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// queryMeta is the rate-limit metadata stamped on Earn calls (forwarded 1:1 to
// the observer).
var queryMeta = rest.RequestMeta{Category: "query"}

// errInvalidRequest is the canonical client-side validation error.
func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "earn."+method+": "+msg, nil)
}

// errAuthRequired is returned by signed calls when the client has no
// credentials configured.
func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "earn."+method+": signed endpoint requires API credentials", nil)
}

// doGET issues a signed GET and decodes data into dest.
func (c *Client) doGET(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "GET", path, query, nil, dest)
}

// doPOST issues a signed POST with a JSON body and decodes data into dest.
func (c *Client) doPOST(ctx context.Context, path string, body any, dest any) error {
	return c.do(ctx, "POST", path, nil, body, dest)
}

// doDELETE issues a signed DELETE and decodes data into dest.
func (c *Client) doDELETE(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "DELETE", path, query, nil, dest)
}

// do is the shared REST invocation core. Every Earn endpoint is signed, so it
// short-circuits with errAuthRequired when no credentials are configured.
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any, dest any) error {
	if !c.signerEnabled() {
		return errAuthRequired(method + " " + path)
	}
	var opts rest.Options = rest.Options{
		Method: method,
		Path:   path,
		Body:   body,
		Signed: true,
		Meta:   queryMeta,
	}
	if len(query) > 0 {
		opts.Query = toValues(query)
	}
	var resp rest.Response
	var err error
	resp, _, err = c.rest().Do(ctx, opts)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	return resp.UnmarshalData(dest)
}

// toValues converts a flat string map into url.Values.
func toValues(m map[string]string) url.Values {
	var v url.Values = make(url.Values, len(m))
	var k, val string
	for k, val = range m {
		v[k] = []string{val}
	}
	return v
}

// itoa is a local shortcut for base-10 int formatting.
func itoa(v int) string { return strconv.Itoa(v) }
