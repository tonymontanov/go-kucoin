/*
FILE: config.go

DESCRIPTION:
Public SDK configuration — REST endpoint, credentials + key version,
transport timeouts, WS reconnect policy, orderbook tuning, observer
hooks. Default values target the production KuCoin CLASSIC Futures API
(NOT the new UTA / unified account family) and conservative HFT-friendly
timeouts.

ENDPOINTS (defaults, v1.0 — Futures):

	REST (prod):     https://api-futures.kucoin.com
	REST (sandbox):  https://api-sandbox-futures.kucoin.com   (Demo == true)

KuCoin splits REST by section across DIFFERENT hosts (Futures on
api-futures.kucoin.com, Spot on api.kucoin.com). v1.0 ships Futures only,
so the default REST.BaseURL is the futures host. When Spot lands in v2.0
the spot profile builds its own REST client against the spot host — the
root transport stays section-agnostic.

WEBSOCKET (bullet-token model):
KuCoin WS has NO fixed, pre-known URL. A connection is opened with a
"bullet" token obtained from a REST POST to one of:

	BulletPublicPath   /api/v1/bullet-public    (public market data)
	BulletPrivatePath  /api/v1/bullet-private   (private, signed REST)

The bullet response returns the actual WS endpoint(s) and the server's
ping interval. Therefore WsConfig carries only transport/reconnect tuning
— never URLs.

DEMO / SANDBOX:
KuCoin runs a separate sandbox host for Futures. Setting Demo == true
selects DefaultFuturesSandboxRestBaseURL automatically (unless REST.BaseURL
is set explicitly). Bullet tokens — and thus WS — follow the same host.
*/

package kucoin

import (
	"time"

	"github.com/tonymontanov/go-kucoin/v2/internal/auth"
)

// KuCoin endpoints. Declared as vars so tests can override them
// (e.g. point at a mock server).
var (
	// DefaultFuturesRestBaseURL — production Futures REST endpoint.
	DefaultFuturesRestBaseURL string = "https://api-futures.kucoin.com"

	// DefaultFuturesSandboxRestBaseURL — sandbox Futures REST endpoint,
	// selected automatically when Config.Demo is true.
	DefaultFuturesSandboxRestBaseURL string = "https://api-sandbox-futures.kucoin.com"

	// DefaultSpotRestBaseURL — production Spot REST endpoint (v2.0). KuCoin
	// serves Spot on a DIFFERENT host than Futures; the spot profile builds
	// its own REST client against this host (see spot.NewClient). The root
	// transport stays section-agnostic and defaults to the futures host.
	DefaultSpotRestBaseURL string = "https://api.kucoin.com"

	// DefaultSpotSandboxRestBaseURL — historical Spot sandbox host. KuCoin
	// has deprecated the public spot sandbox; kept for completeness and
	// selected when Config.Demo is true, but most accounts must test Spot
	// against production with small sizes.
	DefaultSpotSandboxRestBaseURL string = "https://openapi-sandbox.kucoin.com"
)

// Bullet-token REST paths. The WS layer POSTs to one of these to obtain a
// connection token and the live endpoint list. Exposed for discoverability
// and contract tests; user code does not call them directly.
const (
	// BulletPublicPath — public bullet token (no auth). Market-data WS.
	BulletPublicPath string = "/api/v1/bullet-public"
	// BulletPrivatePath — private bullet token (signed REST). Order/account WS.
	BulletPrivatePath string = "/api/v1/bullet-private"
)

// KeyVersion is the KuCoin API key version, selecting the KC-API-PASSPHRASE
// encoding scheme. Re-exported from internal/auth so callers configure it
// through the root package only.
type KeyVersion = auth.KeyVersion

// Key-version aliases. Default is KeyVersionV2 (most widely-deployed
// classic key). V1 keys send the passphrase in plaintext; V2/V3 HMAC-encode it.
const (
	// KeyVersionV1 — legacy keys; passphrase sent in plaintext.
	KeyVersionV1 = auth.KeyVersionV1
	// KeyVersionV2 — passphrase HMAC-SHA256 encoded. SDK default.
	KeyVersionV2 = auth.KeyVersionV2
	// KeyVersionV3 — same scheme as V2; only the header value differs.
	KeyVersionV3 = auth.KeyVersionV3
)

