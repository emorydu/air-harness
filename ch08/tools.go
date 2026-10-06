package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
)

type Tool interface {
	Definition() ToolDef
	Execute(ctx context.Context, input string) (result string, isError bool)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) { r.tools[t.Definition().Name] = t }

func (r *Registry) Definitions() []ToolDef {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]ToolDef, 0, len(names))

	for _, n := range names {
		out = append(out, r.tools[n].Definition())
	}

	return out
}

func (r *Registry) Execute(ctx context.Context, name, input string) (string, bool) {
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("unknown too: %s", name), true
	}

	return t.Execute(ctx, input)
}

var Default = NewRegistry()

type bashTool struct{}

func init() { Default.Register(bashTool{}) }

func (bashTool) Definition() ToolDef {
	return ToolDef{
		Name:        "bash",
		Description: "Run a shell command and return its combined stdout/stderr.",
		InputSchema: map[string]any{
			"command": map[string]any{"type": "string", "description": "The command to run."},
		},
		Required: []string{"command"},
	}
}

func (bashTool) Execute(ctx context.Context, rawInput string) (string, bool) {
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

type readFileTool struct{}

func init() { Default.Register(readFileTool{}) }

func (readFileTool) Definition() ToolDef {
	return ToolDef{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path.",
		InputSchema: map[string]any{
			"path": map[string]any{"type": "string", "description": "Filesystem path to read."},
		},
		Required: []string{"path"},
	}
}

func (readFileTool) Execute(_ context.Context, rawInput string) (string, bool) {
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

type writeFileTool struct{}

func init() { Default.Register(writeFileTool{}) }

func (writeFileTool) Definition() ToolDef {
	return ToolDef{
		Name:        "write_file",
		Description: "Write content to a file (creating or overwriting it).",
		InputSchema: map[string]any{
			"path":    map[string]any{"type": "string", "description": "Filesystem path to write."},
			"content": map[string]any{"type": "string", "description": "The bytes to write."},
		},
		Required: []string{"path", "content"},
	}
}

func (writeFileTool) Execute(_ context.Context, rawInput string) (string, bool) {
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
