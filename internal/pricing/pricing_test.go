package pricing

import (
	"math"
	"testing"
)

func TestRates(t *testing.T) {
	tab := New(map[string]Price{"claude-fable-5-1": {Input: 10, Output: 50}, "claude-opus-5-5": {Input: 6, Output: 30}})
	cases := []struct {
		model string
		ok    bool
		cost  float64 // of a million of each kind of token
	}{
		{"claude-opus-5-5", true, 6 + 30 + 0.6 + 7.5 + 12},          // set in the config
		{"claude-opus-5", true, 5 + 25 + 0.5 + 6.25 + 10},           // the family's
		{"claude-haiku-4-5-20251001", true, 1 + 5 + 0.1 + 1.25 + 2}, // a dated name
		{"claude-fable-5-1", true, 10 + 50 + 1 + 12.5 + 20},
		{"claude-fable-5", false, 0}, // no family price
		{"gpt-5", false, 0},
	}
	for _, c := range cases {
		r, ok := tab.Rates(c.model)
		if ok != c.ok {
			t.Errorf("%s: known %v, want %v", c.model, ok, c.ok)
			continue
		}
		if got := r.Cost(Tokens{1e6, 1e6, 1e6, 1e6, 1e6}); math.Abs(got-c.cost) > 1e-9 {
			t.Errorf("%s: cost %v, want %v", c.model, got, c.cost)
		}
	}
}