// Config — public SDK configuration. Pass to NewClient.
type Config struct {
	// APIKey — KuCoin public API key. Required for signed endpoints; safe
	// to leave empty for public-only access.
	APIKey string
	// SecretKey — KuCoin secret used to compute KC-API-SIGN.
	SecretKey string
	// Passphrase — KuCoin API passphrase (set when the key was created).
	// Required by every signed call alongside the signature.
	Passphrase string
	// KeyVersion — API key version selecting the passphrase encoding.
	// Empty defaults to KeyVersionV2.
	KeyVersion KeyVersion

	// REST — REST transport settings. Empty fields fall back to defaults.
	REST RestConfig
	// WS — WebSocket transport settings. Empty fields fall back to defaults.
	WS WsConfig
	// Orderbook — orderbook engine settings. Empty fields fall back to
	// defaults.
	Orderbook OrderbookConfig

	// Logger — optional logger. NoopLogger if nil.
	Logger Logger
	// Metrics — optional counter factory. NoopMetrics if nil.
	Metrics CounterFactory

	// UserAgent — User-Agent value sent on REST requests. Default
	// "go-kucoin/2".
	UserAgent string

	// Demo — when true, the SDK targets the KuCoin SANDBOX Futures host
	// (DefaultFuturesSandboxRestBaseURL) unless REST.BaseURL is set
	// explicitly. Sandbox uses a dedicated set of API keys created in the
	// sandbox web UI. WS follows the same host via the sandbox bullet
	// token.
	Demo bool

	// RateLimitObserver — legacy observer (endpoint, headers). Kept for
	// source-level back-compat with the OKX-style pattern. nil → no-op.
	RateLimitObserver func(endpoint string, headers map[string]string)

	// RateLimitEventObserver — primary observer. Receives the full
	// RateLimitEvent with OrderCount/Symbols/Category/Headers.
	//
	// Speed contract: called synchronously from the goroutine that issued
	// the REST call. Implementations must be O(1) (typically a
	// non-blocking send to a buffered channel).
	//
	// nil → no-op.
	RateLimitEventObserver func(RateLimitEvent)
}

// RestConfig — REST transport parameters.
type RestConfig struct {
	// BaseURL — REST host. Default DefaultFuturesRestBaseURL (or the
	// sandbox host when Config.Demo is true).
	BaseURL string
	// RequestTimeout — global timeout for one REST call. Default 10s.
	// A ctx with its own deadline overrides this for a single request.
	RequestTimeout time.Duration
	// MaxIdleConns — http.Transport pool size. Default 100.
	MaxIdleConns int
	// MaxIdleConnsPerHost — per-host pool size. Default 100.
	MaxIdleConnsPerHost int
	// IdleConnTimeout — keep-alive idle timeout. Default 90s.
	IdleConnTimeout time.Duration
}

// WsConfig — WebSocket transport parameters. KuCoin WS uses the bullet
// token model, so there are NO endpoint URLs here — the endpoint and the
// server's ping interval come from the bullet REST response.
type WsConfig struct {
	// HandshakeTimeout — TLS+HTTP upgrade timeout. Default 10s.
	HandshakeTimeout time.Duration
	// ReadTimeout — read deadline. Default 35s. KuCoin's server pings on a
	// schedule reported in the bullet response (typically 18s); 35s leaves
	// a full cycle of slack.
	ReadTimeout time.Duration
	// WriteTimeout — write deadline. Default 5s.
	WriteTimeout time.Duration
	// PingInterval — client→server ping interval. Default 15s and used as
	// a FALLBACK only: the bullet response carries the authoritative
	// pingInterval, which the WS layer honours when present.
	PingInterval time.Duration

	// ReconnectInitialBackoff — first sleep after a connection failure.
	// Default 200ms.
	ReconnectInitialBackoff time.Duration
	// ReconnectMaxBackoff — backoff cap. Default 10s.
	ReconnectMaxBackoff time.Duration
	// ReconnectJitter — relative jitter [0..1] applied to backoff.
	// Default 0.2.
	ReconnectJitter float64

	// ReadBufferSize / WriteBufferSize — gorilla/websocket buffer sizes.
	// Defaults: 64KB / 16KB.
	ReadBufferSize  int
	WriteBufferSize int
}

// OrderbookConfig — orderbook engine parameters. Used by the sequence-based
// engine (added in a later milestone); settings are exposed now so the
// public surface is stable from the start.
type OrderbookConfig struct {
	// MaxDepth — depth of the local order book per side. Default 200.
	MaxDepth int
}

