package agent

import (
	"testing"
)

// --- formatUSD ---

func TestFormatUSD_SubCentKeepsPrecision(t *testing.T) {
	// A turn costing fractions of a cent must not render as $0.00.
	if got := formatUSD(0.00015); got != "$0.000150" {
		t.Errorf("expected $0.000150, got %s", got)
	}
}

func TestFormatUSD_NormalAmount(t *testing.T) {
	if got := formatUSD(1.2345); got != "$1.2345" {
		t.Errorf("expected $1.2345, got %s", got)
	}
}

func TestFormatUSD_Zero(t *testing.T) {
	if got := formatUSD(0); got != "$0.0000" {
		t.Errorf("expected $0.0000, got %s", got)
	}
}

// --- sessionUsage ---

func TestSessionUsage_AccumulatesAcrossTurns(t *testing.T) {
	var s sessionUsage

	s.add(&TurnUsage{Model: "gpt-4o", InputTokens: 100, OutputTokens: 50, CostUSD: 0.001, CostKnown: true})
	s.add(&TurnUsage{Model: "gpt-4o", InputTokens: 200, OutputTokens: 80, CostUSD: 0.002, CostKnown: true})

	if s.turns != 2 {
		t.Errorf("expected 2 turns, got %d", s.turns)
	}
	if s.inputTokens != 300 {
		t.Errorf("expected 300 input tokens, got %d", s.inputTokens)
	}
	if s.outputTokens != 130 {
		t.Errorf("expected 130 output tokens, got %d", s.outputTokens)
	}
	if s.costUSD != 0.003 {
		t.Errorf("expected 0.003 total cost, got %v", s.costUSD)
	}
}

func TestSessionUsage_UnknownCostDoesNotClaimKnown(t *testing.T) {
	var s sessionUsage
	s.add(&TurnUsage{Model: "mystery-model", InputTokens: 10, OutputTokens: 5, CostKnown: false})

	if s.costKnown {
		t.Error("expected costKnown to stay false for an unpriced model")
	}
	if s.inputTokens != 10 {
		t.Errorf("tokens must still accumulate without pricing, got %d", s.inputTokens)
	}
}

func TestSessionUsage_MixedPricingMarksKnown(t *testing.T) {
	var s sessionUsage
	s.add(&TurnUsage{Model: "mystery", CostKnown: false})
	s.add(&TurnUsage{Model: "gpt-4o", CostUSD: 0.5, CostKnown: true})

	// One priced turn is enough to make the running total meaningful.
	if !s.costKnown {
		t.Error("expected costKnown to become true once any turn is priced")
	}
	if s.costUSD != 0.5 {
		t.Errorf("expected 0.5, got %v", s.costUSD)
	}
}

func TestSessionUsage_NilIsIgnored(t *testing.T) {
	var s sessionUsage
	s.add(nil)
	if s.turns != 0 {
		t.Errorf("expected a nil usage to be ignored, got %d turns", s.turns)
	}
}
