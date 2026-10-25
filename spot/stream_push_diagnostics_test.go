/*
FILE: spot/stream_push_diagnostics_test.go

DESCRIPTION:
v2.8.1 — twin of futures/stream_push_diagnostics_test.go for the spot
tradeOrders topic: an undecodable push is dropped visibly (Warn with the raw
payload + kucoin_ws_push_decode_failed_total), a decodable one is delivered.
*/

package spot

import (
	"strings"
	"sync"
	"testing"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	spottypes "github.com/tonymontanov/go-kucoin/v2/spot/types"
)

type spotRecordingLogger struct {
	mu    sync.Mutex
	warns []map[string]string
}

func (l *spotRecordingLogger) Debug(string, ...kucoin.Field) {}
func (l *spotRecordingLogger) Info(string, ...kucoin.Field)  {}
func (l *spotRecordingLogger) Error(string, ...kucoin.Field) {}
func (l *spotRecordingLogger) Warn(msg string, fields ...kucoin.Field) {
	var fm map[string]string = map[string]string{"msg": msg}
	var i int
	for i = 0; i < len(fields); i++ {
		if fields[i].ErrVal != nil {
			fm[fields[i].Key] = fields[i].ErrVal.Error()
			continue
		}
		fm[fields[i].Key] = fields[i].Str
	}
	l.mu.Lock()
	l.warns = append(l.warns, fm)
	l.mu.Unlock()
}

type spotCounter struct {
	mu sync.Mutex
	n  uint64
}

func (c *spotCounter) Inc()         { c.mu.Lock(); c.n++; c.mu.Unlock() }
func (c *spotCounter) Add(d uint64) { c.mu.Lock(); c.n += d; c.mu.Unlock() }

type spotMetrics struct {
	mu       sync.Mutex
	counters map[string]*spotCounter
}

func (m *spotMetrics) Counter(name string) kucoin.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]*spotCounter)
	}
	if m.counters[name] == nil {
		m.counters[name] = &spotCounter{}
	}
	return m.counters[name]
}

func TestSpotOrderPushHandler_DecodeFailureIsVisible(t *testing.T) {
	var logger *spotRecordingLogger = &spotRecordingLogger{}
	var metrics *spotMetrics = &spotMetrics{}
	var cfg kucoin.Config = kucoin.DefaultConfig()
	cfg.Logger = logger
	cfg.Metrics = metrics
	var parent *kucoin.Client
	var err error
	parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var client *Client = NewClient(parent)

	var delivered int
	var handle func(topic, subject string, data []byte) = client.stream.orderPushHandler(func(*spottypes.OrderInfo) {
		delivered++
	})
	handle("/spotMarket/tradeOrders", "orderChange", []byte(`{"orderId":"o1","symbol":"BTC-USDT","size":{"oops":true}}`))
	if delivered != 0 {
		t.Fatalf("undecodable frame reached the handler")
	}
	logger.mu.Lock()
	var warns []map[string]string = append([]map[string]string(nil), logger.warns...)
	logger.mu.Unlock()
	if len(warns) != 1 || !strings.Contains(warns[0]["msg"], "decode failed") ||
		warns[0]["topic"] != "/spotMarket/tradeOrders" || !strings.Contains(warns[0]["data"], "oops") || warns[0]["error"] == "" {
		t.Fatalf("want one warn with topic, raw data and error, got %+v", warns)
	}
	var c *spotCounter = metrics.Counter(metricPushDecodeFailed).(*spotCounter)
	c.mu.Lock()
	var n uint64 = c.n
	c.mu.Unlock()
	if n != 1 {
		t.Fatalf("%s = %d, want 1", metricPushDecodeFailed, n)
	}

	handle("/spotMarket/tradeOrders", "orderChange", []byte(`{"orderId":"o1","symbol":"BTC-USDT","type":"open","status":"open",`+
		`"side":"buy","price":"100","size":"1","filledSize":"0","remainSize":"1","clientOid":"c1","ts":1545914149935808589}`))
	if delivered != 1 {
		t.Fatalf("decodable frame must be delivered once, got %d", delivered)
	}
}
