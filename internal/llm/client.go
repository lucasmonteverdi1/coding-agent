package llm

import (
	"context"
	"os"

	"github.com/lucasmonteverdi1/coding-agent/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Client struct {
	api   openai.Client
	model string
}

func NewClient() *Client {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" || !models.IsValidModel(model) {
		model = "gpt-4o"
	}
	return &Client{
		api:   openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY"))),
		model: model,
	}
}

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

func (c *Client) SetModel(model string) {
	c.model = model
}

func (c *Client) GetModel() string {
	return c.model
}
