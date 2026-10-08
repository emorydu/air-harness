// Package tool defines the Tool interface and a Registry that holds tools
// by name. Tools live as one file per tool in this package and self-register
// via init(), so adding a tool means dropping a file in this directory.
//
// Convention: each tool file is responsible for calling
// tool.Default().Register(myTool) in its own init().
package tool

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/emorydu/air-harness/internal/api"
)

type Tool interface {
	Definition() api.ToolDef
	Execute(ctx context.Context, input string) (result string, isErr bool)
}

type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	name := t.Definition().Name

	r.mu.Lock()
	r.tools[name] = t
	r.mu.Unlock()
}

// Get returns a previously-registered tool, used by Subset to compose
// curated tool registries for subagents.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]

	return t, ok
}

// Subset returns a new Registry containing only the named tools from r.
// Used to build curated tool sets for subagents (e.g. give a research
// subagent only read_file and web_search).
func (r *Registry) Subset(names ...string) *Registry {
	out := NewRegistry()
	for _, n := range names {
		if t, ok := r.Get(n); ok {
			out.Register(t)
		}
	}

	return out
}

// Definitions returns all registered tool schemas, sorted by name so the
// output is deterministic (important for prompt caching when you turn it on).
func (r *Registry) Definitions() []api.ToolDef {
	// Snapshot under lock, then sort outside the lock.
	r.mu.RLock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	r.mu.RUnlock()

	sort.Strings(names)

	out := make([]api.ToolDef, 0, len(names))
	for _, n := range names {
		if t, ok := r.Get(n); ok {
			out = append(out, t.Definition())
		}
	}

	return out
}

// Execute dispatches a tool call by name. Unknown tools return an error
// result rather than panicking — the model can read it and recover.
func (r *Registry) Execute(ctx context.Context, name, input string) (string, bool) {
	t, ok := r.Get(name)
	if !ok {
		return fmt.Sprintf("unknown tool: %s", name), true
	}

	return t.Execute(ctx, input)
}

// Default is the package-level registry. Tools in this package self-register
// to it via init() — the "drop a file in, it appears" pattern.
var Default = NewRegistry()
