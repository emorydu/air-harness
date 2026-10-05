package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

type Block struct {
	Type BlockType

	Text string // BlockText

	ToolUseID string // BlockToolUse / BlockToolResult
	ToolName  string // BlockToolUse
	ToolInput string // BlockToolUse - Raw JSON

	ToolResult string // BlockToolResult
	IsError    bool   // BlockToolResult
}

type Message struct {
	Role    Role
	Content []Block
}

type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
	Required    []string
}

type StopReason string

const (
	StopEndTurn StopReason = "end_turn"
	StopToolUse StopReason = "tool_use"
	StopOther   StopReason = "other"
)

type Response struct {
	Content    []Block
	StopReason StopReason
}

type Provider interface {
	Send(ctx context.Context, messages []Message, tools []ToolDef) (Response, error)
	Model() string
	SetModel(name string)
}

type anthropicProvider struct {
	client    anthropic.Client
	model     anthropic.Model
	maxTokens int64
	system    string
}

var _ Provider = (*anthropicProvider)(nil)

func newAnthropicProvider(model string, maxTokens int64, system string) *anthropicProvider {
	m := anthropic.Model(model)
	if m == "" {
		m = anthropic.ModelClaudeOpus4_7
	}

	return &anthropicProvider{
		client: anthropic.NewClient(
			option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
			option.WithBaseURL(os.Getenv("ANTHROPIC_API_BASE_URL")),
		),
		model:     m,
		maxTokens: maxTokens,
		system:    system,
	}
}

func (p *anthropicProvider) Send(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: p.system}},
		Messages:  p.toMessages(messages),
		Tools:     p.toTools(tools),
	})
	if err != nil {
		return Response{}, err
	}

	out := Response{StopReason: fromStopReason(resp.StopReason)}
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			out.Content = append(out.Content, Block{Type: BlockText, Text: v.Text})
		case anthropic.ToolUseBlock:
			out.Content = append(out.Content, Block{
				Type: BlockToolUse, ToolUseID: v.ID, ToolName: v.Name, ToolInput: v.JSON.Input.Raw(),
			})
		}
	}

	return out, nil
}

func (p *anthropicProvider) toMessages(messages []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))

	for _, m := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case BlockText:
				blocks = append(blocks, anthropic.NewTextBlock(b.Text))
			case BlockToolUse:
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID: b.ToolUseID, Name: b.ToolName, Input: json.RawMessage(b.ToolInput),
					},
				})
			case BlockToolResult:
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.ToolResult, b.IsError))
			}
		}
		switch m.Role {
		case RoleUser:
			out = append(out, anthropic.NewUserMessage(blocks...))
		case RoleAssistant:
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		}
	}

	return out
}

func (p *anthropicProvider) toTools(tools []ToolDef) []anthropic.ToolUnionParam {
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

func fromStopReason(s anthropic.StopReason) StopReason {
	switch s {
	case anthropic.StopReasonEndTurn:
		return StopEndTurn
	case anthropic.StopReasonToolUse:
		return StopToolUse
	default:
		return StopOther
	}
}

func (p *anthropicProvider) Model() string {
	return string(p.model)
}

func (p *anthropicProvider) SetModel(name string) {
	p.model = anthropic.Model(name)
}
