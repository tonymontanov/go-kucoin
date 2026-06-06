/*
FILE: account/helpers.go

DESCRIPTION:
Shared helpers for the KuCoin Account & Funding sub-clients: validation/auth
error constructors, client-order-id generation (flex transfer idempotency),
REST invocation wrappers that decode the KuCoin envelope's data field, and
small conversion helpers. Mirrors the spot/margin profile helper surface so the
profiles stay consistent.
*/

package account

import (
	"context"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// Rate-limit metadata stamped on calls (forwarded 1:1 to the observer).
var (
	queryMeta  = rest.RequestMeta{Category: "query"}
	marketMeta = rest.RequestMeta{Category: "market"}
)

// errInvalidRequest is the canonical client-side validation error.
func errInvalidRequest(method, msg string) error {
	return kucoin.NewError(kucoin.ErrorKindInvalidRequest, "", "account."+method+": "+msg, nil)
}

// errAuthRequired is returned by signed calls when the client has no
// credentials configured.
func errAuthRequired(method string) error {
	return kucoin.NewError(kucoin.ErrorKindAuth, "", "account."+method+": signed endpoint requires API credentials", nil)
}

// clientOidSeq backs generateClientOid.
var clientOidSeq uint64

// generateClientOid produces a process-unique client order id used as the flex
// transfer idempotency token when the caller leaves it empty.
func generateClientOid() string {
	return "kca-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" +
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

// do is the shared REST invocation core: it assembles rest.Options, calls the
// transport and decodes the data field. Signed calls short-circuit with
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

// itoa64 is a local shortcut for base-10 int64 formatting.
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
