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

	marginOnce sync.Once
	marginVal  any

	accountOnce sync.Once
	accountVal  any

	earnOnce sync.Once
	earnVal  any

	vipLendingOnce sync.Once
	vipLendingVal  any

	subAccountOnce sync.Once
	subAccountVal  any

	convertOnce sync.Once
	convertVal  any

	affiliateOnce sync.Once
	affiliateVal  any

	copyTradingOnce sync.Once
	copyTradingVal  any
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

	var restClient *rest.Client = rest.NewClient(cfg.REST.BaseURL, signer, buildRestConfig(cfg), cfg.UserAgent, cfg.Logger)

	return &Client{
		cfg:    cfg,
		signer: signer,
		rest:   restClient,
		logger: cfg.Logger,
	}, nil
}

// buildRestConfig copies the public RestConfig + observer hooks into the
// internal transport config. The typed RateLimitEvent observer is forwarded
// through a thin adapter because the public RateLimitEvent struct lives in
// the root package and CANNOT be passed directly into internal/rest (import
// cycle): the transport invokes the callback with flat arguments and we
// assemble the event here. Shared by NewClient and NewSectionRESTClient so
// section profiles on a different host inherit the same observer wiring.
func buildRestConfig(cfg Config) rest.Config {
	var restCfg rest.Config = rest.Config{
		RequestTimeout:      cfg.REST.RequestTimeout,
		MaxIdleConns:        cfg.REST.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.REST.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.REST.IdleConnTimeout,
		RateLimitObserver:   cfg.RateLimitObserver,
	}
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
	return restCfg
}

