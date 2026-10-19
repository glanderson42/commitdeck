package identity

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

var lastTimestamp atomic.Int64

func Now() string { return After("") }

// After keeps optimistic-concurrency tokens distinct even on coarse clocks or after restart.
func After(previous string) string { return nextTimestamp(time.Now(), previous) }

func nextTimestamp(now time.Time, previous string) string {
	next := now.UnixNano()
	if parsed, err := time.Parse(time.RFC3339Nano, previous); err == nil && parsed.UnixNano() >= next {
		next = parsed.UnixNano() + 1
	}
	for {
		last := lastTimestamp.Load()
		candidate := max(next, last+1)
		if lastTimestamp.CompareAndSwap(last, candidate) {
			// Fixed fractional precision also preserves lexical SQLite ordering.
			return time.Unix(0, candidate).UTC().Format("2006-01-02T15:04:05.000000000Z")
		}
	}
}
