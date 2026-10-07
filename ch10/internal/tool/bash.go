package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/emorydu/air-harness/ch10/internal/api"
)

type BashTool struct{}

func init() { Default.Register(&BashTool{}) }

func (BashTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "bash",
		Description: "Run a shell command and return its combined stdout/stderr.",
		InputSchema: map[string]any{
			"command": map[string]any{"type": "string", "description": "The command to run."},
		},
		Required: []string{"command"},
	}
}

func (BashTool) Execute(ctx context.Context, rawInput string) (string, bool) {
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
}