// NewSectionRESTClient builds a REST client bound to baseURL, reusing this
// client's signer, transport tuning and rate-limit observers. Section
// profiles that talk to a DIFFERENT host than the default REST client (e.g.
// Spot on api.kucoin.com while the root defaults to the futures host) call
// this so they share credentials and observability without a second signer.
//
// The returned client is owned by the caller; Close it (or rely on process
// exit) — the root Client.Close only releases the default transport.
func (c *Client) NewSectionRESTClient(baseURL string) *rest.Client {
	return rest.NewClient(baseURL, c.signer, buildRestConfig(c.cfg), c.cfg.UserAgent, c.logger)
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

// marginFactory is set by margin.init() via RegisterMarginFactory.
var marginFactory func(c *Client) any

// RegisterMarginFactory wires the margin.Client builder. Idempotent.
// Available from v2.5 (HF cross/isolated margin trading).
func RegisterMarginFactory(f func(c *Client) any) {
	if marginFactory == nil {
		marginFactory = f
	}
}

// Margin returns the *margin.Client (typed as any). nil when the margin
// package has not been imported. Available from v2.5. The margin profile
// shares the spot host (api.kucoin.com) and the spot public market data /
// order book; it adds HF margin trading, borrow/repay, risk limit and the
// cross/isolated margin accounts.
func (c *Client) Margin() any {
	c.marginOnce.Do(func() {
		if marginFactory == nil {
			c.logger.Warn(`kucoin.Client.Margin: margin factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/margin"`)
			return
		}
		c.marginVal = marginFactory(c)
	})
	return c.marginVal
}

// accountFactory is set by account.init() via RegisterAccountFactory.
var accountFactory func(c *Client) any

// RegisterAccountFactory wires the account.Client builder. Idempotent.
// Available from v2.5 (Account & Funding: balances/ledgers, deposit,
// withdrawal, transfer, fee, currencies).
func RegisterAccountFactory(f func(c *Client) any) {
	if accountFactory == nil {
		accountFactory = f
	}
}

// Account returns the *account.Client (typed as any). nil when the account
// package has not been imported. Available from v2.5. The account profile is
// cross-cutting "treasury" on the spot host (api.kucoin.com): account summary
// / balances / ledgers, deposit & withdrawal, inter-wallet transfers, trade
// fees and the currency directory.
func (c *Client) Account() any {
	c.accountOnce.Do(func() {
		if accountFactory == nil {
			c.logger.Warn(`kucoin.Client.Account: account factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/account"`)
			return
		}
		c.accountVal = accountFactory(c)
	})
	return c.accountVal
}

// earnFactory is set by earn.init() via RegisterEarnFactory.
var earnFactory func(c *Client) any

// RegisterEarnFactory wires the earn.Client builder. Idempotent.
// Available from v2.5 (KuCoin Earn: products, purchase/redeem, holdings).
func RegisterEarnFactory(f func(c *Client) any) {
	if earnFactory == nil {
		earnFactory = f
	}
}

// Earn returns the *earn.Client (typed as any). nil when the earn package has
// not been imported. Available from v2.5. The earn profile lives on the spot
// host (api.kucoin.com) and covers KuCoin Earn: product catalogues (savings /
// promotion / staking / KCS / ETH), subscribe (purchase) / redeem (+ preview)
// and current holdings.
func (c *Client) Earn() any {
	c.earnOnce.Do(func() {
		if earnFactory == nil {
			c.logger.Warn(`kucoin.Client.Earn: earn factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/earn"`)
			return
		}
		c.earnVal = earnFactory(c)
	})
	return c.earnVal
}

// vipLendingFactory is set by viplending.init() via RegisterVIPLendingFactory.
var vipLendingFactory func(c *Client) any

// RegisterVIPLendingFactory wires the viplending.Client builder. Idempotent.
// Available from v2.5 (KuCoin VIP Lending / OTC loan, read-only queries).
func RegisterVIPLendingFactory(f func(c *Client) any) {
	if vipLendingFactory == nil {
		vipLendingFactory = f
	}
}

// VIPLending returns the *viplending.Client (typed as any). nil when the
// viplending package has not been imported. Available from v2.5. The profile
// lives on the spot host (api.kucoin.com) and exposes the read-only OTC-loan
// queries: collateral (discount-rate) configs, current loan info and the
// participating accounts.
func (c *Client) VIPLending() any {
	c.vipLendingOnce.Do(func() {
		if vipLendingFactory == nil {
			c.logger.Warn(`kucoin.Client.VIPLending: viplending factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/viplending"`)
			return
		}
		c.vipLendingVal = vipLendingFactory(c)
	})
	return c.vipLendingVal
}

// subAccountFactory is set by subaccount.init() via RegisterSubAccountFactory.
var subAccountFactory func(c *Client) any

// RegisterSubAccountFactory wires the subaccount.Client builder. Idempotent.
// Available from v2.5 (master-account sub-account management).
func RegisterSubAccountFactory(f func(c *Client) any) {
	if subAccountFactory == nil {
		subAccountFactory = f
	}
}

// SubAccount returns the *subaccount.Client (typed as any). nil when the
// subaccount package has not been imported. Available from v2.5. The profile
// lives on the spot host (api.kucoin.com) and manages sub-accounts from the
// master account: create + grant margin/futures permission, list summaries and
// balances, and the spot sub-account API-key lifecycle.
func (c *Client) SubAccount() any {
	c.subAccountOnce.Do(func() {
		if subAccountFactory == nil {
			c.logger.Warn(`kucoin.Client.SubAccount: subaccount factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/subaccount"`)
			return
		}
		c.subAccountVal = subAccountFactory(c)
	})
	return c.subAccountVal
}

// convertFactory is set by convert.init() via RegisterConvertFactory.
var convertFactory func(c *Client) any

// RegisterConvertFactory wires the convert.Client builder. Idempotent.
// Available from v2.5 (KuCoin Convert: fee-free currency swaps).
func RegisterConvertFactory(f func(c *Client) any) {
	if convertFactory == nil {
		convertFactory = f
	}
}

// Convert returns the *convert.Client (typed as any). nil when the convert
// package has not been imported. Available from v2.5. The profile lives on the
// spot host (api.kucoin.com) and covers KuCoin Convert: public symbol /
// currency directories, market quotes + orders, and the limit-order lifecycle
// (quote / place / detail / list / cancel).
func (c *Client) Convert() any {
	c.convertOnce.Do(func() {
		if convertFactory == nil {
			c.logger.Warn(`kucoin.Client.Convert: convert factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/convert"`)
			return
		}
		c.convertVal = convertFactory(c)
	})
	return c.convertVal
}

// affiliateFactory is set by affiliate.init() via RegisterAffiliateFactory.
var affiliateFactory func(c *Client) any

// RegisterAffiliateFactory wires the affiliate.Client builder. Idempotent.
// Available from v2.5 (affiliate commission / rebate queries).
func RegisterAffiliateFactory(f func(c *Client) any) {
	if affiliateFactory == nil {
		affiliateFactory = f
	}
}

// Affiliate returns the *affiliate.Client (typed as any). nil when the affiliate
// package has not been imported. Available from v2.5. The profile lives on the
// spot host (api.kucoin.com) and exposes the read-only affiliate reports:
// my-commission and the (deprecated) inviter rebate statistics.
func (c *Client) Affiliate() any {
	c.affiliateOnce.Do(func() {
		if affiliateFactory == nil {
			c.logger.Warn(`kucoin.Client.Affiliate: affiliate factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/affiliate"`)
			return
		}
		c.affiliateVal = affiliateFactory(c)
	})
	return c.affiliateVal
}

// copyTradingFactory is set by copytrading.init() via RegisterCopyTradingFactory.
var copyTradingFactory func(c *Client) any

// RegisterCopyTradingFactory wires the copytrading.Client builder. Idempotent.
// Available from v2.5 (futures copy-trading / lead-trader endpoints).
func RegisterCopyTradingFactory(f func(c *Client) any) {
	if copyTradingFactory == nil {
		copyTradingFactory = f
	}
}

// CopyTrading returns the *copytrading.Client (typed as any). nil when the
// copytrading package has not been imported. Available from v2.5. The profile
// lives on the FUTURES host (api-futures.kucoin.com, shared with the futures
// profile) and exposes the lead-trader futures copy-trading endpoints: order
// placement (+TP/SL), cancellation, max open size and isolated-margin /
// risk-limit management. Requires a lead-trader (copy-trading) account.
func (c *Client) CopyTrading() any {
	c.copyTradingOnce.Do(func() {
		if copyTradingFactory == nil {
			c.logger.Warn(`kucoin.Client.CopyTrading: copytrading factory is not registered; import _ "github.com/tonymontanov/go-kucoin/v2/copytrading"`)
			return
		}
		c.copyTradingVal = copyTradingFactory(c)
	})
	return c.copyTradingVal
}

// Compile-time assertion: *Error implements the error interface.
var _ error = (*kcerr.Error)(nil)
