package tool

import (
	"context"
	"encoding/json"
	"os"

	"github.com/emorydu/air-harness/ch09/internal/api"
)

type ReadFileTool struct{}

func init() { Default.Register(&ReadFileTool{}) }

func (ReadFileTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path.",
		InputSchema: map[string]any{
			"path": map[string]any{"type": "string", "description": "Filesystem path to read."},
		},
		Required: []string{"path"},
	}
}

func (ReadFileTool) Execute(_ context.Context, rawInput string) (string, bool) {
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
}
