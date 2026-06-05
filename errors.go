/*
FILE: errors.go

DESCRIPTION:
Public re-export of the SDK error type and category predicates from
internal/kcerr. Importers work through the root package only:

	import kucoin "github.com/tonymontanov/go-kucoin/v2"

	if kucoin.IsRateLimit(err) { ... }

The aliases below preserve the typed identity (`type X = internal.X`),
so users can also do `errors.As(err, &kucoin.Error{})`.
*/

package kucoin

import "github.com/tonymontanov/go-kucoin/v2/internal/kcerr"

// Error is the SDK error type (alias). All SDK methods return *Error
// (sometimes wrapped). errors.As / errors.Is work normally.
type Error = kcerr.Error

// ErrorKind is the error category enum (alias).
type ErrorKind = kcerr.ErrorKind

// Error categories. See internal/kcerr for full semantics of each kind.
const (
	// ErrorKindUnknown — the SDK could not classify the failure.
	ErrorKindUnknown = kcerr.ErrorKindUnknown
	// ErrorKindNetwork — transport-level failure (timeout, conn reset, ...).
	ErrorKindNetwork = kcerr.ErrorKindNetwork
	// ErrorKindRateLimit — KuCoin told us we hit a rate limit.
	ErrorKindRateLimit = kcerr.ErrorKindRateLimit
	// ErrorKindAuth — credentials missing/invalid or signature rejected.
	ErrorKindAuth = kcerr.ErrorKindAuth
	// ErrorKindInvalidRequest — malformed request, validation rejection.
	ErrorKindInvalidRequest = kcerr.ErrorKindInvalidRequest
	// ErrorKindExchange — exchange rejected the request for business reasons.
	ErrorKindExchange = kcerr.ErrorKindExchange
)

// NewError constructs an *Error. Mostly used by SDK internals; user code
// rarely needs this.
func NewError(kind ErrorKind, code, msg string, cause error) *Error {
	return kcerr.New(kind, code, msg, cause)
}

// IsNetwork reports whether err is a network-class error.
func IsNetwork(err error) bool { return kcerr.IsNetwork(err) }

// IsRateLimit reports whether err is a rate-limit error.
func IsRateLimit(err error) bool { return kcerr.IsRateLimit(err) }

// IsAuth reports whether err is an auth/permission error.
func IsAuth(err error) bool { return kcerr.IsAuth(err) }

// IsInvalidRequest reports whether err is a validation/build-time error.
func IsInvalidRequest(err error) bool { return kcerr.IsInvalidRequest(err) }

// IsExchange reports whether err is an exchange-level rejection.
func IsExchange(err error) bool { return kcerr.IsExchange(err) }

// MapKucoinCode returns the SDK ErrorKind for a KuCoin code (string).
func MapKucoinCode(code, msg string) ErrorKind { return kcerr.MapKucoinCode(code, msg) }

// MapHTTPStatus returns the SDK ErrorKind for an HTTP status code.
func MapHTTPStatus(status int) ErrorKind { return kcerr.MapHTTPStatus(status) }
