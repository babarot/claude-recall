// Package pricing estimates what the tokens of a session cost at the list
// prices of the Claude API. A Claude plan does not bill by token, so this
// is a measure of how much was used, not a bill.
package pricing

import "regexp"

// Price is the list price of a model in US dollars per million tokens.
type Price struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

// Tokens are the tokens of one model: input, output, cache read, and cache
// written for 5 minutes and for an hour.
type Tokens [5]int64

// Rates are the price of each kind of token, in dollars per token, in the
// order Tokens lists them.
type Rates [5]float64

// Cost is what tokens cost at rates.
func (r Rates) Cost(t Tokens) float64 {
	var c float64
	for i := range r {
		c += r[i] * float64(t[i])
	}
	return c
}

// model is what the price list says of a model.
type model struct {
	Price
	// read is what a cache read costs, as a share of input: 0 for the
	// usual tenth.
	read float64
	// fast is the price in fast mode, zero when the model has none.
	fast Price
	// long is the price of a request whose prompt (input, cache reads and
	// writes) is over LongPrompt tokens, zero when it is not priced apart.
	long Price
}

// LongPrompt is how many tokens of prompt (input, cache reads and writes)
// a request has before some models price it higher.
const LongPrompt = 100_000

// list is the Claude API's price list, https://platform.claude.com/docs/en/about-claude/pricing,
// as of November 2026, by model ID without its date.
var list = map[string]model{
	"claude-fable-5-1":  {Price: Price{10, 50}, read: 0.025},
	"claude-mythos-5-1": {Price: Price{10, 50}, read: 0.025},
	"claude-fable-5":    {Price: Price{10, 50}},
	"claude-mythos-5":   {Price: Price{10, 50}},
	"claude-opus-5-5":   {Price: Price{4, 20}, read: 0.05, fast: Price{8, 40}},
	"claude-opus-5":     {Price: Price{5, 25}, fast: Price{10, 50}},
	"claude-opus-4-8":   {Price: Price{5, 25}, fast: Price{10, 50}},
	"claude-opus-4-7":   {Price: Price{5, 25}},
	"claude-opus-4-6":   {Price: Price{5, 25}},
	"claude-opus-4-5":   {Price: Price{5, 25}},
	"claude-opus-4-1":   {Price: Price{15, 75}},
	"claude-opus-4":     {Price: Price{15, 75}},
	"claude-sonnet-5-5": {Price: Price{2, 10}, read: 0.05},
	"claude-sonnet-5":   {Price: Price{2, 10}},
	"claude-sonnet-4-6": {Price: Price{3, 15}},
	"claude-sonnet-4-5": {Price: Price{3, 15}},
	"claude-sonnet-4":   {Price: Price{3, 15}},
	"claude-haiku-5-5":  {Price: Price{0.1, 0.5}, long: Price{0.5, 2.5}},
	"claude-haiku-4-5":  {Price: Price{1, 5}},
	"claude-3-5-haiku":  {Price: Price{0.8, 4}},
}

var dated = regexp.MustCompile(`-\d{8}$`)

// Table looks up the price of a model: the config file's ([ui.prices]),
// by model ID, over the price list.
type Table struct {
	set map[string]Price
}

// New returns a table with the prices set in the config file.
func New(set map[string]Price) Table { return Table{set: set} }

// Request is what decides the price of a response besides its model.
type Request struct {
	Fast bool // in fast mode
	Long bool // with a prompt over 100,000 tokens
}

// Rates returns the rates of a model's responses of the kind r, and false
// when its price is not known. A price set in the config file applies to
// every kind of response; cache reads keep the model's share of input.
func (t Table) Rates(id string, r Request) (Rates, bool) {
	base := dated.ReplaceAllString(id, "")
	m, listed := list[base]
	p, set := t.set[id]
	if !set {
		p, set = t.set[base]
	}
	switch {
	case set:
	case !listed:
		return Rates{}, false
	case r.Fast && m.fast != (Price{}):
		p = m.fast
	case r.Long && m.long != (Price{}):
		p = m.long
	default:
		p = m.Price
	}
	read := m.read
	if read == 0 {
		read = 0.1
	}
	in := p.Input / 1e6
	return Rates{in, p.Output / 1e6, in * read, in * 1.25, in * 2}, true
}
