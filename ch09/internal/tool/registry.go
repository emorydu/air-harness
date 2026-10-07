package tool

import (
	"context"
	"fmt"
	"sort"

	"github.com/emorydu/air-harness/ch09/internal/api"
)

type Tool interface {
	Definition() api.ToolDef
	Execute(ctx context.Context, input string) (result string, isError bool)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]Tool{}} }

func (r *Registry) Register(t Tool) { r.tools[t.Definition().Name] = t }

func (r *Registry) Definitions() []api.ToolDef {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}

	sort.Strings(names)

	out := make([]api.ToolDef, 0, len(r.tools))
	for _, n := range names {
		out = append(out, r.tools[n].Definition())
	}

	return out
}

func (r *Registry) Execute(ctx context.Context, name, input string) (string, bool) {
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("unknown tool: %s", name), true
	}

	return t.Execute(ctx, input)
}

var Default = NewRegistry()
