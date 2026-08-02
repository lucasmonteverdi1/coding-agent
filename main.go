package main

import (
	"context"
	"flag"
	"log"

	"github.com/joho/godotenv"
	"github.com/lucasmonteverdi1/coding-agent/internal/agent"
	"github.com/lucasmonteverdi1/coding-agent/internal/guardrails"
	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/lucasmonteverdi1/coding-agent/internal/pricing"
	"github.com/lucasmonteverdi1/coding-agent/internal/telemetry"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
)

func main() {
	maxIter := flag.Int("max-iterations", 50, "maximum number of iterations per agent run")
	otelEnabled := flag.Bool("otel", false, "export OpenTelemetry traces and metrics")
	otelEndpoint := flag.String("otel-endpoint", "", "OTLP gRPC endpoint (default localhost:4317)")
	pricingPath := flag.String("pricing-config", "pricing.config.json", "path to the model pricing config")
	flag.Parse()

	godotenv.Load()

	policy, err := guardrails.LoadPolicy("agent.config.json")
	if err != nil {
		log.Fatal(err)
	}

	prices, err := pricing.LoadPricing(*pricingPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	shutdown, err := telemetry.Init(ctx, telemetry.ConfigFromEnv(telemetry.Config{
		Enabled:  *otelEnabled,
		Endpoint: *otelEndpoint,
	}))
	if err != nil {
		log.Fatal(err)
	}
	// Flush pending spans on the way out, however the REPL exits.
	defer shutdown(ctx)

	client := llm.NewClient()

	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool())
	registry.Register(tools.WriteFileTool())
	registry.Register(tools.RunCommandTool())
	registry.Register(tools.ListFilesTool())
	registry.Register(tools.WebSearchTool())

	agent.RunREPL(agent.New(client, registry, policy, prices, *maxIter))
}
