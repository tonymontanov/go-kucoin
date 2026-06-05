/*
FILE: internal/codec/codec.go

DESCRIPTION:
Thin wrapper around json-iterator/go used everywhere on the SDK hot-path
(WS message dispatch, REST envelope decoding). The standard `encoding/json`
package is intentionally not used directly — jsoniter is 2-3x faster and
allocates less, which matters for the thousands of WS messages/sec a busy
KuCoin Futures market generates.

Also exposes small numeric helpers used across REST/WS parsers:

  - ParseDecimal: returns shopspring/decimal.Decimal from a string. Empty
    string maps to decimal.Zero. Returns error on a malformed value.
  - ParseInt64:   returns int64 from a string. Empty string maps to 0.
  - ParseFloat64: returns float64 from a string. Empty string maps to 0.

Unlike Bitget, KuCoin ships numbers as a MIX of JSON numbers and strings:
order-book levels and futures account fields arrive as JSON numbers, while
some endpoints quote them. The decimal helpers in internal/kccommon cope
with either form; these string-only helpers cover the quoted fields.
*/

package codec

import (
	"bytes"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/shopspring/decimal"
)

// RawJSON is a json.RawMessage equivalent that works correctly with
// jsoniter. Use it inside structs whose fields are forwarded to a
// secondary decoder (typed per topic / per endpoint).
type RawJSON []byte

// MarshalJSON implements json.Marshaler.
func (m RawJSON) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("null"), nil
	}
	return []byte(m), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *RawJSON) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}

// IsNull reports whether the raw payload is empty or literally "null".
func (m RawJSON) IsNull() bool {
	return len(m) == 0 || bytes.Equal(m, []byte("null"))
}

// jsonAPI is the configured jsoniter instance.
// ConfigCompatibleWithStandardLibrary matches encoding/json behaviour
// (case-insensitive field matching disabled, RFC-compliant numerics).
// Same choice as in go-bybit / go-okx / go-bitget.
var jsonAPI = jsoniter.ConfigCompatibleWithStandardLibrary

// Marshal serializes v to JSON.
func Marshal(v any) ([]byte, error) {
	return jsonAPI.Marshal(v)
}

// Unmarshal parses raw into dest.
func Unmarshal(raw []byte, dest any) error {
	return jsonAPI.Unmarshal(raw, dest)
}

// ParseDecimal converts a numeric string into a decimal.Decimal.
// Empty input → decimal.Zero, no error.
func ParseDecimal(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

// ParseInt64 converts a numeric string to int64. Empty input → 0.
func ParseInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// ParseFloat64 converts a numeric string to float64. Empty input → 0.
// Used only at boundaries where downstream code requires float64; inside
// the SDK prefer decimal.Decimal everywhere.
func ParseFloat64(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
