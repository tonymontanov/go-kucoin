/*
FILE: futures/stream_push_diagnostics_test.go

DESCRIPTION:
v2.8.1 — a private tradeOrders push whose `data` does not decode must not
vanish silently: orderPushHandler drops the frame, logs it at Warn with the
raw payload (truncated) and increments kucoin_ws_push_decode_failed_total.
A frame that decodes reaches the consumer handler untouched.
*/

package futures

import (
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
)

// recordingLogger — kucoin.Logger that keeps every line with its fields.
type recordingLogger struct {
	mu    sync.Mutex
	lines []recordedLine
}

type recordedLine struct {
	level  string
	msg    string
	fields map[string]string
}

func (l *recordingLogger) record(level, msg string, fields []kucoin.Field) {
	var fm map[string]string = make(map[string]string, len(fields))
	var i int
	for i = 0; i < len(fields); i++ {
		switch {
		case fields[i].ErrVal != nil:
			fm[fields[i].Key] = fields[i].ErrVal.Error()
		default:
			fm[fields[i].Key] = fields[i].Str
		}
	}
	l.mu.Lock()
	l.lines = append(l.lines, recordedLine{level: level, msg: msg, fields: fm})
	l.mu.Unlock()
}

func (l *recordingLogger) Debug(msg string, f ...kucoin.Field) { l.record("debug", msg, f) }
func (l *recordingLogger) Info(msg string, f ...kucoin.Field)  { l.record("info", msg, f) }
func (l *recordingLogger) Warn(msg string, f ...kucoin.Field)  { l.record("warn", msg, f) }
func (l *recordingLogger) Error(msg string, f ...kucoin.Field) { l.record("error", msg, f) }

func (l *recordingLogger) byLevel(level string) []recordedLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []recordedLine
	var i int
	for i = 0; i < len(l.lines); i++ {
		if l.lines[i].level == level {
			out = append(out, l.lines[i])
		}
	}
	return out
}

// countingMetrics — kucoin.CounterFactory that keeps one counter per name.
type countingMetrics struct {
	mu       sync.Mutex
	counters map[string]*countingCounter
}

type countingCounter struct {
	mu sync.Mutex
	n  uint64
}

func (c *countingCounter) Inc()          { c.mu.Lock(); c.n++; c.mu.Unlock() }
func (c *countingCounter) Add(d uint64)  { c.mu.Lock(); c.n += d; c.mu.Unlock() }
func (c *countingCounter) value() uint64 { c.mu.Lock(); defer c.mu.Unlock(); return c.n }
func (m *countingMetrics) Counter(name string) kucoin.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]*countingCounter)
	}
	var c *countingCounter = m.counters[name]
	if c == nil {
		c = &countingCounter{}
		m.counters[name] = c
	}
	return c
}

// newDiagnosticClient — futures client with a recording logger and counting
// metrics; no network is touched by the handler under test.
func newDiagnosticClient(t *testing.T) (*Client, *recordingLogger, *countingMetrics) {
	t.Helper()
	var logger *recordingLogger = &recordingLogger{}
	var metrics *countingMetrics = &countingMetrics{}
	var cfg kucoin.Config = kucoin.DefaultConfig()
	cfg.Logger = logger
	cfg.Metrics = metrics
	var parent *kucoin.Client
	var err error
	parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClient(parent), logger, metrics
}

func TestOrderPushHandler_DecodeFailureIsVisible(t *testing.T) {
	var client, logger, metrics = newDiagnosticClient(t)
	var delivered int
	var handle func(topic, subject string, data []byte) = client.stream.orderPushHandler(func(*futurestypes.OrderInfo) {
		delivered++
	})

	// `size` as an object cannot become a decimal: the frame must be dropped
	// and reported, the consumer handler must not run.
	var bad []byte = []byte(`{"orderId":"5cdfc138b21023a909e5ad55","symbol":"XBTUSDTM","type":"match","status":"open",` +
		`"size":{"oops":1},"filledSize":"20","matchSize":"20","matchPrice":"3600","ts":1545914149935808589}`)
	handle("/contractMarket/tradeOrders", "orderChange", bad)
	if delivered != 0 {
		t.Fatalf("undecodable frame reached the handler (%d)", delivered)
	}
	var warns []recordedLine = logger.byLevel("warn")
	if len(warns) != 1 || !strings.Contains(warns[0].msg, "decode failed") {
		t.Fatalf("want one warn line about the decode failure, got %+v", warns)
	}
	if warns[0].fields["topic"] != "/contractMarket/tradeOrders" || warns[0].fields["subject"] != "orderChange" {
		t.Fatalf("warn line must carry topic and subject: %+v", warns[0].fields)
	}
	if !strings.Contains(warns[0].fields["data"], `"oops":1`) || warns[0].fields["error"] == "" {
		t.Fatalf("warn line must carry the raw payload and the decode error: %+v", warns[0].fields)
	}
	if got := metrics.Counter(metricPushDecodeFailed).(*countingCounter).value(); got != 1 {
		t.Fatalf("%s = %d, want 1", metricPushDecodeFailed, got)
	}

	// A frame that decodes goes through, nothing new is logged or counted.
	var good []byte = []byte(`{"orderId":"5cdfc138b21023a909e5ad55","symbol":"XBTUSDTM","type":"open","status":"open",` +
		`"matchSize":"","matchPrice":"","orderType":"limit","side":"buy","price":"3600","size":"20000",` +
		`"remainSize":"20000","filledSize":"0","tradeId":"","clientOid":"c1","orderTime":1545914149935808589,` +
		`"liquidity":"","ts":1545914149935808589}`)
	handle("/contractMarket/tradeOrders", "orderChange", good)
	if delivered != 1 {
		t.Fatalf("decodable frame must reach the handler once, got %d", delivered)
	}
	if len(logger.byLevel("warn")) != 1 {
		t.Fatalf("a good frame must not log: %+v", logger.byLevel("warn"))
	}
	if got := metrics.Counter(metricPushDecodeFailed).(*countingCounter).value(); got != 1 {
		t.Fatalf("%s = %d after a good frame, want still 1", metricPushDecodeFailed, got)
	}
}

// The raw payload in the warn line is capped: a huge frame must not flood the log.
func TestOrderPushHandler_RawPayloadTruncated(t *testing.T) {
	var client, logger, _ = newDiagnosticClient(t)
	var handle func(topic, subject string, data []byte) = client.stream.orderPushHandler(func(*futurestypes.OrderInfo) {})
	var huge []byte = []byte(`{"size":{"x":"` + strings.Repeat("a", 4096) + `"}}`)
	handle("/contractMarket/tradeOrders", "orderChange", huge)
	var warns []recordedLine = logger.byLevel("warn")
	if len(warns) != 1 {
		t.Fatalf("want one warn, got %d", len(warns))
	}
	var data string = warns[0].fields["data"]
	if len(data) > pushDataLogLimit+40 || !strings.HasSuffix(data, " bytes)") {
		t.Fatalf("payload must be truncated with a byte-count marker, got len=%d tail=%q", len(data), data[len(data)-24:])
	}
}
