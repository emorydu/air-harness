package main

import (
	"context"
	"fmt"
	"os"

	"github.com/emorydu/air-harness/ch09/internal/agent"
	"github.com/emorydu/air-harness/ch09/internal/provider"
	"github.com/emorydu/air-harness/ch09/internal/tool"
	"github.com/emorydu/air-harness/ch09/internal/ui"
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

	llm = provider.NewAnthropicProvider("", 8192, systemPrompt)
	root = agent.New(llm, systemPrompt, tool.Default)
	root.Confirm = ui.Confirm

	ctx := context.Background()

	runREPL(ctx)
}

func runREPL(ctx context.Context) {
	for {
		input, ok := ui.ReadChatInput()
		if !ok {
			return
		}
		if input == "" {
			continue
		}
		if runCommand(input) {
			continue
		}

		if _, err := root.Send(ctx, input); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}
}
