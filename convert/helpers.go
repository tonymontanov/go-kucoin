/*
FILE: convert/helpers.go

DESCRIPTION:
Shared helpers for the Convert profile: validation/auth error constructors, the
public + signed REST invocation cores, a flexStr that normalises order IDs that
arrive as a quoted string OR a bare number, and small query/format helpers.
*/

package convert

import (
	"context"
	"net/url"
	"strconv"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// queryMeta is the rate-limit metadata stamped on calls.
var queryMeta = rest.RequestMeta{Category: "query"}

func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "convert."+method+": "+msg, nil)
}

func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "convert."+method+": signed endpoint requires API credentials", nil)
}

// flexStr decodes a JSON value that may be a quoted string or a bare number
// (KuCoin returns convert orderId as a string for limit orders and a number for
// market-order detail) and normalises it to a string.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		*f = flexStr(b[1 : len(b)-1])
		return nil
	}
	*f = flexStr(b)
	return nil
}

// doPublicGET issues an UNSIGNED GET (public endpoints) and decodes data.
func (c *Client) doPublicGET(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "GET", path, query, nil, false, dest)
}

// doGET / doPOST / doDELETE issue SIGNED requests and decode data into dest.
func (c *Client) doGET(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "GET", path, query, nil, true, dest)
}

func (c *Client) doPOST(ctx context.Context, path string, body any, dest any) error {
	return c.do(ctx, "POST", path, nil, body, true, dest)
}

func (c *Client) doDELETE(ctx context.Context, path string, body any, dest any) error {
	return c.do(ctx, "DELETE", path, nil, body, true, dest)
}

// do is the shared REST invocation core.
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any, signed bool, dest any) error {
	if signed && !c.signerEnabled() {
		return errAuthRequired(method + " " + path)
	}
	var opts rest.Options = rest.Options{Method: method, Path: path, Body: body, Signed: signed, Meta: queryMeta}
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

// itoa / itoa64 are local base-10 formatting shortcuts.
func itoa(v int) string     { return strconv.Itoa(v) }
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
