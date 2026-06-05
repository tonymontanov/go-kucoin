/*
FILE: internal/kcerr/codes.go

DESCRIPTION:
Mapping from raw transport-level signals (HTTP status, KuCoin code) to
ErrorKind. Centralised here so the rest of the SDK never sprinkles
"if code == ..." chains.

SOURCES:
  - HTTP status mapping is the standard 4xx/5xx convention, with 401/403 as
    Auth and 429 as RateLimit.
  - KuCoin business-code tables are derived from the public docs:
        https://www.kucoin.com/docs-new/rest/  (per-endpoint error tables)
        https://www.kucoin.com/docs-new/authentication
    The list below is intentionally NON-exhaustive — only codes the SDK
    can usefully classify into a Kind. Anything not explicitly listed
    falls back to ErrorKindExchange (so the caller still gets an Exchange-
    class error, just without the SDK pre-classifying it).

DESIGN PRINCIPLES:
  - Keep mappings stable: this is a public contract via IsAuth /
    IsRateLimit / IsInvalidRequest / IsNetwork / IsExchange.
  - When a code is ambiguous between SDK Kind buckets (e.g. "balance
    insufficient" — Exchange business rejection, NOT a build-time
    validation error), prefer ErrorKindExchange.

UPDATE PROCEDURE:
When KuCoin publishes a new error code that the SDK should react to:
  1. Add a `case` to MapKucoinCode below, with the doc-string as a comment.
  2. Add a covering test in codes_test.go (one row per bucket).
  3. If a code's Kind changes for an already-listed code, bump the SDK
     minor version — IsRateLimit/IsAuth callers may rely on it.
*/

package kcerr

// CodeOK is the success code KuCoin returns on every 2xx response.
// Anything else is treated as an error by the REST client.
const CodeOK = "200000"

// MapHTTPStatus maps an HTTP status code to an ErrorKind.
//
// 2xx is a success and is not expected to be passed here.
// 401/403 → Auth (key/IP/permission).
// 429     → RateLimit.
// 4xx     → InvalidRequest (the SDK or caller built a bad request).
// 5xx     → Network (transient at the network/exchange edge — retryable).
// other   → Unknown.
func MapHTTPStatus(status int) ErrorKind {
	switch {
	case status == 401, status == 403:
		return ErrorKindAuth
	case status == 429:
		return ErrorKindRateLimit
	case status >= 400 && status < 500:
		return ErrorKindInvalidRequest
	case status >= 500 && status < 600:
		return ErrorKindNetwork
	default:
		return ErrorKindUnknown
	}
}

// MapKucoinCode maps a KuCoin business code to an ErrorKind. msg is
// currently unused but kept in the signature so future heuristics (e.g.
// parsing "Too many requests" out of msg) do not break callers.
//
// KuCoin returns the code as a string in JSON ("200000" / "400005" / ...).
//
// Code families:
//
//   - 200xxx — success (200000) + a few balance/state codes.
//   - 400xxx — auth, signature, IP, passphrase, parameter formatting.
//   - 404xxx — URL not found.
//   - 411xxx — account frozen / KYC.
//   - 415xxx — content-type / media.
//   - 429xxx — rate limit.
//   - 300xxx — futures order lifecycle / balance.
//   - 330xxx — futures risk / leverage / position.
//   - 500xxx — server-side transient.
//
// Anything outside the explicitly listed codes maps to ErrorKindExchange.
//
//nolint:gocyclo,funlen // table-driven mapping with one case per code.
func MapKucoinCode(code, _ string) ErrorKind {
	switch code {
	// ----- success -----
	case CodeOK:
		return ErrorKindUnknown // success — must not be passed here, but be safe

	// ----- 400xxx auth / signature / params -----
	case "400001":
		// Any of KC-API-KEY, KC-API-SIGN, KC-API-TIMESTAMP,
		// KC-API-PASSPHRASE is missing in the request header.
		return ErrorKindAuth
	case "400002":
		// KC-API-TIMESTAMP is invalid (expired / clock skew).
		return ErrorKindAuth
	case "400003":
		// KC-API-KEY does not exist.
		return ErrorKindAuth
	case "400004":
		// KC-API-PASSPHRASE error.
		return ErrorKindAuth
	case "400005":
		// Signature error (KC-API-SIGN mismatch).
		return ErrorKindAuth
	case "400006":
		// The requesting IP address is not in the API key whitelist.
		return ErrorKindAuth
	case "400007":
		// Access denied — API key lacks the required permission.
		return ErrorKindAuth
	case "400100":
		// Parameter error (generic validation).
		return ErrorKindInvalidRequest
	case "400500":
		// Trading suspended for the symbol — transient.
		return ErrorKindNetwork
	case "400600", "400760":
		// Symbol/contract not available / not supported.
		return ErrorKindInvalidRequest

	// ----- 404xxx not found -----
	case "404000":
		// URL not found.
		return ErrorKindInvalidRequest

	// ----- 411xxx / 415xxx account / media -----
	case "411100":
		// User is frozen.
		return ErrorKindAuth
	case "415000":
		// Unsupported Media Type — Content-Type must be application/json.
		return ErrorKindInvalidRequest

	// ----- 429xxx rate limit -----
	case "429000":
		// Too many requests — back off.
		return ErrorKindRateLimit

	// ----- 100xxx system / sign (futures) -----
	case "100001":
		// Signature/auth error on some futures endpoints.
		return ErrorKindAuth
	case "100002":
		// Parameter validation error on some futures endpoints.
		return ErrorKindInvalidRequest

	// ----- 200xxx balance / state -----
	case "200002":
		// Order does not exist.
		return ErrorKindInvalidRequest
	case "200004":
		// Balance insufficient — exchange business rejection.
		return ErrorKindExchange

	// ----- 300xxx futures order lifecycle / balance -----
	case "300000":
		// The order parameters are invalid / order failed (generic).
		return ErrorKindInvalidRequest
	case "300003":
		// Balance insufficient (futures) — exchange business rejection.
		return ErrorKindExchange
	case "300008":
		// System busy / matching engine unavailable — transient.
		return ErrorKindNetwork
	case "300012":
		// The contract is being settled / closed — transient.
		return ErrorKindNetwork

	// ----- 330xxx futures risk / leverage / position -----
	case "330005":
		// Leverage cannot exceed the maximum for this contract.
		return ErrorKindInvalidRequest
	case "330011":
		// Risk limit exceeded / position cap.
		return ErrorKindInvalidRequest

	// ----- 500xxx server-side -----
	case "500000":
		// Internal server error — transient, retryable.
		return ErrorKindNetwork

	default:
		return ErrorKindExchange
	}
}
