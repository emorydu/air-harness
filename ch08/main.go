package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have three tools:
base, read_file, write_file. Be concise.`

var (
	llm      Provider
	messages []Message
	// =========================================
	compactor CompactionStrategy = NoCompaction{}
	verbose   bool
	// =========================================
)

var confirm func(string) bool

var exitFunc = func() { os.Exit(0) }

func main() {
	fmt.Print(bannerText(TermWidth()))

	runREPL()
}

func runREPL() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	ctx := context.Background()
	confirm = confirmPrompt
	llm = newAnthropicProvider("", 8192, systemPrompt)

	for {
		input, ok := ReadChatInput()
		if !ok {
			return // ctrl-d / ctrl-c
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if runCommand(input) {
			continue
		}

		messages = append(messages, Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: input}}})
		agentLoop(ctx)
	}
}

func agentLoop(ctx context.Context) {
	for {
		// =========================================
		before := messages
		compacted, err := compactor.Compact(ctx, before)
		if err != nil {
			fmt.Printf("compaction error: %v (continuing without)\n", err)
		} else {
			if verbose && len(compacted) != len(before) {
				fmt.Printf("--- compaction: %d → %d ---\n", len(before), len(compacted))
				fmt.Print(renderTranscript(before))
				fmt.Println("---")
				fmt.Print(renderTranscript(compacted))
				fmt.Println("---")
			}
			messages = compacted
		}

		// =========================================

		sp := startSpinner("thinking...")
		resp, err := llm.Send(ctx, messages, Default.Definitions())
		sp.Stop()

		if err != nil {
			fmt.Printf("api error: %v\n", err)
			return
		}
		messages = append(messages, Message{Role: RoleAssistant, Content: resp.Content})

		var toolResults []Block
		for _, b := range resp.Content {
			switch b.Type {
			case BlockText:
				fmt.Println(b.Text)
			case BlockToolUse:
				result, isErr := executeTool(ctx, b.ToolName, b.ToolInput)
				toolResults = append(toolResults, Block{
					Type: BlockToolResult, ToolUseID: b.ToolUseID,
					ToolResult: result, IsError: isErr,
				})
			}
		}
		if resp.StopReason != StopToolUse {
			return
		}

		messages = append(messages, Message{Role: RoleUser, Content: toolResults})
	}
}

func executeTool(ctx context.Context, name, rawInput string) (string, bool) {
	fmt.Printf("[tool] %s %s\n", name, rawInput)
	if confirm != nil && !confirm("approve?") {
		return "user denied this tool call", true
	}

	return Default.Execute(ctx, name, rawInput)
}
