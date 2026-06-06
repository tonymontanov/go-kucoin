/*
FILE: margin/helpers.go

DESCRIPTION:
Shared helpers for the KuCoin Margin sub-clients: validation/auth error
constructors, client-order-id generation, REST invocation wrappers that
decode the KuCoin envelope's data field, and small numeric helpers. Mirrors
the spot profile's helper surface so the two profiles stay consistent.
*/

package margin

import (
	"context"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
	"github.com/tonymontanov/go-kucoin/v2/internal/kclog"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// codecUnmarshal decodes JSON into dest using the SDK's json-iterator codec.
func codecUnmarshal(data []byte, dest any) error { return codec.Unmarshal(data, dest) }

// logStr / logErr are thin field constructors for the structured logger.
func logStr(key, value string) kclog.Field { return kclog.Str(key, value) }
func logErr(err error) kclog.Field         { return kclog.Err(err) }

// errInvalidRequest is the canonical client-side validation error.
func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "margin."+method+": "+msg, nil)
}

// errAuthRequired is returned by signed calls when the client has no
// credentials configured.
func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "margin."+method+": signed endpoint requires API credentials", nil)
}

// clientOidSeq backs generateClientOid.
var clientOidSeq uint64

// generateClientOid produces a process-unique client order id. KuCoin
// requires clientOid on order placement and uses it for idempotency; the
// SDK fills one when the caller leaves it empty.
func generateClientOid() string {
	return "kcm-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" +
		strconv.FormatUint(atomic.AddUint64(&clientOidSeq, 1), 10)
}

// doGET issues a signed/unsigned GET and decodes data into dest.
func (c *Client) doGET(ctx context.Context, signed bool, path string, query map[string]string, meta rest.RequestMeta, dest any) error {
	return c.do(ctx, "GET", signed, path, query, nil, meta, dest)
}

// doPOST issues a signed POST with a JSON body and decodes data into dest.
func (c *Client) doPOST(ctx context.Context, path string, body any, meta rest.RequestMeta, dest any) error {
	return c.do(ctx, "POST", true, path, nil, body, meta, dest)
}

// doDELETE issues a signed DELETE and decodes data into dest.
func (c *Client) doDELETE(ctx context.Context, path string, query map[string]string, meta rest.RequestMeta, dest any) error {
	return c.do(ctx, "DELETE", true, path, query, nil, meta, dest)
}

// do is the shared REST invocation core: it assembles rest.Options, calls
// the transport and decodes the data field. Signed calls short-circuit with
// errAuthRequired when no credentials are configured.
func (c *Client) do(ctx context.Context, method string, signed bool, path string, query map[string]string, body any, meta rest.RequestMeta, dest any) error {
	if signed && !c.signerEnabled() {
		return errAuthRequired(method + " " + path)
	}
	var opts rest.Options = rest.Options{
		Method: method,
		Path:   path,
		Body:   body,
		Signed: signed,
		Meta:   meta,
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

// itoa is a local shortcut for base-10 int64 formatting.
func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// nsToMs converts a nanosecond timestamp to milliseconds. The margin order
// WS channel (shared with spot) ships order timestamps in nanoseconds.
func nsToMs(ns int64) int64 {
	if ns == 0 {
		return 0
	}
	return ns / 1_000_000
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
