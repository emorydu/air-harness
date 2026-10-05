package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/joho/godotenv"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have three tools:
base, read_file, write_file. Be concise.`

var toolDefs = []ToolDef{
	{
		Name:        "bash",
		Description: "Run a shell command and return its combined stdout/stderr.",
		InputSchema: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to run.",
			},
		},
		Required: []string{"command"},
	},
	{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path.",
		InputSchema: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Filesystem path to read.",
			},
		},
		Required: []string{"path"},
	},
	{
		Name:        "write_file",
		Description: "Write content to a file (creating or overwriting it).",
		InputSchema: map[string]any{
			"path":    map[string]any{"type": "string", "description": `The path to the file to write.`},
			"content": map[string]any{"type": "string", "description": `The content of the file to write.`},
		},
		Required: []string{"path", "content"},
	},
}

var (
	llm      Provider
	messages []Message
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

	llm = newAnthropicProvider("", 8192, systemPrompt)

	scanner := bufio.NewScanner(os.Stdin)
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
			return
		}
		input := strings.TrimSpace(scanner.Text())
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
		sp := startSpinner("thinking...")
		resp, err := llm.Send(ctx, messages, toolDefs)
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

	switch name {
	case "bash":
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}
		out, err := exec.CommandContext(ctx, "sh", "-c", in.Command).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("%s\n[exit error: %v]", out, err), true
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
