/*
FILE: internal/kccommon/orderbook/engine.go

DESCRIPTION:
Profile-agnostic local order-book engine for the KuCoin Futures
"/contractMarket/level2:{symbol}" channel. Maintains a per-symbol
ascending-asks / descending-bids state, seeds from a REST snapshot, and
applies the single-change WS stream with SEQUENCE-based gap detection.

PROTOCOL (KuCoin Futures level2 — sequence model, NO CRC32):

  - SNAPSHOT: GET /api/v1/level2/snapshot?symbol= returns a baseline
    {sequence, asks, bids}. ApplySnapshot installs it and records the
    sequence as the high-water mark.
  - CHANGE (WS push): every frame carries one "price,side,size" change and
    a monotonic `sequence`. The reconciliation rules are:
      * sequence <= lastSequence        → stale, drop silently (already
        covered by the snapshot or a prior change);
      * sequence == lastSequence + 1    → apply (size==0 removes the level,
        otherwise upsert) and advance lastSequence;
      * sequence  > lastSequence + 1    → GAP: messages were missed. The
        engine flips `dirty` and returns ErrGap so the caller re-fetches
        the snapshot and resubscribes.

  There is NO checksum on KuCoin Futures level2 — correctness rests
  entirely on sequence contiguity.

ENGINE LIFECYCLE:

  - construct via NewEngine(symbol, maxDepth);
  - seed with ApplySnapshot, then feed each WS change through ApplyChange;
  - read state via Snapshot();
  - on Reset (called by ws.Conn before every (re)subscribe) the engine
    drops its state so the next snapshot becomes the authoritative truth.

CONCURRENCY:
Feature-complete behind a single mutex. The hot path is one mutex
acquisition per change — negligible for 200-level depth.
*/

package orderbook

import (
	"errors"
	"strings"
	"sync"

	"github.com/shopspring/decimal"

	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// Side constants as they appear in the KuCoin level2 change string.
const (
	// SideBuy — bid side.
	SideBuy = "buy"
	// SideSell — ask side.
	SideSell = "sell"
)

// ErrGap is returned by ApplyChange when the incoming sequence skips ahead
// of lastSequence+1, meaning one or more changes were missed. Callers must
// re-fetch the REST snapshot and resubscribe.
var ErrGap = errors.New("orderbook: sequence gap, resync required")

// ErrDirty is returned by ApplyChange while the engine is awaiting a
// (re)snapshot — before the first ApplySnapshot or right after a gap.
// Callers should drop the change and wait for the snapshot.
var ErrDirty = errors.New("orderbook: dirty, awaiting snapshot")

// ErrBadChange is returned by ParseChange when the wire change string is
// malformed.
var ErrBadChange = errors.New("orderbook: malformed level2 change")

// Level — one parsed order book level.
type Level struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

// Engine — per-symbol engine state.
type Engine struct {
	mu sync.Mutex

	symbol   string
	maxDepth int

	// asks — sorted ASCENDING by price (best ask = asks[0]).
	asks []Level
	// bids — sorted DESCENDING by price (best bid = bids[0]).
	bids []Level
	// tsMs — last applied push timestamp (ms).
	tsMs int64
	// lastSequence — sequence of the last applied snapshot/change.
	lastSequence int64
	// dirty — true before the first snapshot or after a gap.
	dirty bool
}

// NewEngine constructs an empty engine. maxDepth caps the stored side
// length; 0 falls back to 200 (matches the SDK Orderbook.MaxDepth default).
func NewEngine(symbol string, maxDepth int) *Engine {
	if maxDepth <= 0 {
		maxDepth = 200
	}
	return &Engine{
		symbol:   symbol,
		maxDepth: maxDepth,
		asks:     make([]Level, 0, maxDepth),
		bids:     make([]Level, 0, maxDepth),
		dirty:    true,
	}
}

// Reset drops state. Called by the ws.Conn supervisor before every
// (re)subscribe so a stale push from the previous socket cannot race the
// engine.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.asks = e.asks[:0]
	e.bids = e.bids[:0]
	e.tsMs = 0
	e.lastSequence = 0
	e.dirty = true
}

// ApplySnapshot installs the REST baseline. The snapshot's sequence becomes
// the high-water mark; subsequent ApplyChange calls expect sequence+1.
func (e *Engine) ApplySnapshot(asks, bids []Level, sequence, tsMs int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sortAsks(asks)
	sortBids(bids)

	if len(asks) > e.maxDepth {
		asks = asks[:e.maxDepth]
	}
	if len(bids) > e.maxDepth {
		bids = bids[:e.maxDepth]
	}

	e.asks = asks
	e.bids = bids
	e.tsMs = tsMs
	e.lastSequence = sequence
	e.dirty = false
}

// ApplyChange merges one level2 change.
//
//   - ErrDirty   — engine not seeded / awaiting snapshot;
//   - nil (drop) — stale change (sequence <= lastSequence);
//   - ErrGap     — sequence skipped ahead; caller must resync;
//   - nil (ok)   — applied; lastSequence advanced.
func (e *Engine) ApplyChange(side string, price, size decimal.Decimal, sequence, tsMs int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.dirty {
		return ErrDirty
	}
	if sequence <= e.lastSequence {
		return nil // stale — already covered by snapshot or prior change
	}
	if sequence != e.lastSequence+1 {
		e.dirty = true
		return ErrGap
	}

	switch side {
	case SideSell:
		e.asks = upsertAsk(e.asks, Level{Price: price, Size: size})
		if len(e.asks) > e.maxDepth {
			e.asks = e.asks[:e.maxDepth]
		}
	case SideBuy:
		e.bids = upsertBid(e.bids, Level{Price: price, Size: size})
		if len(e.bids) > e.maxDepth {
			e.bids = e.bids[:e.maxDepth]
		}
	default:
		// Unknown side — ignore the change but still advance the sequence
		// so contiguity is preserved (KuCoin should never emit this).
	}

	e.lastSequence = sequence
	e.tsMs = tsMs
	return nil
}

