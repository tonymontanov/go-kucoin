/*
FILE: internal/rest/client.go

DESCRIPTION:
Low-level KuCoin REST client. Stays at the transport / envelope layer:

  - assembles the URL (BaseURL + path + canonical query);
  - signs requests via auth.Signer (KC-API-* headers);
  - executes the HTTP call honouring ctx deadline / Config.RequestTimeout;
  - parses the KuCoin envelope { code, data, msg };
  - maps non-success code and 4xx/5xx HTTP statuses into *kcerr.Error with
    the right Kind via kcerr.MapKucoinCode / kcerr.MapHTTPStatus;
  - notifies the legacy and event-based rate-limit observers with the
    KuCoin gateway rate-limit headers.

KUCOIN ENVELOPE:
KuCoin wraps every response in { "code": "200000", "data": {...}, "msg": "" }.
"200000" is success; any other code is an application-level error, even
on HTTP 200. The transport classifies it via kcerr.MapKucoinCode.

RATE-LIMIT HEADERS:
KuCoin returns gw-ratelimit-limit / gw-ratelimit-remaining /
gw-ratelimit-reset on most responses. They are forwarded 1:1 so an
external rate-limiter at the desk level can reconcile its model with the
live remaining budget.

DESIGN:
  - The package does NOT import the public root (which imports rest), so
    everything it needs lives in internal/* (auth, codec, kcerr, kclog).
  - Domain layers (futures/trading.go, etc.) call Do() with a populated
    Options.Meta describing OrderCount / Symbols / Category. The metadata
    is forwarded to the event-observer 1:1.
*/

package rest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonymontanov/go-kucoin/v2/internal/auth"
	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
	"github.com/tonymontanov/go-kucoin/v2/internal/kcerr"
	"github.com/tonymontanov/go-kucoin/v2/internal/kclog"
)

// Config — REST transport parameters. Populated from the public Config.REST
// in the root package via explicit field copy (avoids an import cycle).
type Config struct {
	// RequestTimeout — global timeout for a single REST call. ctx with its
	// own deadline overrides this for a particular request.
	RequestTimeout time.Duration
	// MaxIdleConns — http.Transport pool size.
	MaxIdleConns int
	// MaxIdleConnsPerHost — http.Transport per-host pool size.
	MaxIdleConnsPerHost int
	// IdleConnTimeout — keep-alive idle timeout.
	IdleConnTimeout time.Duration
	// RateLimitObserver — legacy callback. Receives only (endpoint, headers).
	// nil → no-op.
	RateLimitObserver func(endpoint string, headers map[string]string)
	// RateLimitEventObserver — primary callback. Receives every REST
	// response with the full RequestMeta plus the live rate-limit headers.
	//
	// Speed contract: called synchronously from the goroutine that issued
	// the REST request. Implementations must be O(1) — typically a
	// non-blocking send to a buffered channel.
	RateLimitEventObserver func(endpoint, method string, headers map[string]string, meta RequestMeta)
}

// RequestMeta — domain-level information about the request that the
// external rate-limiter needs to model KuCoin limits accurately. Set by
// the calling domain method (futures/trading.go etc.) at the point where
// the symbol set, batch size and category are known.
//
//   - OrderCount: 1 for single-order endpoints, len(orders) for batch
//     endpoints, 0 for non-trading queries.
//   - Symbols:    unique list of symbols affected by the request.
//   - Category:   "place" | "amend" | "cancel" | "query" | "market" | "".
//   - Endpoint:   populated by Do() before invoking the observer; ignored
//     on input.
type RequestMeta struct {
	OrderCount int
	Symbols    []string
	Category   string
	Endpoint   string
}

// Options — parameters for a single REST request.
type Options struct {
	// Method — HTTP method, upper-case ("GET", "POST", ...). The client
	// also tolerates lower-case and upper-cases internally.
	Method string
	// Path — request path including the leading "/" (e.g. "/api/v1/orders").
	Path string
	// Query — query parameters; serialized in URL-encoded form. For signed
	// GET requests the canonical query string is part of the signature
	// pre-hash (KuCoin signs path?query for GET).
	Query url.Values
	// Body — JSON body. Marshalled by codec; the resulting bytes are used
	// both for the wire and for the signature pre-hash. Pass nil for GET.
	Body any
	// Signed — true for endpoints that require KC-API-SIGN.
	Signed bool
	// Meta — request metadata for the rate-limit observer.
	Meta RequestMeta
}

// Response — KuCoin response envelope. data is kept as raw JSON so domain
// methods can decode into typed structs without re-marshalling.
type Response struct {
	Code string        `json:"code"`
	Msg  string        `json:"msg"`
	Data codec.RawJSON `json:"data"`
}

