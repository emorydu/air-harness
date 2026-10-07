package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emorydu/air-harness/ch11/internal/api"
	"github.com/emorydu/air-harness/ch11/internal/subagent"
	"github.com/emorydu/air-harness/ch11/internal/tool"
	"github.com/emorydu/air-harness/ch11/internal/ui"
)

type DelegateTool struct {
	Subagent subagent.Subagent
}

var _ tool.Tool = &DelegateTool{}

func (d *DelegateTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "delegate_" + d.Subagent.Name(),
		Description: d.Subagent.Description(),
		InputSchema: map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "Concrete description of what the subagent should do.",
			},
		},
		Required: []string{"task"},
	}
}

func (d *DelegateTool) Execute(ctx context.Context, rawInput string) (string, bool) {
	var in struct {
		Task string `json:"task"`
	}

	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid tool input: %v", err), true
	}

	name := d.Subagent.Name()
	fmt.Println(ui.Dimmed(fmt.Sprintf("↳ delegating to %s subagent", name)))

	start := time.Now()
	result, err := d.Subagent.Run(ctx, in.Task)
	elapsed := time.Since(start).Round(time.Millisecond)

	fmt.Println(ui.Dimmed(fmt.Sprintf("← %s subagent done (%s)", name, elapsed)))

	if err != nil {
		return fmt.Sprintf("subagent err: %v", err), true
	}

	return result, false
}
