package agent

// map for O(1) lookup; values are empty structs to avoid wasted memory.
var validModels = map[string]struct{}{
	"gpt-5.5":      {},
	"gpt-5.4":      {},
	"gpt-5.4-mini": {},
	"gpt-5.4-nano": {},
	"gpt-5":        {},
	"gpt-5-mini":   {},
	"gpt-5-nano":   {},

	"gpt-5.3-chat-latest": {},
	"gpt-5.2-chat-latest": {},
	"gpt-5.1-chat-latest": {},
	"gpt-5-chat-latest":   {},
	"chatgpt-4o-latest":   {},

	"gpt-4.1":      {},
	"gpt-4.1-mini": {},
	"gpt-4.1-nano": {},

	"gpt-4o":      {},
	"gpt-4o-mini": {},

	"gpt-4.5-preview": {},
	"gpt-4-turbo":     {},
	"gpt-4":           {},
	"gpt-3.5-turbo":   {},

	// Reasoning models
	"o1":      {},
	"o1-mini": {},
	"o1-pro":  {},
	"o3":      {},
	"o3-mini": {},
	"o3-pro":  {},
	"o4-mini": {},
}

func isValidModel(model string) bool {
	_, ok := validModels[model]
	return ok
}
