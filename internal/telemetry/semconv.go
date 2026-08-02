package telemetry

import "go.opentelemetry.io/otel/attribute"

// Attribute keys specific to this agent. Anything with a GenAI semantic
// convention equivalent is not redefined here — use semconv directly.
const (
	AttrTurnID          = attribute.Key("agent.turn.id")
	AttrModePlan        = attribute.Key("agent.mode.plan")
	AttrModeSupervision = attribute.Key("agent.mode.supervision")
	AttrIterations      = attribute.Key("agent.iterations")
	AttrIterationCapHit = attribute.Key("agent.iteration_cap_hit")
	AttrToolCallsTotal  = attribute.Key("agent.tool_calls.total")
	AttrCostUSD         = attribute.Key("agent.cost.usd")
	AttrOutcome         = attribute.Key("agent.outcome")

	AttrIterationIndex = attribute.Key("agent.iteration.index")
	AttrIterationPhase = attribute.Key("agent.iteration.phase")

	AttrToolDecision        = attribute.Key("agent.tool.decision")
	AttrToolDeniedBy        = attribute.Key("agent.tool.denied_by")
	AttrToolApprovalGranted = attribute.Key("agent.tool.approval_granted")
	AttrToolApprovalWaitMS  = attribute.Key("agent.tool.approval_wait_ms")
	AttrToolOutputTruncated = attribute.Key("agent.tool.output_truncated")
	AttrToolOutputBytes     = attribute.Key("agent.tool.output_bytes")

	AttrErrorType = attribute.Key("error.type")
)

// Span names.
const (
	SpanTurn        = "agent.turn"
	SpanChat        = "gen_ai.chat"
	SpanExecuteTool = "gen_ai.execute_tool"
)

// agent.outcome values.
const (
	OutcomeCompleted        = "completed"
	OutcomeCapReached       = "cap_reached"
	OutcomeUserRejectedPlan = "user_rejected_plan"
	OutcomeError            = "error"
)

// agent.iteration.phase values.
const (
	PhasePlan    = "plan"
	PhaseExecute = "execute"
)
