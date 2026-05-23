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
)

type AgentEvent struct {
	Type      AgentEventType
	Message   string
	ToolName  string
	Iteration int
}

type AgentEventHandler func(AgentEvent)

func emitEvent(onEvent AgentEventHandler, event AgentEvent) {
	if onEvent != nil {
		onEvent(event)
	}
}
