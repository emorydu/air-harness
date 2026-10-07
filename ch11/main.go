package main

import (
	"context"
	"fmt"
	"os"

	"github.com/emorydu/air-harness/ch11/internal/agent"
	"github.com/emorydu/air-harness/ch11/internal/provider"
	"github.com/emorydu/air-harness/ch11/internal/subagent"
	"github.com/emorydu/air-harness/ch11/internal/tool"
	"github.com/emorydu/air-harness/ch11/internal/ui"
	"github.com/joho/godotenv"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have three tools:
bash, read_file, write_file. Be concise.`

var (
	llm  provider.Provider
	root *agent.Agent

	exitFunc = func() { os.Exit(0) }
)

func main() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	fmt.Print(ui.BannerText(ui.TermWidth()))

	llm = provider.NewAnthropicProvider("deepseek-flash", 8192, systemPrompt)
	root = agent.New(llm, systemPrompt, tool.Default)
	root.Confirm = func(prompt string) bool {
		return ui.RequestApproval(prompt, "")
	}
	registerSubagents()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pipe:", err)
		os.Exit(1)
	}
	os.Stdout = w
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				ui.Post(string(buf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()

	ui.NewProgram(oldStdout, runUserInput, usageState, ui.BannerText(ui.TermWidth()))
}

func runUserInput(ctx context.Context, input string) error {
	if runCommand(input) {
		return nil
	}
	_, err := root.Send(ctx, input)

	return err
}

func usageState() string {
	if llm == nil {
		return ""
	}

	return llm.Model()
}

func registerSubagents() {
	subagent.Default.Register(subagent.Research{
		Provider: llm,
		Tools:    tool.Default.Subset("read_file", "bash"),
	})

	for _, sa := range subagent.Default.All() {
		tool.Default.Register(&DelegateTool{Subagent: sa})
	}
}