// Snapshot returns the current engine state. The slices are copies —
// callers may retain them across calls without worrying about mutation.
func (e *Engine) Snapshot() roottypes.OrderBookSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.dirty {
		return roottypes.OrderBookSnapshot{Symbol: e.symbol}
	}
	return roottypes.OrderBookSnapshot{
		Symbol:   e.symbol,
		Asks:     copyLevels(e.asks),
		Bids:     copyLevels(e.bids),
		TsMs:     e.tsMs,
		Sequence: e.lastSequence,
	}
}

// IsDirty reports whether the engine is awaiting a snapshot.
func (e *Engine) IsDirty() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dirty
}

// LastSequence returns the sequence of the last applied snapshot/change.
func (e *Engine) LastSequence() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastSequence
}

// ---------------------------------------------------------------------
// Wire decoding.
// ---------------------------------------------------------------------

// ParseChange splits a KuCoin level2 change string "price,side,size" into
// its parts. side is "buy" or "sell"; size 0 means the level is removed.
func ParseChange(change string) (side string, price, size decimal.Decimal, err error) {
	var parts []string = strings.Split(change, ",")
	if len(parts) != 3 {
		err = ErrBadChange
		return
	}
	price, err = decimal.NewFromString(strings.TrimSpace(parts[0]))
	if err != nil {
		return
	}
	side = strings.TrimSpace(parts[1])
	size, err = decimal.NewFromString(strings.TrimSpace(parts[2]))
	if err != nil {
		return
	}
	return side, price, size, nil
}

// LevelsFromPairs converts REST snapshot [price, size] pairs into engine
// Levels. The KuCoin REST level2 snapshot ships them as JSON numbers, which
// decode cleanly into decimal.Decimal via the codec, so the caller passes
// the already-decoded pairs here.
func LevelsFromPairs(pairs [][]decimal.Decimal) ([]Level, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	var out []Level = make([]Level, 0, len(pairs))
	var i int
	for i = 0; i < len(pairs); i++ {
		if len(pairs[i]) < 2 {
			return nil, errors.New("orderbook: snapshot level must be [price, size]")
		}
		out = append(out, Level{Price: pairs[i][0], Size: pairs[i][1]})
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Sorted-slice helpers.
// ---------------------------------------------------------------------

// sortAsks sorts ASCENDING by price (insertion sort — input is mostly
// pre-sorted, so the O(N) best case beats sort.Slice on the hot path).
func sortAsks(levels []Level) {
	var i int
	for i = 1; i < len(levels); i++ {
		var j int
		for j = i; j > 0 && levels[j].Price.LessThan(levels[j-1].Price); j-- {
			levels[j], levels[j-1] = levels[j-1], levels[j]
		}
	}
}

// sortBids sorts DESCENDING by price.
func sortBids(levels []Level) {
	var i int
	for i = 1; i < len(levels); i++ {
		var j int
		for j = i; j > 0 && levels[j].Price.GreaterThan(levels[j-1].Price); j-- {
			levels[j], levels[j-1] = levels[j-1], levels[j]
		}
	}
}

// upsertAsk inserts or replaces an ask level (ascending). size==0 → remove.
func upsertAsk(levels []Level, lvl Level) []Level {
	var i int
	for i = 0; i < len(levels); i++ {
		var cmp int = lvl.Price.Cmp(levels[i].Price)
		if cmp == 0 {
			if lvl.Size.IsZero() {
				return append(levels[:i], levels[i+1:]...)
			}
			levels[i] = lvl
			return levels
		}
		if cmp < 0 {
			if lvl.Size.IsZero() {
				return levels
			}
			return insertAt(levels, i, lvl)
		}
	}
	if lvl.Size.IsZero() {
		return levels
	}
	return append(levels, lvl)
}

// upsertBid inserts or replaces a bid level (descending). Mirror of
// upsertAsk except for the comparison direction.
func upsertBid(levels []Level, lvl Level) []Level {
	var i int
	for i = 0; i < len(levels); i++ {
		var cmp int = lvl.Price.Cmp(levels[i].Price)
		if cmp == 0 {
			if lvl.Size.IsZero() {
				return append(levels[:i], levels[i+1:]...)
			}
			levels[i] = lvl
			return levels
		}
		if cmp > 0 {
			if lvl.Size.IsZero() {
				return levels
			}
			return insertAt(levels, i, lvl)
		}
	}
	if lvl.Size.IsZero() {
		return levels
	}
	return append(levels, lvl)
}

// insertAt inserts lvl at index i, shifting the tail right.
func insertAt(levels []Level, i int, lvl Level) []Level {
	levels = append(levels, Level{})
	copy(levels[i+1:], levels[i:])
	levels[i] = lvl
	return levels
}

// copyLevels converts engine levels to roottypes OrderBookLevels in a
// fresh slice so callers can retain the result across mutations.
func copyLevels(src []Level) []roottypes.OrderBookLevel {
	var out []roottypes.OrderBookLevel = make([]roottypes.OrderBookLevel, len(src))
	var i int
	for i = 0; i < len(src); i++ {
		out[i] = roottypes.OrderBookLevel{Price: src[i].Price, Size: src[i].Size}
	}
	return out
}
