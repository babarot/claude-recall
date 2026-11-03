package pricing

import (
	"math"
	"testing"
)

func TestRates(t *testing.T) {
	tab := New(map[string]Price{"claude-new-6": {Input: 7, Output: 35}, "claude-opus-5": {Input: 6, Output: 30}})
	million := Tokens{1e6, 1e6, 1e6, 1e6, 1e6}
	cases := []struct {
		model string
		r     Request
		ok    bool
		cost  float64 // of a million of each kind of token
	}{
		{"claude-opus-5-5", Request{}, true, 4 + 20 + 0.2 + 5 + 8},             // a cache read at 0.05x
		{"claude-opus-5-5", Request{Fast: true}, true, 8 + 40 + 0.4 + 10 + 16}, // fast mode
		{"claude-fable-5-1", Request{}, true, 10 + 50 + 0.25 + 12.5 + 20},      // 0.025x
		{"claude-sonnet-5", Request{}, true, 2 + 10 + 0.2 + 2.5 + 4},           // the usual 0.1x
		{"claude-haiku-5-5", Request{}, true, 0.1 + 0.5 + 0.01 + 0.125 + 0.2},  // a short prompt
		{"claude-haiku-5-5", Request{Long: true}, true, 0.5 + 2.5 + 0.05 + 0.625 + 1},
		{"claude-haiku-4-5-20251001", Request{}, true, 1 + 5 + 0.1 + 1.25 + 2}, // a dated ID
		{"claude-opus-5", Request{Fast: true}, true, 6 + 30 + 0.6 + 7.5 + 12},  // the config's, in every mode
		{"claude-new-6", Request{}, true, 7 + 35 + 0.7 + 8.75 + 14},            // only in the config
		{"claude-sonnet-9", Request{}, false, 0},
	}
	for _, c := range cases {
		r, ok := tab.Rates(c.model, c.r)
		if ok != c.ok {
			t.Errorf("%s %+v: known %v, want %v", c.model, c.r, ok, c.ok)
			continue
		}
		if got := r.Cost(million); math.Abs(got-c.cost) > 1e-9 {
			t.Errorf("%s %+v: cost %v, want %v", c.model, c.r, got, c.cost)
		}
	}
}
