package provider

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/emorydu/air-harness/ch11/internal/api"
)

type AnthropicProvider struct {
	client    anthropic.Client
	maxTokens int64
	system    string

	mu    sync.RWMutex
	model anthropic.Model
}

var _ Provider = &AnthropicProvider{}

func NewAnthropicProvider(model string, maxTokens int64, system string) *AnthropicProvider {
	m := anthropic.Model(model)
	if m == "" {
		m = anthropic.ModelClaudeOpus4_7
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return &AnthropicProvider{
		client:    anthropic.NewClient(opts...),
		model:     m,
		maxTokens: maxTokens,
		system:    system,
	}
}

func (p *AnthropicProvider) Send(ctx context.Context, messages []api.Message, tools []api.ToolDef) (api.Response, error) {
	p.mu.RLock()
	model := p.model
	p.mu.RUnlock()

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: p.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: p.system}},
		Messages:  p.toMessages(messages),
		Tools:     p.toTools(tools),
	})
	if err != nil {
		return api.Response{}, err
	}

	out := api.Response{StopReason: fromStopReason(resp.StopReason)}
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			out.Content = append(out.Content, api.Block{Type: api.BlockText, Text: v.Text})
		case anthropic.ToolUseBlock:
			out.Content = append(out.Content, api.Block{Type: api.BlockToolUse, ToolUseID: v.ID, ToolName: v.Name, ToolInput: v.JSON.Input.Raw()})
		}
	}

	return out, nil
}

func (p *AnthropicProvider) toMessages(messages []api.Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))

	for _, m := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case api.BlockText:
				blocks = append(blocks, anthropic.NewTextBlock(b.Text))
			case api.BlockToolUse:
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID: b.ToolUseID, Name: b.ToolName, Input: json.RawMessage(b.ToolInput),
					},
				})
			case api.BlockToolResult:
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.ToolResult, b.IsError))
			}
		}

		switch m.Role {
		case api.RoleUser:
			out = append(out, anthropic.NewUserMessage(blocks...))
		case api.RoleAssistant:
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		}
	}

	return out
}

func (p *AnthropicProvider) toTools(tools []api.ToolDef) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))

	for _, t := range tools {
		out = append(out, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: t.InputSchema, Required: t.Required},
		}})
	}

	return out
}

func (p *AnthropicProvider) Model() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return string(p.model)
}

func (p *AnthropicProvider) SetModel(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.model = anthropic.Model(name)
}

func fromStopReason(s anthropic.StopReason) api.StopReason {
	switch s {
	case anthropic.StopReasonEndTurn:
		return api.StopEndTurn
	case anthropic.StopReasonToolUse:
		return api.StopToolUse
	default:
		return api.StopOther
	}
}
