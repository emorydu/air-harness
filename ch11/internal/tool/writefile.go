package tool

import (
	"context"
	"encoding/json"
	"os"

	"github.com/emorydu/air-harness/ch11/internal/api"
)

type WriteFileTool struct{}

func init() { Default.Register(&WriteFileTool{}) }

func (WriteFileTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "write_file",
		Description: "Write content to a file (creating or overwriting it).",
		InputSchema: map[string]any{
			"path":    map[string]any{"type": "string", "description": "Filesystem path to write."},
			"content": map[string]any{"type": "string", "description": "The bytes to write."},
		},
		Required: []string{"path", "content"},
	}
}

func (WriteFileTool) Execute(_ context.Context, rawInput string) (string, bool) {
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
}
