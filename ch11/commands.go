package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/emorydu/air-harness/ch11/internal/compact"
	"github.com/emorydu/air-harness/ch11/internal/subagent"
	"github.com/emorydu/air-harness/ch11/internal/tool"
)

type command struct {
	description string
	usage       string
	run         func(args string)
}

var commands = map[string]command{}

func init() {
	commands["help"] = command{description: "show available commands", run: cmdHelp}
	commands["model"] = command{description: "show or change the model", usage: "/model [name]", run: cmdModel}
	commands["clear"] = command{description: "clear conversation history", run: cmdClear}
	commands["tools"] = command{description: "list available tools", run: cmdTools}
	commands["subagents"] = command{description: "list subagents (registered and currently running)", run: cmdSubagents}
	commands["compact"] = command{description: "run compaction now", usage: "/compact [sliding|summarize|none]", run: cmdCompact}
	commands["exit"] = command{description: "exit the harness", run: cmdExit}
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
	"claude-opus-4-7", "claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5",
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
	if root != nil {
		root.ClearMessages()
	}

	fmt.Println("conversation cleared")
}

func cmdTools(_ string) {
	for _, t := range tool.Default.Definitions() {
		fmt.Printf("  %-12s %s\n", t.Name, t.Description)
	}
}

func cmdSubagents(_ string) {
	fmt.Println("registered:")
	for _, sa := range subagent.Default.All() {
		fmt.Printf("  %-12s %s\n", sa.Name(), firstLine(sa.Description()))
	}

	if active := subagent.Active(); len(active) > 0 {
		fmt.Println("running:")
		for name, n := range active {
			fmt.Printf("  %-12s %d\n", name, n)
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func cmdCompact(args string) {
	if root == nil {
		fmt.Println("no active agent")
		return
	}

	var s compact.CompactionStrategy
	switch args {
	case "", "none":
		s = compact.NoCompaction{}
	case "sliding":
		s = &compact.SlidingWindow{KeepLast: 6}
	case "summarize":
		s = &compact.Summarize{Provider: llm, Threshold: 0, KeepRecent: 4}
	default:
		fmt.Printf("unknown strategy: %s\n", args)
		return
	}

	before := len(root.Messages())
	after, err := s.Compact(context.Background(), root.Messages())
	if err != nil {
		fmt.Printf("compact error: %v\n", err)
		return
	}
	root.SetMessages(after)

	fmt.Printf("compacted: %d → %d messages\n", before, len(root.Messages()))
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
	exitFunc()
}
