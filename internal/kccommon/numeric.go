/*
FILE: internal/kccommon/numeric.go

DESCRIPTION:
Profile-agnostic numeric parsing helpers for the KuCoin wire format.

Unlike Bitget (everything quoted), KuCoin is INCONSISTENT: order-book
levels and many futures fields arrive as JSON numbers, while order
lifecycle fields and some account fields are quoted strings. These
string-only helpers cover the quoted fields and the comma-delimited
level2 "change" payload; numeric JSON fields are decoded directly into
decimal.Decimal / int64 struct fields by the codec.

All helpers are pure functions, hot-path safe, and treat the empty
string as zero (KuCoin occasionally emits "" for inapplicable fields).

DECIMAL VS FLOAT:
Prices/sizes are parsed via decimal.NewFromString exclusively — never
float — to avoid precision loss on values like 0.000123456.
*/

package kccommon

import (
	"strconv"

	"github.com/shopspring/decimal"
)

// ParseDecimalOrZero is a forgiving counterpart to decimal.NewFromString —
// empty strings are treated as zero.
func ParseDecimalOrZero(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

// ParseInt64OrZero parses an integer field shipped as a string. Empty
// string → 0.
func ParseInt64OrZero(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// ParseIntOrZero is the int counterpart of ParseInt64OrZero. Used for
// small-range fields (precision digits, leverage, ...).
func ParseIntOrZero(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	var v int64
	var err error
	v, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
