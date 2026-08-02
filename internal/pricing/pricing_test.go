package pricing

import (
	"os"
	"testing"
)

// --- Cost ---

func TestCost_KnownModel(t *testing.T) {
	table := Table{prices: map[string]ModelPrice{
		"gpt-4o": {InputPer1M: 2.50, OutputPer1M: 10.00},
	}}

	// 1M input at $2.50 + 1M output at $10.00
	usd, known := table.Cost("gpt-4o", 1_000_000, 1_000_000)
	if !known {
		t.Fatal("expected gpt-4o to be priced")
	}
	if usd != 12.50 {
		t.Errorf("expected 12.50, got %v", usd)
	}
}

func TestCost_UnknownModelIsNotZero(t *testing.T) {
	table := DefaultPricing()
	usd, known := table.Cost("not-a-model", 1000, 1000)
	if known {
		t.Error("expected known=false for an unpriced model")
	}
	// The zero is only safe because known=false tells the caller to omit it.
	if usd != 0 {
		t.Errorf("expected 0 alongside known=false, got %v", usd)
	}
}

func TestCost_ZeroTokens(t *testing.T) {
	usd, known := DefaultPricing().Cost("gpt-4o", 0, 0)
	if !known || usd != 0 {
		t.Errorf("expected free call to cost 0, got %v (known=%v)", usd, known)
	}
}

// --- LoadPricing ---

func TestLoadPricing_FileNotFound_ReturnsDefault(t *testing.T) {
	table, err := LoadPricing("/nonexistent/pricing.json")
	if err != nil {
		t.Fatalf("expected no error when file is missing, got: %v", err)
	}
	if _, known := table.Cost("gpt-4o", 1, 1); !known {
		t.Error("expected the default table to price gpt-4o")
	}
}

func TestLoadPricing_ValidFile(t *testing.T) {
	f, err := os.CreateTemp("", "pricing-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{"custom-model": {"input_per_1m": 1.0, "output_per_1m": 2.0}}`)
	f.Close()

	table, err := LoadPricing(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usd, known := table.Cost("custom-model", 1_000_000, 1_000_000)
	if !known {
		t.Fatal("expected custom-model to be priced")
	}
	if usd != 3.0 {
		t.Errorf("expected 3.0, got %v", usd)
	}
}

func TestLoadPricing_MalformedJSON_ReturnsError(t *testing.T) {
	f, err := os.CreateTemp("", "pricing-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{ not valid json }`)
	f.Close()

	if _, err := LoadPricing(f.Name()); err == nil {
		t.Error("expected an error for malformed JSON, got nil")
	}
}
