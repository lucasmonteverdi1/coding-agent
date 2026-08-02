package agent

type AgentEventType string

const (
	EventThinking            AgentEventType = "thinking"
	EventToolCall            AgentEventType = "tool_call"
	EventToolArgsError       AgentEventType = "tool_args_error"
	EventToolDone            AgentEventType = "tool_done"
	EventFinalizing          AgentEventType = "finalizing"
	EventPlanReady           AgentEventType = "plan_ready"
	EventPlanRejected        AgentEventType = "plan_rejected"
	EventSupervisionRejected AgentEventType = "supervision_rejected"
	EventGuardrailBlocked    AgentEventType = "guardrail_blocked"
	EventGuardrailApproval   AgentEventType = "guardrail_approval"
	EventTurnUsage           AgentEventType = "turn_usage"
)

type AgentEvent struct {
	Type      AgentEventType
	Message   string
	ToolName  string
	Iteration int

	// Set on EventTurnUsage only: what the turn consumed. Sourced from the
	// OpenAI response metadata, so it needs no collector.
	Usage *TurnUsage
}

// TurnUsage is what one REPL turn cost, in tokens and dollars.
type TurnUsage struct {
	Model        string
	Iterations   int
	ToolCalls    int
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	CostKnown    bool // false when the model has no pricing entry
}

type AgentEventHandler func(AgentEvent)

func emitEvent(onEvent AgentEventHandler, event AgentEvent) {
	if onEvent != nil {
		onEvent(event)
	}
}
