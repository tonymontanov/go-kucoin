package ws

import (
	"testing"
	"time"
)

// A connection that lived healthyConnectionSpan or longer resets the
// reconnect backoff; a short-lived one keeps climbing the ladder.
func TestConnectionWasHealthy(t *testing.T) {
	if connectionWasHealthy(healthyConnectionSpan - time.Millisecond) {
		t.Fatal("a connection shorter than the healthy span reset the backoff")
	}
	if !connectionWasHealthy(healthyConnectionSpan) {
		t.Fatal("a connection that lived the healthy span did not reset the backoff")
	}
}

// The configured ping interval caps the server one from the bullet response;
// either side may be unset.
func TestEffectivePingInterval(t *testing.T) {
	var cases = []struct {
		server, configured, want time.Duration
	}{
		{18 * time.Second, 5 * time.Second, 5 * time.Second},
		{18 * time.Second, 30 * time.Second, 18 * time.Second},
		{18 * time.Second, 0, 18 * time.Second},
		{0, 15 * time.Second, 15 * time.Second},
	}
	for _, c := range cases {
		if got := effectivePingInterval(c.server, c.configured); got != c.want {
			t.Errorf("effectivePingInterval(%v, %v) = %v, want %v", c.server, c.configured, got, c.want)
		}
	}
}