// UnmarshalData decodes the Data field into dest. No-op if Data is missing
// or "null".
func (r Response) UnmarshalData(dest any) error {
	if r.Data.IsNull() {
		return nil
	}
	return codec.Unmarshal(r.Data, dest)
}

// Client — low-level REST client.
type Client struct {
	httpClient             *http.Client
	signer                 *auth.Signer
	baseURL                string
	userAgent              string
	logger                 kclog.Logger
	rateLimitObserver      func(endpoint string, headers map[string]string)
	rateLimitEventObserver func(endpoint, method string, headers map[string]string, meta RequestMeta)
}

// NewClient creates a REST client. signer may have empty credentials —
// public endpoints will still work, signed calls surface
// auth.ErrSignerDisabled at call time.
func NewClient(baseURL string, signer *auth.Signer, cfg Config, ua string, log kclog.Logger) *Client {
	if log == nil {
		log = kclog.Noop()
	}
	var transport *http.Transport = &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		ForceAttemptHTTP2:   true,
	}
	var httpClient *http.Client = &http.Client{
		Timeout:   cfg.RequestTimeout,
		Transport: transport,
	}
	return &Client{
		httpClient:             httpClient,
		signer:                 signer,
		baseURL:                strings.TrimRight(baseURL, "/"),
		userAgent:              ua,
		logger:                 log,
		rateLimitObserver:      cfg.RateLimitObserver,
		rateLimitEventObserver: cfg.RateLimitEventObserver,
	}
}

// Close releases idle transport connections.
func (c *Client) Close() {
	if c == nil || c.httpClient == nil {
		return
	}
	if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

/*
Do executes a single REST call. Returns the response envelope, the
collected rate-limit headers, and an error.

Error semantics:
  - context cancel/deadline / network failures → *kcerr.Error with
    Kind=Network, Cause=underlying error.
  - HTTP 4xx/5xx without a parseable envelope → *kcerr.Error with
    Kind = kcerr.MapHTTPStatus(status), HTTPStatus set, Message=truncated body.
  - HTTP 2xx with code != "200000"          → *kcerr.Error with
    Kind = kcerr.MapKucoinCode(code, msg), KucoinCode=code, Message=msg.
  - HTTP 4xx/5xx WITH a parseable envelope    → same as above.

The rate-limit headers map is always non-nil but may be empty. It is
allocated fresh on every call so observers may safely retain references.
*/
func (c *Client) Do(ctx context.Context, opts Options) (Response, map[string]string, error) {
	var resp Response
	var rateHeaders map[string]string = map[string]string{}

	var fullURL string
	var bodyStr string
	var signPath string
	var err error
	fullURL, bodyStr, signPath, err = c.buildRequest(opts)
	if err != nil {
		return resp, rateHeaders, err
	}

	var method string = strings.ToUpper(opts.Method)

	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, method, fullURL, bytes.NewBufferString(bodyStr))
	if err != nil {
		return resp, rateHeaders, kcerr.New(kcerr.ErrorKindInvalidRequest, "", "rest: build request", err)
	}

	c.applyHeaders(req, opts, method, bodyStr, signPath)

	var httpResp *http.Response
	var started time.Time = time.Now()
	httpResp, err = c.httpClient.Do(req)
	if err != nil {
		return resp, rateHeaders, classifyTransportError(err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	rateHeaders = collectRateLimitHeaders(httpResp.Header)

	if c.rateLimitObserver != nil || c.rateLimitEventObserver != nil {
		var meta RequestMeta = opts.Meta
		meta.Endpoint = opts.Path
		if c.rateLimitObserver != nil {
			c.rateLimitObserver(opts.Path, rateHeaders)
		}
		if c.rateLimitEventObserver != nil {
			c.rateLimitEventObserver(opts.Path, method, rateHeaders, meta)
		}
	}

	var raw []byte
	raw, err = io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, rateHeaders, kcerr.New(kcerr.ErrorKindNetwork, "", "rest: read body", err)
	}

	c.logger.Debug(
		"rest.Do",
		kclog.Str("method", method),
		kclog.Str("path", opts.Path),
		kclog.Int("status", int64(httpResp.StatusCode)),
		kclog.Int("durationMs", time.Since(started).Milliseconds()),
		kclog.Int("bytes", int64(len(raw))),
	)

	// KuCoin returns the { code, msg, data } envelope on every status,
	// including 4xx for application-level errors. Decode on every status.
	var parseErr error = codec.Unmarshal(raw, &resp)

	if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
		if parseErr != nil {
			return resp, rateHeaders, kcerr.New(kcerr.ErrorKindUnknown, "", "rest: parse response", parseErr)
		}
		if resp.Code != kcerr.CodeOK && resp.Code != "" {
			return resp, rateHeaders, &kcerr.Error{
				Kind:       kcerr.MapKucoinCode(resp.Code, resp.Msg),
				HTTPStatus: httpResp.StatusCode,
				KucoinCode: resp.Code,
				Message:    resp.Msg,
			}
		}
		return resp, rateHeaders, nil
	}

	// Non-2xx path. Prefer the typed envelope when available.
	if parseErr == nil && resp.Code != "" && resp.Code != kcerr.CodeOK {
		return resp, rateHeaders, &kcerr.Error{
			Kind:       kcerr.MapKucoinCode(resp.Code, resp.Msg),
			HTTPStatus: httpResp.StatusCode,
			KucoinCode: resp.Code,
			Message:    resp.Msg,
		}
	}
	return resp, rateHeaders, &kcerr.Error{
		Kind:       kcerr.MapHTTPStatus(httpResp.StatusCode),
		HTTPStatus: httpResp.StatusCode,
		Message:    truncate(string(raw), 256),
	}
}

