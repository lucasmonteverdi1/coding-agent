package llm

import (
	"context"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Client struct {
	api   openai.Client
	model string
}

func NewClient() *Client {
	return &Client{
		api:   openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY"))),
		model: "gpt-4o",
	}
}

// (c *Client) is the receiver. Equivalent to "this" or "self".
func (c *Client) Send(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	tools []openai.ChatCompletionToolParam,
) (*openai.ChatCompletion, error) {
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: messages,
	}
	if len(tools) > 0 {
		params.Tools = tools
	}
	return c.api.Chat.Completions.New(ctx, params)
}
