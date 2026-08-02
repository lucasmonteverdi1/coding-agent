package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// Metrics holds the agent's instruments. Attributes recorded here are limited
// to bounded-cardinality values (tool names, decision enums), never paths,
// commands, or queries.
type Metrics struct {
	chatDuration   metric.Float64Histogram
	tokenUsage     metric.Int64Histogram
	toolCalls      metric.Int64Counter
	turnIterations metric.Int64Histogram
}

// NewMetrics builds the instruments from the global meter provider. With
// telemetry disabled the provider is a no-op and every record is a cheap
// no-op call, so callers need no enabled check.
func NewMetrics() *Metrics {
	meter := otel.Meter(scopeName)

	chatDuration, _ := meter.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of a GenAI chat operation."),
	)
	tokenUsage, _ := meter.Int64Histogram(
		"gen_ai.client.token.usage",
		metric.WithUnit("token"),
		metric.WithDescription("Tokens used per GenAI operation."),
	)
	toolCalls, _ := meter.Int64Counter(
		"agent.tool.calls",
		metric.WithUnit("{call}"),
		metric.WithDescription("Tool calls by name and guardrail decision."),
	)
	turnIterations, _ := meter.Int64Histogram(
		"agent.turn.iterations",
		metric.WithUnit("{iteration}"),
		metric.WithDescription("Iterations consumed per REPL turn."),
	)

	return &Metrics{
		chatDuration:   chatDuration,
		tokenUsage:     tokenUsage,
		toolCalls:      toolCalls,
		turnIterations: turnIterations,
	}
}

func (m *Metrics) RecordChatDuration(ctx context.Context, seconds float64, model string) {
	m.chatDuration.Record(ctx, seconds, metric.WithAttributes(
		semconv.GenAIOperationNameChat,
		semconv.GenAIRequestModel(model),
	))
}

func (m *Metrics) RecordTokens(ctx context.Context, in, out int64) {
	m.tokenUsage.Record(ctx, in, metric.WithAttributes(semconv.GenAITokenTypeInput))
	m.tokenUsage.Record(ctx, out, metric.WithAttributes(semconv.GenAITokenTypeOutput))
}

func (m *Metrics) RecordToolCall(ctx context.Context, tool, decision string) {
	m.toolCalls.Add(ctx, 1, metric.WithAttributes(
		semconv.GenAIToolName(tool),
		AttrToolDecision.String(decision),
	))
}

func (m *Metrics) RecordTurnIterations(ctx context.Context, iterations int, outcome string) {
	m.turnIterations.Record(ctx, int64(iterations), metric.WithAttributes(
		AttrOutcome.String(outcome),
	))
}
