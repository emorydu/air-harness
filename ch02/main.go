package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have three tools:
base, read_file, write_file. Be concise.`

var tools = []anthropic.ToolUnionParam{
	{OfTool: &anthropic.ToolParam{
		Name:        "bash",
		Description: anthropic.String("Run a shell command and return its combined stdout/stderr."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"command": map[string]any{"type": "string", "description": "The command to run."},
			},
			Required: []string{"command"},
		},
	}},
	{OfTool: &anthropic.ToolParam{
		Name:        "read_file",
		Description: anthropic.String("Read the contents of a file at the give path."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "Filesystem path to read."},
			},
			Required: []string{"path"},
		},
	}},
	{OfTool: &anthropic.ToolParam{
		Name:        "write_file",
		Description: anthropic.String("Write content to a file (creating or overwriting it)."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path":    map[string]any{"type": "string", "description": "Filesystem path to write."},
				"content": map[string]any{"type": "string", "description": "The bytes to write."},
			},
			Required: []string{"path", "content"},
		},
	}},
}

var client anthropic.Client

var confirm = func(string) bool { return true }

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	client = anthropic.NewClient(
		option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
		option.WithBaseURL(os.Getenv("ANTHROPIC_API_BASE_URL")),
	)

	ctx := context.Background()

	var messages []anthropic.MessageParam

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 4MB max line size

	confirm = func(prompt string) bool {
		fmt.Printf("%s [y/n] ", prompt)
		if !scanner.Scan() {
			return false
		}

		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(scanner.Text())), "y")
	}

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Printf("input read error: %v\n", err)
			}
			return // EOF (ctrl-D)
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(input)))
		messages = agentLoop(ctx, messages)
	}
}

func agentLoop(ctx context.Context, messages []anthropic.MessageParam) []anthropic.MessageParam {
	for {
		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeOpus4_7,
			MaxTokens: 8192,
			System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			fmt.Printf("api error: %v\n", err)
			return messages
		}
		messages = append(messages, resp.ToParam())

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.TextBlock:
				fmt.Println(v.Text)
			case anthropic.ToolUseBlock:
				result, isErr := executeTool(v.Name, v.JSON.Input.Raw())
				toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, result, isErr))
			}
		}
		if resp.StopReason != anthropic.StopReasonToolUse {
			return messages
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}

func executeTool(name, rawInput string) (string, bool) {
	fmt.Printf("[tool] %s %s\n", name, rawInput)

	if !confirm("approve?") {
		return "user denied this tool call", true
	}

	switch name {
	case "bash":
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}

		out, err := exec.Command("sh", "-c", in.Command).CombinedOutput()
		if err != nil {
			return err.Error(), true
		}

		return string(out), false

	case "read_file":
		var in struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}

		data, err := os.ReadFile(in.Path)
		if err != nil {
			return err.Error(), true
		}

		return string(data), false

	case "write_file":
		var in struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}

		if err := os.WriteFile(in.Path, []byte(in.Content), 0644); err != nil {
			return err.Error(), true
		}

		return "wrote " + in.Path, false

	default:
		return fmt.Sprintf("unknown tool: %s", name), true
	}
}