// DefaultConfig returns a Config pre-populated with production endpoints
// and HFT-friendly timeouts. Callers can override individual fields and
// pass the result to NewClient — empty sub-fields fall back to these
// defaults.
func DefaultConfig() Config {
	return Config{
		KeyVersion: KeyVersionV2,
		REST: RestConfig{
			BaseURL:             DefaultFuturesRestBaseURL,
			RequestTimeout:      10 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
		WS: WsConfig{
			HandshakeTimeout:        10 * time.Second,
			ReadTimeout:             35 * time.Second,
			WriteTimeout:            5 * time.Second,
			PingInterval:            15 * time.Second,
			ReconnectInitialBackoff: 200 * time.Millisecond,
			ReconnectMaxBackoff:     10 * time.Second,
			ReconnectJitter:         0.2,
			ReadBufferSize:          64 * 1024,
			WriteBufferSize:         16 * 1024,
		},
		Orderbook: OrderbookConfig{
			MaxDepth: 200,
		},
		Logger:    NoopLogger(),
		Metrics:   NoopMetrics(),
		UserAgent: "go-kucoin/2",
	}
}

// withDefaults returns a copy of c with empty fields filled from
// DefaultConfig. Already-set explicit URLs/values are preserved.
func (c Config) withDefaults() Config {
	var def Config = DefaultConfig()

	if c.KeyVersion == "" {
		c.KeyVersion = def.KeyVersion
	}

	// REST. BaseURL gets special handling: an unset URL resolves to the
	// sandbox host when Demo is requested, otherwise the production host.
	if c.REST.BaseURL == "" {
		if c.Demo {
			c.REST.BaseURL = DefaultFuturesSandboxRestBaseURL
		} else {
			c.REST.BaseURL = def.REST.BaseURL
		}
	}
	if c.REST.RequestTimeout == 0 {
		c.REST.RequestTimeout = def.REST.RequestTimeout
	}
	if c.REST.MaxIdleConns == 0 {
		c.REST.MaxIdleConns = def.REST.MaxIdleConns
	}
	if c.REST.MaxIdleConnsPerHost == 0 {
		c.REST.MaxIdleConnsPerHost = def.REST.MaxIdleConnsPerHost
	}
	if c.REST.IdleConnTimeout == 0 {
		c.REST.IdleConnTimeout = def.REST.IdleConnTimeout
	}

	// WS.
	if c.WS.HandshakeTimeout == 0 {
		c.WS.HandshakeTimeout = def.WS.HandshakeTimeout
	}
	if c.WS.ReadTimeout == 0 {
		c.WS.ReadTimeout = def.WS.ReadTimeout
	}
	if c.WS.WriteTimeout == 0 {
		c.WS.WriteTimeout = def.WS.WriteTimeout
	}
	if c.WS.PingInterval == 0 {
		c.WS.PingInterval = def.WS.PingInterval
	}
	if c.WS.ReconnectInitialBackoff == 0 {
		c.WS.ReconnectInitialBackoff = def.WS.ReconnectInitialBackoff
	}
	if c.WS.ReconnectMaxBackoff == 0 {
		c.WS.ReconnectMaxBackoff = def.WS.ReconnectMaxBackoff
	}
	if c.WS.ReconnectJitter == 0 {
		c.WS.ReconnectJitter = def.WS.ReconnectJitter
	}
	if c.WS.ReadBufferSize == 0 {
		c.WS.ReadBufferSize = def.WS.ReadBufferSize
	}
	if c.WS.WriteBufferSize == 0 {
		c.WS.WriteBufferSize = def.WS.WriteBufferSize
	}

	if c.Orderbook.MaxDepth == 0 {
		c.Orderbook.MaxDepth = def.Orderbook.MaxDepth
	}

	if c.Logger == nil {
		c.Logger = NoopLogger()
	}
	if c.Metrics == nil {
		c.Metrics = NoopMetrics()
	}
	if c.UserAgent == "" {
		c.UserAgent = def.UserAgent
	}

	return c
}

// validate ensures the minimal set of required fields is present.
// Credentials are NOT enforced here — public endpoints work without keys
// and the signer surfaces auth.ErrSignerDisabled at call time.
func (c Config) validate() error {
	if c.REST.BaseURL == "" {
		return NewError(ErrorKindInvalidRequest, "", "config: REST.BaseURL is empty", nil)
	}
	return nil
}
