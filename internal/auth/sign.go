/*
FILE: internal/auth/sign.go

DESCRIPTION:
Request signing for the KuCoin CLASSIC API (NOT the new UTA / unified
account family). KuCoin authenticates every private REST request with a
set of headers and an HMAC-SHA256 signature:

	preHash   = timestamp + method + requestPath + body
	signature = base64( HMAC_SHA256(secretKey, preHash) )

	- timestamp:   current Unix time in MILLISECONDS, as a decimal string.
	- method:      HTTP method in UPPER case ("GET" / "POST" / ...).
	- requestPath: the exact path that goes on the wire, INCLUDING the
	               leading "/api/..." and the canonical (un-encoded) query
	               string for GET. KuCoin signs the path that is sent.
	- body:        the exact JSON body string on the wire; empty for GET.
	               Signing MUST happen on the same byte sequence that is
	               sent — re-marshalling can reorder map keys and break the
	               signature, so SignREST takes the already-rendered body.

Headers emitted:

	KC-API-KEY         — apiKey
	KC-API-SIGN        — base64(signature)
	KC-API-TIMESTAMP   — ms timestamp string
	KC-API-PASSPHRASE  — see below (version-dependent)
	KC-API-KEY-VERSION — "1" | "2" | "3"
	Content-Type       — application/json

PASSPHRASE ENCODING (KEY VERSION):
KuCoin API keys carry a version that selects how KC-API-PASSPHRASE is
encoded:

  - version 1: the passphrase is sent in PLAINTEXT (legacy keys).
  - version 2 / 3: KC-API-PASSPHRASE = base64(HMAC_SHA256(secret, passphrase)).
    Versions 2 and 3 share the same passphrase-encryption and request-
    signing scheme; the SDK treats them identically and only echoes the
    configured version in the KC-API-KEY-VERSION header.

The SDK defaults to version "2" (the most widely-deployed classic-key
version); embedders running version-1 or version-3 keys set KeyVersion
on the public Config.

WEBSOCKET:
KuCoin WS does NOT use this signature directly. A WS connection is opened
with a "bullet" token obtained from a REST POST to /api/v1/bullet-public
(public) or /api/v1/bullet-private (private). The private bullet call is
itself a signed REST request, so it flows through SignREST like any other
private endpoint — there is no separate WS signing step.

SECURITY:
  - Secret material is stored as []byte and never serialized. String()
    redacts the API key and passphrase.
  - Pre-hash and body strings MUST NOT be logged.
*/

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrSignerDisabled is returned when Sign* is called on a signer that has
// no credentials. The same Signer can serve public REST/WS endpoints
// without keys; private endpoints surface this error at call time.
var ErrSignerDisabled = errors.New("auth: signer is disabled (api key/secret/passphrase not configured)")

// KeyVersion is the KuCoin API key version, selecting the passphrase
// encoding scheme. Sent verbatim in the KC-API-KEY-VERSION header.
type KeyVersion string

const (
	// KeyVersionV1 — legacy keys; passphrase sent in plaintext.
	KeyVersionV1 KeyVersion = "1"
	// KeyVersionV2 — passphrase encrypted with HMAC-SHA256. SDK default.
	KeyVersionV2 KeyVersion = "2"
	// KeyVersionV3 — same scheme as V2; only the header value differs.
	KeyVersionV3 KeyVersion = "3"
)

// Signer holds KuCoin credentials and produces conformant signatures.
// Safe for concurrent use: all fields are read-only after construction.
type Signer struct {
	apiKey     string
	secretKey  []byte
	passphrase string
	keyVersion KeyVersion
	// encodedPassphrase is precomputed at construction time: for V2/V3 it
	// is base64(HMAC-SHA256(secret, passphrase)); for V1 it equals the
	// raw passphrase. Cached so the hot path does not re-HMAC on every call.
	encodedPassphrase string
	enabled           bool
}

