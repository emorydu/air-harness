package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type command struct {
	description string
	usage       string // optional
	run         func(args string)
}

var commands = map[string]command{}

func init() {
	commands["help"] = command{
		description: "show available commands",
		run:         cmdHelp,
	}
	commands["model"] = command{
		description: "show or change the model",
		usage:       "/model [name]",
		run:         cmdModel,
	}
	commands["clear"] = command{
		description: "clear conversation history",
		run:         cmdClear,
	}

	// ========================================================================================
	commands["compact"] = command{
		description: "run compaction now (optionally with a specific strategy)",
		usage:       "/compact [sliding|summarize|none]",
		run:         cmdCompact,
	}
	commands["verbose"] = command{
		description: "toggle printing of compaction before/after",
		usage:       "/verbose [on|off]",
		run:         cmdVerbose,
	}
	// ========================================================================================

	commands["tools"] = command{
		description: "list available tools",
		run:         cmdTools,
	}
	commands["exit"] = command{
		description: "exit the harness",
		run:         cmdExit,
	}
}

func runCommand(line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(line, "/"), " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	c, ok := commands[name]
	if !ok {
		fmt.Printf("unknown command: /%s (try /help)\n", name)
		return true
	}

	c.run(args)

	return true
}

var knownModels = []string{
	"claude-opus-4-7",
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-haiku-4-5",
}

func cmdModel(args string) {
	if args == "" {
		fmt.Printf("current: %s\n", llm.Model())
		fmt.Println("suggestions:")
		for _, m := range knownModels {
			fmt.Printf("  %s\n", m)
		}
		return
	}
	llm.SetModel(args)
	fmt.Printf("model: %s\n", args)
}

func cmdClear(_ string) {
	messages = messages[:0]
	fmt.Println("conversation cleared")
}

// ========================================================================================

func cmdCompact(args string) {
	var s CompactionStrategy
	switch args {
	case "":
		s = compactor
	case "sliding":
		s = &SlidingWindow{KeepLast: 6}
	case "summarize":
		s = &Summarize{
			Provider:   llm,
			Threshold:  0,
			KeepRecent: 4,
		}
	case "none":
		s = NoCompaction{}
	default:
		fmt.Printf("unknown strategy: %s (try sliding|summarize|none)\n", args)
		return
	}

	before := len(messages)
	after, err := s.Compact(context.Background(), messages)
	if err != nil {
		fmt.Printf("compact error: %v\n", err)
		return
	}
	messages = after
	fmt.Printf("compacted: %d → %d messages\n", before, len(messages))
}

func cmdVerbose(args string) {
	switch args {
	case "on":
		verbose = true
	case "off":
		verbose = false
	default:
		verbose = !verbose
	}
	fmt.Printf("verbose: %v\n", verbose)
}

// ========================================================================================

func cmdTools(_ string) {
	for _, t := range Default.Definitions() {
		fmt.Printf("  %-12s %s\n", t.Name, t.Description)
	}
}

func cmdHelp(_ string) {
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := commands[n]
		display := "/" + n
		if c.usage != "" {
			display = c.usage
		}
		fmt.Printf("  %-22s %s\n", display, c.description)
	}
}

func cmdExit(_ string) {
	fmt.Println("bye")

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("conversation.json", data, 0644)
	if err != nil {
		panic(err)
	}

	exitFunc()
}
