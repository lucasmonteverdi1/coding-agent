package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/lucasmonteverdi1/coding-agent/internal/guardrails"
	"github.com/lucasmonteverdi1/coding-agent/internal/pricing"
	"github.com/lucasmonteverdi1/coding-agent/internal/telemetry"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
	"github.com/openai/openai-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// --- fakes ---

// fakeLLM returns canned responses in order, satisfying the llmClient
// interface without any network access.
type fakeLLM struct {
	responses []*openai.ChatCompletion
	calls     int
	model     string
}

func (f *fakeLLM) Send(context.Context, []openai.ChatCompletionMessageParamUnion, []openai.ChatCompletionToolParam) (*openai.ChatCompletion, error) {
	resp := f.responses[f.calls]
	f.calls++
	return resp, nil
}

func (f *fakeLLM) GetModel() string      { return f.model }
func (f *fakeLLM) SetModel(model string) { f.model = model }

func toolCall(id, name, args string) openai.ChatCompletionMessageToolCall {
	var tc openai.ChatCompletionMessageToolCall
	tc.ID = id
	tc.Function.Name = name
	tc.Function.Arguments = args
	return tc
}

// respWithToolCalls builds a response that asks for tools; respFinal ends the loop.
func respWithToolCalls(calls ...openai.ChatCompletionMessageToolCall) *openai.ChatCompletion {
	var resp openai.ChatCompletion
	resp.Model = "gpt-4o"
	resp.Usage.PromptTokens = 100
	resp.Usage.CompletionTokens = 50
	resp.Choices = []openai.ChatCompletionChoice{{FinishReason: "tool_calls"}}
	resp.Choices[0].Message.ToolCalls = calls
	return &resp
}

func respFinal(content string) *openai.ChatCompletion {
	var resp openai.ChatCompletion
	resp.Model = "gpt-4o"
	resp.Usage.PromptTokens = 20
	resp.Usage.CompletionTokens = 10
	resp.Choices = []openai.ChatCompletionChoice{{FinishReason: "stop"}}
	resp.Choices[0].Message.Content = content
	return &resp
}

// newTestAgent wires an agent against a fake LLM and an in-memory span
// exporter, returning the recorder for assertions.
func newTestAgent(t *testing.T, llm *fakeLLM) (*Agent, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	ag := New(llm, testRegistry(), guardrails.DefaultPolicy(), pricing.DefaultPricing(), 10)
	return ag, exporter
}

// testRegistry provides trivial tools: two that succeed and one that fails.
func testRegistry() tools.Registry {
	r := tools.NewRegistry()
	for _, name := range []string{"echo_a", "echo_b"} {
		r.Register(tools.Tool{
			Name: name,
			Handler: func(context.Context, map[string]any) (string, error) {
				return "ok", nil
			},
		})
	}
	r.Register(tools.Tool{
		Name: "boom",
		Handler: func(context.Context, map[string]any) (string, error) {
			return "", errors.New("tool exploded")
		},
	})
	return r
}

