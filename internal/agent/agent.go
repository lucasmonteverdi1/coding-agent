package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lucasmonteverdi1/coding-agent/internal/guardrails"
	"github.com/lucasmonteverdi1/coding-agent/internal/pricing"
	"github.com/lucasmonteverdi1/coding-agent/internal/telemetry"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// llmClient is the agent's view of the language model. Declared here rather
// than in internal/llm so tests can substitute a fake; *llm.Client satisfies
// it as-is.
type llmClient interface {
	Send(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam) (*openai.ChatCompletion, error)
	GetModel() string
	SetModel(model string)
}

type Agent struct {
	client        llmClient
	registry      tools.Registry
	guardrails    guardrails.Policy
	pricing       pricing.Table
	metrics       *telemetry.Metrics
	planMode      bool
	supervision   bool
	maxIterations int
	reader        *bufio.Reader
}

func New(client llmClient, registry tools.Registry, policy guardrails.Policy, prices pricing.Table, maxIter int) *Agent {
	return &Agent{
		client:        client,
		registry:      registry,
		guardrails:    policy,
		pricing:       prices,
		metrics:       telemetry.NewMetrics(),
		maxIterations: maxIter,
		reader:        bufio.NewReader(os.Stdin),
	}
}

func tracer() trace.Tracer { return otel.Tracer("github.com/lucasmonteverdi1/coding-agent") }

// turnStats accumulates per-turn totals for the root span. Only the agent
// goroutine writes to it — tool goroutines report through their own spans.
type turnStats struct {
	iterations   int
	toolCalls    int
	inputTokens  int64
	outputTokens int64
	costUSD      float64
	costKnown    bool
	capHit       bool
	planRejected bool
}

// phaseKey marks whether a nested Run is planning or executing. Plan mode
// calls Run recursively, so without this the two phases emit indistinguishable
// gen_ai.chat spans with colliding iteration indexes.
type phaseKey struct{}

func phaseFrom(ctx context.Context) string {
	if p, ok := ctx.Value(phaseKey{}).(string); ok {
		return p
	}
	return telemetry.PhaseExecute
}

func (agent *Agent) SetPlanMode(on bool) { agent.planMode = on }

func (agent *Agent) SetSupervision(on bool) { agent.supervision = on }

// Run executes one REPL turn under a root agent.turn span.
func (agent *Agent) Run(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	onEvent AgentEventHandler,
) (string, error) {
	ctx, span := tracer().Start(ctx, telemetry.SpanTurn,
		trace.WithAttributes(
			telemetry.AttrModePlan.Bool(agent.planMode),
			telemetry.AttrModeSupervision.Bool(agent.supervision),
		),
	)
	defer span.End()

	// ponytail: the trace ID already uniquely identifies the turn and joins it
	// to the trace — no separate UUID dependency for agent.turn.id.
	span.SetAttributes(telemetry.AttrTurnID.String(span.SpanContext().TraceID().String()))

	stats := &turnStats{}
	reply, err := agent.run(ctx, messages, onEvent, stats)
	agent.finishTurn(ctx, span, stats, err)

	// Report consumption to the terminal from the API's own usage metadata —
	// no collector involved.
	emitEvent(onEvent, AgentEvent{
		Type: EventTurnUsage,
		Usage: &TurnUsage{
			Model:        agent.client.GetModel(),
			Iterations:   stats.iterations,
			ToolCalls:    stats.toolCalls,
			InputTokens:  stats.inputTokens,
			OutputTokens: stats.outputTokens,
			CostUSD:      stats.costUSD,
			CostKnown:    stats.costKnown,
		},
	})
	return reply, err
}