// NewSigner creates a Signer. If any of apiKey / secretKey / passphrase is
// empty the signer is disabled and Sign* will return ErrSignerDisabled.
// This lets a single Client hit public endpoints without credentials.
//
// keyVersion selects the passphrase-encoding scheme; an empty value
// defaults to KeyVersionV2.
func NewSigner(apiKey, secretKey, passphrase string, keyVersion KeyVersion) *Signer {
	if keyVersion == "" {
		keyVersion = KeyVersionV2
	}
	var enabled bool = apiKey != "" && secretKey != "" && passphrase != ""
	var s *Signer = &Signer{
		apiKey:     apiKey,
		secretKey:  []byte(secretKey),
		passphrase: passphrase,
		keyVersion: keyVersion,
		enabled:    enabled,
	}
	s.encodedPassphrase = s.computeEncodedPassphrase()
	return s
}

// Enabled reports whether the signer has credentials.
func (s *Signer) Enabled() bool { return s != nil && s.enabled }

// APIKey returns the bound API key, used to populate KC-API-KEY.
func (s *Signer) APIKey() string {
	if s == nil {
		return ""
	}
	return s.apiKey
}

// KeyVersion returns the configured key version string for the
// KC-API-KEY-VERSION header.
func (s *Signer) KeyVersion() string {
	if s == nil {
		return string(KeyVersionV2)
	}
	return string(s.keyVersion)
}

// EncodedPassphrase returns the value for the KC-API-PASSPHRASE header:
// the HMAC-encrypted passphrase for V2/V3 keys, or the plaintext
// passphrase for V1 keys.
func (s *Signer) EncodedPassphrase() string {
	if s == nil {
		return ""
	}
	return s.encodedPassphrase
}

// MillisTimestamp returns now in milliseconds as a decimal string. Used for
// KC-API-TIMESTAMP and as the timestamp component of the REST pre-hash.
// If now is zero, time.Now() is used.
func (s *Signer) MillisTimestamp(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return strconv.FormatInt(now.UnixMilli(), 10)
}

/*
SignREST returns the base64 HMAC-SHA256 signature for a KuCoin REST
request:

	preHash = timestamp + method + requestPath + body

Parameters:
  - timestamp:   ms timestamp string (use MillisTimestamp).
  - method:      HTTP method in UPPER case ("GET" / "POST" / ...).
  - requestPath: full URL path including the leading "/" and the canonical
    query string (e.g. "/api/v1/orders?symbol=XBTUSDTM").
  - body:        for POST — the exact JSON body string; empty for GET.

Returns ErrSignerDisabled if the signer has no credentials.
*/
func (s *Signer) SignREST(timestamp, method, requestPath, body string) (string, error) {
	if !s.Enabled() {
		return "", ErrSignerDisabled
	}
	var sb strings.Builder
	sb.Grow(len(timestamp) + len(method) + len(requestPath) + len(body))
	sb.WriteString(timestamp)
	sb.WriteString(method)
	sb.WriteString(requestPath)
	sb.WriteString(body)
	return s.hmacBase64(sb.String()), nil
}

// computeEncodedPassphrase produces the KC-API-PASSPHRASE value once at
// construction. V2/V3: base64(HMAC-SHA256(secret, passphrase)); V1: raw.
func (s *Signer) computeEncodedPassphrase() string {
	if !s.enabled {
		return ""
	}
	if s.keyVersion == KeyVersionV1 {
		return s.passphrase
	}
	return s.hmacBase64(s.passphrase)
}

// hmacBase64 computes base64(HMAC_SHA256(secret, msg)).
func (s *Signer) hmacBase64(msg string) string {
	var mac = hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(msg))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// String returns a log-safe representation that NEVER includes the secret
// or passphrase.
func (s *Signer) String() string {
	if s == nil || !s.enabled {
		return "auth.Signer{disabled}"
	}
	return "auth.Signer{enabled, keyVersion=" + string(s.keyVersion) + ", apiKey=" + redact(s.apiKey) + "}"
}

// redact turns a string into "abcd…wxyz" — first/last 4 chars. For logs only.
func redact(v string) string {
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "…" + v[len(v)-4:]
}