func findSpans(spans tracetest.SpanStubs, name string) []tracetest.SpanStub {
	var out []tracetest.SpanStub
	for _, s := range spans {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

func attrValue(s tracetest.SpanStub, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range s.Attributes {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

// --- span hierarchy ---

func TestTelemetry_ProducesExpectedSpanHierarchy(t *testing.T) {
	llm := &fakeLLM{
		model: "gpt-4o",
		responses: []*openai.ChatCompletion{
			respWithToolCalls(
				toolCall("call_1", "echo_a", `{"value":"a"}`),
				toolCall("call_2", "echo_b", `{"value":"b"}`),
			),
			respFinal("done"),
		},
	}
	ag, exporter := newTestAgent(t, llm)

	reply, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("do the thing"),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "done" {
		t.Errorf("expected reply %q, got %q", "done", reply)
	}

	spans := exporter.GetSpans()

	turns := findSpans(spans, telemetry.SpanTurn)
	if len(turns) != 1 {
		t.Fatalf("expected exactly 1 %s span, got %d", telemetry.SpanTurn, len(turns))
	}
	turn := turns[0]
	if turn.Parent.IsValid() {
		t.Error("expected agent.turn to be a root span")
	}

	chats := findSpans(spans, telemetry.SpanChat)
	if len(chats) != 2 {
		t.Fatalf("expected 2 %s spans, got %d", telemetry.SpanChat, len(chats))
	}
	for _, c := range chats {
		if c.Parent.SpanID() != turn.SpanContext.SpanID() {
			t.Errorf("expected %s to be a child of agent.turn", telemetry.SpanChat)
		}
	}

	tools := findSpans(spans, telemetry.SpanExecuteTool)
	if len(tools) != 2 {
		t.Fatalf("expected 2 %s spans, got %d", telemetry.SpanExecuteTool, len(tools))
	}

	// Both tool spans must hang off the same chat span — the one whose
	// iteration requested them — not off the turn.
	firstChat := chatSpanWithToolCalls(t, chats, tools)
	for _, tl := range tools {
		if tl.Parent.SpanID() != firstChat.SpanContext.SpanID() {
			t.Errorf("expected tool span to be a child of its gen_ai.chat, got parent %s", tl.Parent.SpanID())
		}
		if tl.Parent.SpanID() == turn.SpanContext.SpanID() {
			t.Error("tool span must not be a direct child of agent.turn")
		}
	}
}

// chatSpanWithToolCalls returns the chat span that the tool spans belong to.
func chatSpanWithToolCalls(t *testing.T, chats, tools []tracetest.SpanStub) tracetest.SpanStub {
	t.Helper()
	parent := tools[0].Parent.SpanID()
	for _, c := range chats {
		if c.SpanContext.SpanID() == parent {
			return c
		}
	}
	t.Fatalf("no gen_ai.chat span matches the tool spans' parent")
	return tracetest.SpanStub{}
}

func TestTelemetry_ParallelToolCallsAreSiblings(t *testing.T) {
	llm := &fakeLLM{
		model: "gpt-4o",
		responses: []*openai.ChatCompletion{
			respWithToolCalls(
				toolCall("call_1", "echo_a", `{"value":"a"}`),
				toolCall("call_2", "echo_b", `{"value":"b"}`),
			),
			respFinal("done"),
		},
	}
	ag, exporter := newTestAgent(t, llm)

	if _, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("run both")}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tools := findSpans(exporter.GetSpans(), telemetry.SpanExecuteTool)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tool spans, got %d", len(tools))
	}
	if tools[0].Parent.SpanID() != tools[1].Parent.SpanID() {
		t.Error("expected concurrent tool spans to share a parent (siblings)")
	}
	if tools[0].SpanContext.SpanID() == tools[1].SpanContext.SpanID() {
		t.Error("expected distinct span IDs for the two tool calls")
	}
}

// --- turn attributes ---

func TestTelemetry_TurnAttributes(t *testing.T) {
	llm := &fakeLLM{
		model: "gpt-4o",
		responses: []*openai.ChatCompletion{
			respWithToolCalls(toolCall("call_1", "echo_a", `{"value":"a"}`)),
			respFinal("done"),
		},
	}
	ag, exporter := newTestAgent(t, llm)

	if _, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("go")}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	turn := findSpans(exporter.GetSpans(), telemetry.SpanTurn)[0]

	if v, ok := attrValue(turn, telemetry.AttrOutcome); !ok || v.AsString() != telemetry.OutcomeCompleted {
		t.Errorf("expected outcome %q, got %v", telemetry.OutcomeCompleted, v.AsString())
	}
	if v, ok := attrValue(turn, telemetry.AttrIterations); !ok || v.AsInt64() != 2 {
		t.Errorf("expected 2 iterations, got %v", v.AsInt64())
	}
	if v, ok := attrValue(turn, telemetry.AttrToolCallsTotal); !ok || v.AsInt64() != 1 {
		t.Errorf("expected 1 tool call, got %v", v.AsInt64())
	}
	// 100+20 input, 50+10 output across the two iterations.
	if v, ok := attrValue(turn, attribute.Key("gen_ai.usage.input_tokens")); !ok || v.AsInt64() != 120 {
		t.Errorf("expected 120 input tokens, got %v", v.AsInt64())
	}
	if v, ok := attrValue(turn, telemetry.AttrCostUSD); !ok {
		t.Error("expected agent.cost.usd on a priced model")
	} else if v.AsFloat64() <= 0 {
		t.Errorf("expected positive cost, got %v", v.AsFloat64())
	}
}

func TestTelemetry_UnknownModelOmitsCost(t *testing.T) {
	llm := &fakeLLM{
		model:     "not-a-real-model",
		responses: []*openai.ChatCompletion{respFinal("done")},
	}
	ag, exporter := newTestAgent(t, llm)

	if _, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("go")}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	turn := findSpans(exporter.GetSpans(), telemetry.SpanTurn)[0]
	if _, ok := attrValue(turn, telemetry.AttrCostUSD); ok {
		t.Error("expected agent.cost.usd to be absent for an unpriced model, not zero")
	}
}

