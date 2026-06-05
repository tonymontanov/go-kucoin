/*
FILE: internal/ws/protocol_test.go

DESCRIPTION:
Unit tests for the KuCoin WS wire types: flexCode (string/number forms),
Envelope classification (push vs control), and outboundOp marshalling
(omitempty on ping frames).
*/

package ws

import (
	"testing"

	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
)

func TestFlexCode_StringAndNumber(t *testing.T) {
	var cases = []struct {
		name string
		raw  string
		want string
	}{
		{"string", `{"code":"404"}`, "404"},
		{"number", `{"code":404}`, "404"},
		{"null", `{"code":null}`, ""},
		{"absent", `{}`, ""},
	}
	var i int
	for i = 0; i < len(cases); i++ {
		var tc = cases[i]
		t.Run(tc.name, func(t *testing.T) {
			var env Envelope
			if err := codec.Unmarshal([]byte(tc.raw), &env); err != nil {
				t.Fatalf("unmarshal %q: %v", tc.raw, err)
			}
			if env.Code.String() != tc.want {
				t.Fatalf("code = %q, want %q", env.Code.String(), tc.want)
			}
		})
	}
}

func TestEnvelope_Classification(t *testing.T) {
	var cases = []struct {
		name      string
		raw       string
		isPush    bool
		isControl bool
	}{
		{"push", `{"type":"message","topic":"/contractMarket/level2:XBTUSDTM","subject":"level2","data":{}}`, true, false},
		{"welcome", `{"id":"c1","type":"welcome"}`, false, true},
		{"ack", `{"id":"s1","type":"ack"}`, false, true},
		{"pong", `{"id":"p1","type":"pong"}`, false, true},
		{"error", `{"id":"e1","type":"error","code":404}`, false, true},
		{"message_without_topic", `{"type":"message"}`, false, false},
	}
	var i int
	for i = 0; i < len(cases); i++ {
		var tc = cases[i]
		t.Run(tc.name, func(t *testing.T) {
			var env Envelope
			if err := codec.Unmarshal([]byte(tc.raw), &env); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if env.IsPush() != tc.isPush {
				t.Errorf("IsPush() = %v, want %v", env.IsPush(), tc.isPush)
			}
			if env.IsControl() != tc.isControl {
				t.Errorf("IsControl() = %v, want %v", env.IsControl(), tc.isControl)
			}
		})
	}
}

func TestOutboundOp_PingOmitsTopicFields(t *testing.T) {
	var ping = outboundOp{ID: "1", Type: typePing}
	var raw []byte
	var err error
	raw, err = codec.Marshal(ping)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got string = string(raw)
	var want string = `{"id":"1","type":"ping"}`
	if got != want {
		t.Fatalf("ping = %s, want %s", got, want)
	}
}

func TestOutboundOp_SubscribeCarriesTopic(t *testing.T) {
	var sub = outboundOp{
		ID:       "2",
		Type:     typeSubscribe,
		Topic:    "/contractMarket/level2:XBTUSDTM",
		Response: true,
	}
	var raw []byte
	var err error
	raw, err = codec.Marshal(sub)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// privateChannel:false is omitted; topic + response are present.
	var got string = string(raw)
	var want string = `{"id":"2","type":"subscribe","topic":"/contractMarket/level2:XBTUSDTM","response":true}`
	if got != want {
		t.Fatalf("subscribe = %s, want %s", got, want)
	}
}

func TestBuildDialURL(t *testing.T) {
	var got string
	var err error
	got, err = buildDialURL("wss://ws-api.kucoin.com/endpoint", "tok123")
	if err != nil {
		t.Fatalf("buildDialURL: %v", err)
	}
	// token must be present; connectId is generated, so just check prefix
	// and the token param.
	if !contains(got, "token=tok123") {
		t.Errorf("missing token in %q", got)
	}
	if !contains(got, "connectId=") {
		t.Errorf("missing connectId in %q", got)
	}
}

func TestBuildDialURL_EmptyEndpoint(t *testing.T) {
	if _, err := buildDialURL("", "tok"); err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

// contains is a tiny substring helper (avoids importing strings just for
// the assertions above).
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	var n int = len(s) - len(sub)
	var i int
	for i = 0; i <= n; i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
