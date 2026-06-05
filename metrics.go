/*
FILE: metrics.go

DESCRIPTION:
Public re-export of the metrics interfaces defined in internal/kcmet.
The SDK emits only monotonic counters; histograms/gauges are out of
scope (embedders that want timing distributions should wrap user-facing
calls themselves).

STABLE COUNTER NAMES (the SDK contract — see README):

	kucoin_ws_messages_received_total
	kucoin_ws_messages_dropped_total
	kucoin_ws_reconnects_total
	kucoin_ws_subscriptions_total
	kucoin_ws_ping_failed_total
	kucoin_ws_token_failed_total
	kucoin_ws_resyncs_total
*/

package kucoin

import "github.com/tonymontanov/go-kucoin/v2/internal/kcmet"

// Counter is a monotonically increasing metric.
type Counter = kcmet.Counter

// CounterFactory creates named counters. Implementations may attach common
// labels at construction time.
type CounterFactory = kcmet.CounterFactory

// NoopMetrics returns a CounterFactory that discards every increment.
func NoopMetrics() CounterFactory { return kcmet.Noop() }