// --- tool span attributes ---

func TestTelemetry_ToolSpanRecordsDecision(t *testing.T) {
	llm := &fakeLLM{
		model: "gpt-4o",
		responses: []*openai.ChatCompletion{
			respWithToolCalls(toolCall("call_1", "echo_a", `{"value":"a"}`)),
			respFinal("done"),
		},
	}
	ag, exporter := newTestAgent(t, llm)

	if _, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("go")}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := findSpans(exporter.GetSpans(), telemetry.SpanExecuteTool)[0]
	if v, ok := attrValue(tool, telemetry.AttrToolDecision); !ok || v.AsString() != string(guardrails.Allow) {
		t.Errorf("expected decision %q, got %v", guardrails.Allow, v.AsString())
	}
	if _, ok := attrValue(tool, telemetry.AttrToolOutputBytes); !ok {
		t.Error("expected agent.tool.output_bytes on the tool span")
	}
}

func TestTelemetry_ToolErrorIsRecordedButLoopContinues(t *testing.T) {
	llm := &fakeLLM{
		model: "gpt-4o",
		responses: []*openai.ChatCompletion{
			respWithToolCalls(toolCall("call_1", "boom", `{}`)),
			respFinal("recovered"),
		},
	}
	ag, exporter := newTestAgent(t, llm)

	reply, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("go")}, nil)
	if err != nil {
		t.Fatalf("a failing tool must not fail the run: %v", err)
	}
	if reply != "recovered" {
		t.Errorf("expected the loop to continue, got %q", reply)
	}

	tool := findSpans(exporter.GetSpans(), telemetry.SpanExecuteTool)[0]
	if tool.Status.Code != codes.Error {
		t.Errorf("expected span status Error, got %v", tool.Status.Code)
	}
	if len(tool.Events) == 0 {
		t.Error("expected RecordError to add an exception event")
	}
}

// --- plan phase ---

func TestTelemetry_ChatSpanCarriesIterationPhase(t *testing.T) {
	llm := &fakeLLM{
		model:     "gpt-4o",
		responses: []*openai.ChatCompletion{respFinal("done")},
	}
	ag, exporter := newTestAgent(t, llm)

	if _, err := ag.Run(context.Background(), []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("go")}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chat := findSpans(exporter.GetSpans(), telemetry.SpanChat)[0]
	if v, ok := attrValue(chat, telemetry.AttrIterationPhase); !ok || v.AsString() != telemetry.PhaseExecute {
		t.Errorf("expected phase %q, got %v", telemetry.PhaseExecute, v.AsString())
	}
}
