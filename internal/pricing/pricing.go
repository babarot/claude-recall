// Package pricing estimates what the tokens of a session cost at the list
// prices of the Claude API. A Claude plan does not bill by token, so this
// is a measure of how much was used, not a bill.
package pricing

import (
	"regexp"
	"strings"
)

// Price is the list price of a model in US dollars per million tokens.
type Price struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

// Rates are a Price for each kind of token, in dollars per token, in the
// order Tokens lists them.
type Rates [5]float64

// Tokens are the tokens of one model: input, output, cache read, and cache
// written for 5 minutes and for an hour.
type Tokens [5]int64

// Cache reads cost a tenth of input; writes cost 1.25 times input for 5
// minutes and twice input for an hour.
func (p Price) rates() Rates {
	in := p.Input / 1e6
	return Rates{in, p.Output / 1e6, in * 0.1, in * 1.25, in * 2}
}

// Table looks up the price of a model.
type Table struct {
	set map[string]Price
}

// families are the prices a model gets when neither the config file nor
// the model's own name is in the table, by family. They are the list
// prices of the latest models of each family known when this was written,
// so a newer model is an estimate; the config file can set its price.
var families = map[string]Price{
	"opus":   {Input: 5, Output: 25},
	"sonnet": {Input: 3, Output: 15},
	"haiku":  {Input: 1, Output: 5},
}

var dated = regexp.MustCompile(`-\d{8}$`)

// New returns a table of the prices set in the config file ([ui.prices]),
// by model ID, over the defaults of each family.
func New(set map[string]Price) Table { return Table{set: set} }

// Rates returns the rates of model, and false when its price is not known.
func (t Table) Rates(model string) (Rates, bool) {
	base := dated.ReplaceAllString(model, "")
	for _, name := range []string{model, base} {
		if p, ok := t.set[name]; ok {
			return p.rates(), true
		}
	}
	for family, p := range families {
		if strings.HasPrefix(base, "claude-"+family+"-") || base == "claude-"+family {
			return p.rates(), true
		}
	}
	return Rates{}, false
}

// Cost is what tokens cost at rates.
func (r Rates) Cost(t Tokens) float64 {
	var c float64
	for i := range r {
		c += r[i] * float64(t[i])
	}
	return c
}