// finishTurn stamps the aggregated turn attributes onto the root span.
func (agent *Agent) finishTurn(ctx context.Context, span trace.Span, stats *turnStats, err error) {
	outcome := telemetry.OutcomeCompleted
	switch {
	case stats.planRejected:
		outcome = telemetry.OutcomeUserRejectedPlan
	case stats.capHit:
		outcome = telemetry.OutcomeCapReached
	case err != nil:
		outcome = telemetry.OutcomeError
	}

	span.SetAttributes(
		telemetry.AttrIterations.Int(stats.iterations),
		telemetry.AttrIterationCapHit.Bool(stats.capHit),
		telemetry.AttrToolCallsTotal.Int(stats.toolCalls),
		semconv.GenAIUsageInputTokens(int(stats.inputTokens)),
		semconv.GenAIUsageOutputTokens(int(stats.outputTokens)),
		telemetry.AttrOutcome.String(outcome),
	)
	if stats.costKnown {
		span.SetAttributes(telemetry.AttrCostUSD.Float64(stats.costUSD))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	agent.metrics.RecordTurnIterations(ctx, stats.iterations, outcome)
}

func (agent *Agent) run(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	onEvent AgentEventHandler,
	stats *turnStats,
) (string, error) {
	toolParams := buildToolParams(agent.registry)

	// Inject system prompt if not present
	if len(messages) == 0 || messages[0].OfSystem == nil {
		messages = append([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
		}, messages...)
	}

	// Plan mode: generate and approve a plan before executing anything
	if agent.planMode {
		userMessage := extractLastUserMessage(messages)
		approved, revised, err := agent.runPlanMode(ctx, userMessage, onEvent, stats)
		if err != nil || !approved {
			// Rejection returns a nil error, so the outcome has to be flagged
			// here rather than inferred from the error at the call site.
			stats.planRejected = true
			return "Plan rejected. Send a new prompt to try again.", nil
		}
		if revised != "" {
			original := extractLastUserMessage(messages)
			messages[len(messages)-1] = openai.UserMessage(original + "\n\nRevision: " + revised)
		}
	}

	// INNER LOOP
	phase := phaseFrom(ctx)
	for i := 0; i < agent.maxIterations; i++ {
		stats.iterations++

		emitEvent(onEvent, AgentEvent{
			Type:      EventThinking,
			Message:   "Thinking...",
			Iteration: i + 1,
		})

		model := agent.client.GetModel()
		chatCtx, chatSpan := tracer().Start(ctx, telemetry.SpanChat,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				semconv.GenAIOperationNameChat,
				semconv.GenAIProviderNameOpenAI,
				semconv.GenAIRequestModel(model),
				telemetry.AttrIterationIndex.Int(i+1),
				telemetry.AttrIterationPhase.String(phase),
			),
		)

		start := time.Now()
		resp, err := agent.client.Send(chatCtx, messages, toolParams)
		agent.metrics.RecordChatDuration(chatCtx, time.Since(start).Seconds(), model)

		if err != nil {
			chatSpan.RecordError(err)
			chatSpan.SetStatus(codes.Error, err.Error())
			chatSpan.End()
			return "", fmt.Errorf("LLM: %w", err)
		}

		if len(resp.Choices) == 0 {
			err := fmt.Errorf("LLM returned no choices")
			chatSpan.RecordError(err)
			chatSpan.SetStatus(codes.Error, err.Error())
			chatSpan.End()
			return "", err
		}
		choice := resp.Choices[0]

		agent.recordUsage(chatCtx, chatSpan, resp, stats)
		chatSpan.SetAttributes(
			semconv.GenAIResponseModel(resp.Model),
			semconv.GenAIResponseFinishReasons(choice.FinishReason),
		)

		// No tool calls — LLM is done, return its text
		if len(choice.Message.ToolCalls) == 0 {
			chatSpan.End()
			emitEvent(onEvent, AgentEvent{
				Type:      EventFinalizing,
				Message:   "Writing answer...",
				Iteration: i + 1,
			})
			return choice.Message.Content, nil
		}

		// Append assistant's message with tool calls.
		messages = append(messages, choice.Message.ToParam())

		results := agent.runToolCalls(chatCtx, choice.Message.ToolCalls, onEvent, i+1, stats)
		messages = append(messages, results...)
		chatSpan.End()
	}

	stats.capHit = true
	return "", fmt.Errorf("reached max iterations (%d)", agent.maxIterations)
}

