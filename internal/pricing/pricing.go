// Package pricing resolves per-model token costs, loaded from an external
// JSON file. It mirrors the guardrails policy loading pattern: a missing file
// degrades to built-in defaults, malformed JSON is an error.
package pricing

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// ModelPrice is USD per million tokens.
type ModelPrice struct {
	InputPer1M  float64 `json:"input_per_1m"`
	OutputPer1M float64 `json:"output_per_1m"`
}

type Table struct {
	prices map[string]ModelPrice
}

func LoadPricing(path string) (Table, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultPricing(), nil
	}

	var prices map[string]ModelPrice
	if err := json.Unmarshal(data, &prices); err != nil {
		return Table{}, fmt.Errorf("pricing: invalid config: %w", err)
	}
	return Table{prices: prices}, nil
}

func DefaultPricing() Table {
	return Table{prices: map[string]ModelPrice{
		"gpt-4o":      {InputPer1M: 2.50, OutputPer1M: 10.00},
		"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.60},
	}}
}

// Cost returns the USD cost of a call. known is false when the model has no
// pricing entry — callers should omit the cost attribute rather than report
// zero, since a silent $0.00 is indistinguishable from a free turn.
func (t Table) Cost(model string, inputTokens, outputTokens int64) (usd float64, known bool) {
	p, ok := t.prices[model]
	if !ok {
		return 0, false
	}
	cost := float64(inputTokens)/1e6*p.InputPer1M +
		float64(outputTokens)/1e6*p.OutputPer1M

	// Rounded to the microdollar: plenty of precision for token pricing, and
	// it keeps binary float noise out of the exported attribute.
	return math.Round(cost*1e6) / 1e6, true
}
