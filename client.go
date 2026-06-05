/*
FILE: client.go

DESCRIPTION:
The root SDK client. Holds shared resources (REST transport, signer,
config, logger) and exposes lazy domain sub-clients on demand. Domain
profiles (futures, and later spot, …) are implemented in their own
packages and register a factory at init() time so the root package never
imports them directly (avoids a circular dependency: domain packages
import the root for Config/Error/etc.).

USAGE:

	var cfg kucoin.Config = kucoin.DefaultConfig()
	cfg.APIKey = "..."
	cfg.SecretKey = "..."
	cfg.Passphrase = "..."
	var c, err = kucoin.NewClient(cfg)
	if err != nil { panic(err) }
	defer c.Close()

	// Once the futures package is imported (anonymously is fine):
	//   import _ "github.com/tonymontanov/go-kucoin/v2/futures"
	var fut = c.Futures().(*futures.Client)

The .(*futures.Client) cast is by design: the root package returns `any`
because it cannot know about the futures.Client type without importing
the futures package (which already imports root). The cast is a single
line and keeps the SDK structure flat.
*/

package kucoin

import (
	"sync"

	"github.com/tonymontanov/go-kucoin/v2/internal/auth"
	"github.com/tonymontanov/go-kucoin/v2/internal/kcerr"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// Client is the root SDK object. Safe for concurrent use; methods on Client
// itself are stateless apart from the lazy sub-client cache.
type Client struct {
	cfg    Config
	signer *auth.Signer
	rest   *rest.Client
	logger Logger

	futuresOnce sync.Once
	futuresVal  any

	spotOnce sync.Once
	spotVal  any
}

// NewClient validates cfg, fills defaults, and returns a configured root
// client. Returns an *Error with ErrorKindInvalidRequest on configuration
// problems.
func NewClient(cfg Config) (*Client, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var signer *auth.Signer = auth.NewSigner(cfg.APIKey, cfg.SecretKey, cfg.Passphrase, cfg.KeyVersion)

	var restCfg rest.Config = rest.Config{
		RequestTimeout:      cfg.REST.RequestTimeout,
		MaxIdleConns:        cfg.REST.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.REST.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.REST.IdleConnTimeout,
		RateLimitObserver:   cfg.RateLimitObserver,
	}
	// Forward the typed event observer through a thin adapter. The public
	// RateLimitEvent struct lives in the root package and CANNOT be passed
	// directly into internal/rest (import cycle). The transport invokes the
	// callback with flat arguments and we assemble RateLimitEvent here.
	if cfg.RateLimitEventObserver != nil {
		var userObserver = cfg.RateLimitEventObserver
		restCfg.RateLimitEventObserver = func(endpoint, method string, headers map[string]string, meta rest.RequestMeta) {
			userObserver(RateLimitEvent{
				Endpoint:   endpoint,
				Method:     method,
				Headers:    headers,
				OrderCount: meta.OrderCount,
				Symbols:    meta.Symbols,
				Category:   RateLimitCategory(meta.Category),
			})
		}
	}

	var restClient *rest.Client = rest.NewClient(cfg.REST.BaseURL, signer, restCfg, cfg.UserAgent, cfg.Logger)

	return &Client{
		cfg:    cfg,
		signer: signer,
		rest:   restClient,
		logger: cfg.Logger,
	}, nil
}

// Config returns a copy of the resolved Config (after defaults applied).
func (c *Client) Config() Config { return c.cfg }

// Logger returns the configured logger. Useful for the same logger to be
// reused by a desk-side adapter.
func (c *Client) Logger() Logger { return c.logger }

// Signer is exposed to internal SDK sub-packages (futures, spot, …) so they
// can sign WS bullet-token requests. User code SHOULD NOT touch it.
func (c *Client) Signer() *auth.Signer { return c.signer }

// REST is exposed to internal SDK sub-packages.
func (c *Client) REST() *rest.Client { return c.rest }

// Close releases idle HTTP connections. WS connections owned by domain
// sub-clients close on their own contexts.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.rest.Close()
	return nil
}

// ----------------------------------------------------------------------------
// Sub-client factories (registered by domain packages via init).
// ----------------------------------------------------------------------------

// futuresFactory is set by futures.init() via RegisterFuturesFactory.
var futuresFactory func(c *Client) any

// RegisterFuturesFactory wires the futures.Client builder. Idempotent —
// only the first call is honoured.
func RegisterFuturesFactory(f func(c *Client) any) {
	if futuresFactory == nil {
		futuresFactory = f
	}
}

// Futures returns the *futures.Client (typed as any for import-cycle
// reasons). nil when the futures package has not been imported. Default
// profile in v1.0 (KuCoin USD-M perpetual futures).
func (c *Client) Futures() any {
	c.futuresOnce.Do(func() {
		if futuresFactory == nil {
			c.logger.Warn(`kucoin.Client.Futures: futures factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/futures"`)
			return
		}
		c.futuresVal = futuresFactory(c)
	})
	return c.futuresVal
}

// spotFactory is set by spot.init() via RegisterSpotFactory.
var spotFactory func(c *Client) any

// RegisterSpotFactory wires the spot.Client builder. Idempotent. Available
// from v2.0; v1.0 ships only the futures profile.
func RegisterSpotFactory(f func(c *Client) any) {
	if spotFactory == nil {
		spotFactory = f
	}
}

// Spot returns the *spot.Client (typed as any). nil when the spot package
// has not been imported. Available from v2.0.
func (c *Client) Spot() any {
	c.spotOnce.Do(func() {
		if spotFactory == nil {
			c.logger.Warn(`kucoin.Client.Spot: spot factory is not registered; available from v2.0`)
			return
		}
		c.spotVal = spotFactory(c)
	})
	return c.spotVal
}

// Compile-time assertion: *Error implements the error interface.
var _ error = (*kcerr.Error)(nil)
