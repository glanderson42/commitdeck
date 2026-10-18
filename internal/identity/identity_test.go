package identity

import (
	"testing"
	"time"
)

func TestRevisionTokensAdvanceOnFrozenOrRegressedClock(t *testing.T) {
	clock := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	first := nextTimestamp(clock, "")
	second := nextTimestamp(clock, first)
	third := nextTimestamp(clock.Add(-time.Hour), second)
	a, _ := time.Parse(time.RFC3339Nano, first)
	b, _ := time.Parse(time.RFC3339Nano, second)
	c, _ := time.Parse(time.RFC3339Nano, third)
	if !b.After(a) || !c.After(b) {
		t.Fatal("reused revision token")
	}
	// A persisted revision can be ahead of this process's last clock value.
	persisted := clock.Add(time.Hour)
	newStamp, _ := time.Parse(time.RFC3339Nano, nextTimestamp(clock, persisted.Format(time.RFC3339Nano)))
	if !newStamp.After(persisted) {
		t.Fatal("restart reused persisted revision")
	}
}

func TestConcurrentFrozenClockHasUniqueTokens(t *testing.T) {
	clock := time.Now()
	values := make(chan string, 100)
	for range 100 {
		go func() { values <- nextTimestamp(clock, "") }()
	}
	seen := map[string]bool{}
	for range 100 {
		v := <-values
		if seen[v] {
			t.Fatal("duplicate revision")
		}
		seen[v] = true
	}
}
