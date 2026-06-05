/*
FILE: internal/ws/protocol.go

DESCRIPTION:
On-the-wire types for the KuCoin WebSocket protocol. The frame set is
small and mostly schema-only (no logic), kept here so json-iterator can
decode incoming frames in one pass.

CONNECTION MODEL (bullet token):
KuCoin WS has NO fixed URL and NO in-band login. A connection is opened
against an endpoint obtained from a REST "bullet" call:

	POST /api/v1/bullet-public   → public market-data token (no auth)
	POST /api/v1/bullet-private  → private token (signed REST request)

The bullet response yields a token and one or more instanceServers, each
with an endpoint and a server-driven pingInterval. The socket is dialled
at:

	<endpoint>?token=<token>&connectId=<unique>

The server greets a fresh socket with a {"type":"welcome"} frame echoing
connectId. Privacy is a property of the TOKEN (private bullet), not of an
in-band login step — so there is no login frame here, unlike Bitget.

OUTBOUND FRAMES we issue (all JSON):

	{"id":"<id>","type":"subscribe",  "topic":"/contractMarket/level2:XBTUSDTM","privateChannel":false,"response":true}
	{"id":"<id>","type":"unsubscribe","topic":"/contractMarket/level2:XBTUSDTM","privateChannel":false,"response":true}
	{"id":"<id>","type":"ping"}

INBOUND FRAMES we observe (all JSON):

	{"id":"<connectId>","type":"welcome"}
	{"id":"<pingId>",   "type":"pong"}
	{"id":"<subId>",    "type":"ack"}
	{"id":"<id>","type":"error","code":404,"data":"topic ... not found"}
	{"type":"message","topic":"/contractMarket/level2:XBTUSDTM","subject":"level2","data":{...}}

The Envelope struct below captures the union; the dispatcher distinguishes
by Type ("message" → push; everything else → control).
*/

package ws

import (
	"strconv"

	"github.com/tonymontanov/go-kucoin/v2/internal/codec"
)

// Frame type constants — the value of the "type" field on every KuCoin WS
// frame (inbound and outbound).
const (
	// typeWelcome — server greeting on a fresh socket.
	typeWelcome = "welcome"
	// typePing — client→server heartbeat.
	typePing = "ping"
	// typePong — server→client heartbeat reply.
	typePong = "pong"
	// typeSubscribe — client→server subscribe op.
	typeSubscribe = "subscribe"
	// typeUnsubscribe — client→server unsubscribe op.
	typeUnsubscribe = "unsubscribe"
	// typeAck — server acknowledgement of a subscribe/unsubscribe (when
	// response:true was requested).
	typeAck = "ack"
	// typeMessage — server data push (carries topic + subject + data).
	typeMessage = "message"
	// typeError — server error frame.
	typeError = "error"
)

/*
flexCode accepts either a JSON string or a JSON number and stores the
canonical decimal form as a string.

WHY:
KuCoin's "code" field on error frames is emitted as a JSON NUMBER
(e.g. {"type":"error","code":404,"data":"..."}), while some REST-adjacent
shapes quote it. Declaring Code as a plain string would make jsoniter
reject the number form and fail the whole envelope parse, so we would
misclassify and drop the frame. flexCode unifies both shapes into a
stable string so the dispatcher can keep its switch/equality style.
*/
type flexCode string

// UnmarshalJSON implements json.Unmarshaler — accepted both by the std
// library and by jsoniter's reflection-based decoder.
func (c *flexCode) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*c = ""
		return nil
	}
	if data[0] == '"' && data[len(data)-1] == '"' {
		var s string
		if err := codec.Unmarshal(data, &s); err != nil {
			return err
		}
		*c = flexCode(s)
		return nil
	}
	// Numeric form — keep the raw bytes (canonical decimal form).
	if _, err := strconv.ParseInt(string(data), 10, 64); err == nil {
		*c = flexCode(string(data))
		return nil
	}
	if _, err := strconv.ParseFloat(string(data), 64); err == nil {
		*c = flexCode(string(data))
		return nil
	}
	*c = flexCode(string(data))
	return nil
}

// String returns the canonical decimal form for use in switch / equality.
func (c flexCode) String() string { return string(c) }

// outboundOp is the JSON payload for a control frame we send to KuCoin.
// For ping frames only ID and Type are set; Topic/PrivateChannel/Response
// are omitted via omitempty.
type outboundOp struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Topic          string `json:"topic,omitempty"`
	PrivateChannel bool   `json:"privateChannel,omitempty"`
	Response       bool   `json:"response,omitempty"`
}

// Envelope captures the union of inbound frames.
//
//   - Control frames (welcome / ack / pong / error): Type + (Code/Data for
//     error) are populated; Topic is empty.
//   - Data pushes: Type == "message" with Topic / Subject / Data populated.
type Envelope struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Topic   string        `json:"topic"`
	Subject string        `json:"subject"`
	Code    flexCode      `json:"code"`
	Data    codec.RawJSON `json:"data"`
}

// IsPush returns true when the envelope describes a data push.
func (e *Envelope) IsPush() bool {
	return e.Type == typeMessage && e.Topic != ""
}

// IsControl returns true when the envelope is a control frame
// (welcome / ack / pong / error / anything that is not a data push).
func (e *Envelope) IsControl() bool {
	return e.Type != "" && e.Type != typeMessage
}
