/*
FILE: subaccount/helpers.go

DESCRIPTION:
Shared helpers for the Sub-Account profile: validation/auth error constructors,
the signed REST invocation core, and a flexInt64 that tolerates timestamps
delivered as either bare JSON numbers or quoted strings (KuCoin returns
createdAt inconsistently across these endpoints).
*/

package subaccount

import (
	"context"
	"net/url"
	"strconv"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// queryMeta is the rate-limit metadata stamped on calls.
var queryMeta = rest.RequestMeta{Category: "query"}

// errInvalidRequest is the canonical client-side validation error.
func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "subaccount."+method+": "+msg, nil)
}

// errAuthRequired is returned when no credentials are configured.
func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "subaccount."+method+": signed endpoint requires API credentials", nil)
}

// flexInt64 decodes a JSON value that may be a bare number or a quoted string.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string = string(b[1 : len(b)-1])
		if s == "" {
			*f = 0
			return nil
		}
		var v, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*f = flexInt64(v)
		return nil
	}
	var v, err = strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt64(v)
	return nil
}

// doGET / doPOST / doDELETE issue signed requests and decode data into dest.
func (c *Client) doGET(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "GET", path, query, nil, dest)
}

func (c *Client) doPOST(ctx context.Context, path string, body any, dest any) error {
	return c.do(ctx, "POST", path, nil, body, dest)
}

func (c *Client) doDELETE(ctx context.Context, path string, query map[string]string, dest any) error {
	return c.do(ctx, "DELETE", path, query, nil, dest)
}

// do is the shared signed REST invocation core.
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any, dest any) error {
	if !c.signerEnabled() {
		return errAuthRequired(method + " " + path)
	}
	var opts rest.Options = rest.Options{Method: method, Path: path, Body: body, Signed: true, Meta: queryMeta}
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
