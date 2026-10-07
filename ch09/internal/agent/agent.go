package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/emorydu/air-harness/ch09/internal/api"
	"github.com/emorydu/air-harness/ch09/internal/compact"
	"github.com/emorydu/air-harness/ch09/internal/provider"
	"github.com/emorydu/air-harness/ch09/internal/tool"
)

type Agent struct {
	Provider  provider.Provider
	Tools     *tool.Registry
	Compactor compact.CompactionStrategy
	System    string
	MaxTurns  int
	Verbose   bool

	Confirm func(prompt string) bool

	messages []api.Message
}

func New(p provider.Provider, system string, tools *tool.Registry) *Agent {
	return &Agent{
		Provider:  p,
		Tools:     tools,
		Compactor: compact.NoCompaction{},
		System:    system,
		MaxTurns:  20,
	}
}

func (a *Agent) Messages() []api.Message { return a.messages }

func (a *Agent) SetMessages(msgs []api.Message) { a.messages = msgs }

func (a *Agent) ClearMessages() { a.messages = a.messages[:0] }

func (a *Agent) Send(ctx context.Context, prompt string) (string, error) {
	a.messages = append(a.messages, api.Message{
		Role:    api.RoleUser,
		Content: []api.Block{{Type: api.BlockText, Text: prompt}},
	})

	return a.loop(ctx)
}

func (a *Agent) loop(ctx context.Context) (string, error) {
	var finalText strings.Builder

	for turn := 0; turn < a.MaxTurns; turn++ {
		before := a.messages
		compacted, err := a.Compactor.Compact(ctx, before)
		if err != nil {
			fmt.Printf("compaction error: %v (continuing without)\n", err)
		} else {
			if a.Verbose && len(compacted) != len(before) {
				fmt.Printf("--- compaction: %d → %d ---\n", len(before), len(compacted))
				fmt.Print(api.RenderTranscript(before))
				fmt.Println("---")
				fmt.Print(api.RenderTranscript(compacted))
				fmt.Println("---")
			}
			a.messages = compacted
		}

		resp, err := a.Provider.Send(ctx, a.messages, a.Tools.Definitions())
		if err != nil {
			return finalText.String(), err
		}

		a.messages = append(a.messages, api.Message{Role: api.RoleAssistant, Content: resp.Content})

		var toolResults []api.Block
		var hasToolCall bool

		for _, b := range resp.Content {
			switch b.Type {
			case api.BlockText:
				if b.Text == "" {
					continue
				}
				fmt.Println(b.Text)
				finalText.WriteString(b.Text)
				finalText.WriteString("\n")
			case api.BlockToolUse:
				hasToolCall = true
				result, isErr := a.executeTool(ctx, b.ToolName, b.ToolInput)
				toolResults = append(toolResults, api.Block{
					Type:       api.BlockToolResult,
					ToolUseID:  b.ToolUseID,
					ToolResult: result,
					IsError:    isErr,
				})
			}
		}

		if resp.StopReason != api.StopToolUse || !hasToolCall {
			return strings.TrimSpace(finalText.String()), nil
		}

		a.messages = append(a.messages, api.Message{Role: api.RoleUser, Content: toolResults})
	}

	return strings.TrimSpace(finalText.String()), fmt.Errorf("max turns (%d) reached", a.MaxTurns)
}

func (a *Agent) executeTool(ctx context.Context, name, rawInput string) (string, bool) {
	fmt.Printf("[tool] %s %s\n", name, rawInput)
	if a.Confirm != nil && !a.Confirm("approve?") {
		return "user denied this tool call", true
	}

	return a.Tools.Execute(ctx, name, rawInput)
}

func ExtractWritePath(rawInput string) string {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return ""
	}
	return in.Path
}