// runToolCalls processes one iteration's tool calls in two phases: approval
// decisions sequentially, then approved executions concurrently.
//
// ponytail: the approval phase must stay sequential — askConfirmation blocks on
// stdin, and concurrent prompts would interleave into an unreadable terminal.
// Only execution is parallelised.
func (agent *Agent) runToolCalls(
	ctx context.Context,
	toolCalls []openai.ChatCompletionMessageToolCall,
	onEvent AgentEventHandler,
	iteration int,
	stats *turnStats,
) []openai.ChatCompletionMessageParamUnion {
	type pending struct {
		idx      int // slot in results, so output order stays deterministic
		tc       openai.ChatCompletionMessageToolCall
		args     map[string]any
		decision guardrails.Decision
		waitMS   int64
	}

	// Results are written by index so the order of tool messages stays
	// deterministic regardless of which goroutine finishes first.
	results := make([]openai.ChatCompletionMessageParamUnion, len(toolCalls))
	approved := make([]pending, 0, len(toolCalls))

	// Phase A — sequential: parse, supervise, and check guardrails.
	for i, tc := range toolCalls {
		stats.toolCalls++

		args, err := parseToolArguments(tc.Function.Arguments)
		if err != nil {
			emitEvent(onEvent, AgentEvent{
				Type:      EventToolArgsError,
				Message:   fmt.Sprintf("Could not parse arguments for tool: %s", tc.Function.Name),
				ToolName:  tc.Function.Name,
				Iteration: iteration,
			})
			results[i] = openai.ToolMessage("error: could not parse tool arguments", tc.ID)
			agent.metrics.RecordToolCall(ctx, tc.Function.Name, "args_error")
			continue
		}

		emitEvent(onEvent, AgentEvent{
			Type:      EventToolCall,
			Message:   describeToolCall(tc.Function.Name, args),
			ToolName:  tc.Function.Name,
			Iteration: iteration,
		})

		supervisionStart := time.Now()
		rejected, msg := agent.handleSupervision(ctx, tc, args, onEvent, iteration)
		waitMS := time.Since(supervisionStart).Milliseconds()
		if rejected {
			results[i] = msg
			agent.metrics.RecordToolCall(ctx, tc.Function.Name, "user_rejected")
			continue
		}

		decision := agent.guardrails.CheckToolCall(guardrails.ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})

		switch decision.Type {
		case guardrails.Deny:
			emitEvent(onEvent, AgentEvent{
				Type:      EventGuardrailBlocked,
				Message:   decision.Reason,
				ToolName:  tc.Function.Name,
				Iteration: iteration,
			})
			results[i] = openai.ToolMessage("action blocked by guardrail: "+decision.Reason, tc.ID)
			agent.recordBlockedTool(ctx, tc, decision)
			continue
		case guardrails.RequireApproval:
			emitEvent(onEvent, AgentEvent{
				Type:      EventGuardrailApproval,
				Message:   decision.Reason,
				ToolName:  tc.Function.Name,
				Iteration: iteration,
			})
			approvalStart := time.Now()
			if !agent.supervision && !agent.askConfirmation(ctx, tc.Function.Name, args) {
				results[i] = openai.ToolMessage("user rejected action requiring approval", tc.ID)
				agent.metrics.RecordToolCall(ctx, tc.Function.Name, "user_rejected")
				continue
			}
			waitMS += time.Since(approvalStart).Milliseconds()
		}

		approved = append(approved, pending{
			idx: i, tc: tc, args: args, decision: decision, waitMS: waitMS,
		})
	}

	// Phase B — concurrent: execute approved calls as sibling spans. Each
	// goroutine writes to its own slot, so no lock is needed.
	var wg sync.WaitGroup
	for _, p := range approved {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[p.idx] = agent.executeToolInstrumented(ctx, p.tc, p.args, p.decision, p.waitMS, onEvent, iteration)
		}()
	}
	wg.Wait()

	// Drop unset slots: a tool call can only be skipped if it produced no
	// message, which never happens today, but a nil union would break the API.
	out := results[:0]
	for _, r := range results {
		if !isZeroMessage(r) {
			out = append(out, r)
		}
	}
	return out
}