// buildRequest assembles the URL, the body string and the signature path
// (path + canonical query). KuCoin signs the FULL request path including
// query string for GET requests, so signPath = "/api/v1/...?k1=v1&k2=v2".
// For POST the canonical query is empty and signPath is just opts.Path.
func (c *Client) buildRequest(opts Options) (string, string, string, error) {
	var u *url.URL
	var err error
	u, err = url.Parse(c.baseURL + opts.Path)
	if err != nil {
		return "", "", "", kcerr.New(kcerr.ErrorKindInvalidRequest, "", "rest: invalid url", err)
	}
	var canonicalQuery string
	if len(opts.Query) > 0 {
		canonicalQuery = opts.Query.Encode()
		u.RawQuery = canonicalQuery
	}

	var bodyStr string
	if opts.Body != nil {
		var raw []byte
		raw, err = codec.Marshal(opts.Body)
		if err != nil {
			return "", "", "", kcerr.New(kcerr.ErrorKindInvalidRequest, "", "rest: marshal body", err)
		}
		bodyStr = string(raw)
	}

	var signPath string = opts.Path
	if canonicalQuery != "" {
		signPath = signPath + "?" + canonicalQuery
	}
	return u.String(), bodyStr, signPath, nil
}

// applyHeaders sets common headers and, for signed calls, the KuCoin
// KC-API-* headers.
func (c *Client) applyHeaders(req *http.Request, opts Options, method, body, signPath string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	if !opts.Signed {
		return
	}
	if c.signer == nil || !c.signer.Enabled() {
		// Surface signing failure later via auth.ErrSignerDisabled at the
		// call site. The transport keeps going so public-only embedders
		// are not broken.
		return
	}

	var ts string = c.signer.MillisTimestamp(time.Now())
	var signature string
	var err error
	signature, err = c.signer.SignREST(ts, method, signPath, body)
	if err != nil {
		c.logger.Warn("rest: sign skipped", kclog.Err(err))
		return
	}
	req.Header.Set("KC-API-KEY", c.signer.APIKey())
	req.Header.Set("KC-API-SIGN", signature)
	req.Header.Set("KC-API-TIMESTAMP", ts)
	req.Header.Set("KC-API-PASSPHRASE", c.signer.EncodedPassphrase())
	req.Header.Set("KC-API-KEY-VERSION", c.signer.KeyVersion())
}

// rateLimitHeaderAllowList enumerates the headers KuCoin ships with
// rate-limit metadata. Hard-coded to avoid leaking unrelated headers
// (cookies, auth) into observer maps that may be logged downstream.
var rateLimitHeaderAllowList = map[string]struct{}{
	"gw-ratelimit-limit":     {},
	"gw-ratelimit-remaining": {},
	"gw-ratelimit-reset":     {},
	"retry-after":            {},
}

// collectRateLimitHeaders extracts the rate-limit metadata that KuCoin
// returns. The returned map is fresh on every call.
func collectRateLimitHeaders(h http.Header) map[string]string {
	var out map[string]string = map[string]string{}
	var name string
	var values []string
	for name, values = range h {
		if len(values) == 0 {
			continue
		}
		var lower string = strings.ToLower(name)
		if _, ok := rateLimitHeaderAllowList[lower]; ok {
			out[name] = values[0]
		}
	}
	return out
}

// classifyTransportError converts a transport error into *kcerr.Error
// with Kind=Network. Distinguishes context cancel / deadline so callers
// can use errors.Is(err, context.Canceled) when needed.
func classifyTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return kcerr.New(kcerr.ErrorKindNetwork, "", "rest: context canceled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return kcerr.New(kcerr.ErrorKindNetwork, "", "rest: deadline exceeded", err)
	}
	return kcerr.New(kcerr.ErrorKindNetwork, "", "rest: transport error", err)
}

// truncate returns at most n bytes of s, appending an ellipsis when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
