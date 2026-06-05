/*
FILE: logger.go

DESCRIPTION:
Public re-export of the Logger interface and field constructors. The
underlying types live in internal/kclog so the rest/ws/orderbook
sub-packages can import them without taking a dependency on the root
(which itself depends on these packages — circular import otherwise).

The SDK ships only a NoopLogger by default. Embedders are expected to
adapt their own logger (zerolog, zap, slog, log/slog) to kucoin.Logger
once and pass it via Config.Logger.
*/

package kucoin

import "github.com/tonymontanov/go-kucoin/v2/internal/kclog"

// Logger is the SDK logging facade. See internal/kclog for the full contract.
type Logger = kclog.Logger

// Field is a typed key/value pair used in log entries.
type Field = kclog.Field

// FieldKind enumerates supported field value types.
type FieldKind = kclog.FieldKind

// Field-kind aliases.
const (
	// FieldKindString — string value.
	FieldKindString = kclog.FieldKindString
	// FieldKindInt — int64 value.
	FieldKindInt = kclog.FieldKindInt
	// FieldKindFloat — float64 value.
	FieldKindFloat = kclog.FieldKindFloat
	// FieldKindBool — bool value.
	FieldKindBool = kclog.FieldKindBool
	// FieldKindError — error value.
	FieldKindError = kclog.FieldKindError
)

// Str is a shortcut for a string field.
func Str(key, value string) Field { return kclog.Str(key, value) }

// Int is a shortcut for an int64 field.
func Int(key string, value int64) Field { return kclog.Int(key, value) }

// Float is a shortcut for a float64 field.
func Float(key string, value float64) Field { return kclog.Float(key, value) }

// Bool is a shortcut for a bool field.
func Bool(key string, value bool) Field { return kclog.Bool(key, value) }

// Err is a shortcut for an error field with key "error".
func Err(err error) Field { return kclog.Err(err) }

// NoopLogger returns a Logger that discards every record.
func NoopLogger() Logger { return kclog.Noop() }