func (agent *Agent) executeToolInstrumented(
	ctx context.Context,
	tc openai.ChatCompletionMessageToolCall,
	args map[string]any,
	decision guardrails.Decision,
	waitMS int64,
	onEvent AgentEventHandler,
	iteration int,
) openai.ChatCompletionMessageParamUnion {
	ctx, span := tracer().Start(ctx, telemetry.SpanExecuteTool,
		trace.WithAttributes(
			semconv.GenAIOperationNameExecuteTool,
			semconv.GenAIToolName(tc.Function.Name),
			telemetry.AttrToolDecision.String(string(decision.Type)),
			telemetry.AttrToolApprovalGranted.Bool(true),
			telemetry.AttrToolApprovalWaitMS.Int64(waitMS),
		),
	)
	defer span.End()

	if telemetry.CaptureContent() {
		span.AddEvent("gen_ai.tool.call", trace.WithAttributes(
			attribute.String("gen_ai.tool.call.arguments", tc.Function.Arguments),
		))
	}

	raw, err := agent.executeTool(ctx, tc, args)
	if err != nil {
		// The error still goes back to the LLM as a tool message so it can
		// self-correct; the span records it without interrupting the loop.
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(telemetry.AttrErrorType.String(errorType(err)))
		raw = fmt.Sprintf("error: %v", err)
	}

	result := agent.guardrails.SanitizeOutput(raw)
	span.SetAttributes(
		telemetry.AttrToolOutputBytes.Int(len(result)),
		telemetry.AttrToolOutputTruncated.Bool(len(result) != len(raw)),
	)
	agent.metrics.RecordToolCall(ctx, tc.Function.Name, string(decision.Type))

	emitEvent(onEvent, AgentEvent{
		Type:      EventToolDone,
		Message:   summarizeToolResult(result),
		ToolName:  tc.Function.Name,
		Iteration: iteration,
	})
	return openai.ToolMessage(result, tc.ID)
}

func (agent *Agent) recordBlockedTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall, decision guardrails.Decision) {
	_, span := tracer().Start(ctx, telemetry.SpanExecuteTool,
		trace.WithAttributes(
			semconv.GenAIOperationNameExecuteTool,
			semconv.GenAIToolName(tc.Function.Name),
			telemetry.AttrToolDecision.String(string(decision.Type)),
			telemetry.AttrToolDeniedBy.String(decision.Reason),
			telemetry.AttrToolApprovalGranted.Bool(false),
		),
	)
	span.SetStatus(codes.Error, "blocked by guardrail")
	span.End()
	agent.metrics.RecordToolCall(ctx, tc.Function.Name, string(decision.Type))
}

func (agent *Agent) recordUsage(ctx context.Context, span trace.Span, resp *openai.ChatCompletion, stats *turnStats) {
	in, out := resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	stats.inputTokens += in
	stats.outputTokens += out

	span.SetAttributes(
		semconv.GenAIUsageInputTokens(int(in)),
		semconv.GenAIUsageOutputTokens(int(out)),
	)
	agent.metrics.RecordTokens(ctx, in, out)

	// An unpriced model leaves agent.cost.usd absent rather than reporting
	// $0.00, which would be indistinguishable from a genuinely free turn.
	if cost, known := agent.pricing.Cost(agent.client.GetModel(), in, out); known {
		span.SetAttributes(telemetry.AttrCostUSD.Float64(cost))
		stats.costUSD += cost
		stats.costKnown = true
	} else {
		span.AddEvent("pricing.model_unknown", trace.WithAttributes(
			semconv.GenAIRequestModel(agent.client.GetModel()),
		))
	}
}

func (agent *Agent) executeTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall, args map[string]any) (string, error) {
	tool, ok := agent.registry[tc.Function.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", tc.Function.Name)
	}
	return tool.Handler(ctx, args)
}

func buildToolParams(registry tools.Registry) []openai.ChatCompletionToolParam {
	var params []openai.ChatCompletionToolParam
	for _, tool := range registry {
		var schema openai.FunctionParameters
		json.Unmarshal(tool.Schema, &schema)

		params = append(params, openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  schema,
			},
		})
	}
	return params
}

func extractLastUserMessage(messages []openai.ChatCompletionMessageParamUnion) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if msg := messages[i].OfUser; msg != nil {
			if !param.IsOmitted(msg.Content.OfString) {
				return msg.Content.OfString.Value
			}
		}
	}
	return ""
}
