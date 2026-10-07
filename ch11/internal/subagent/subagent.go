package subagent

import (
	"context"
	"sort"
	"sync"
)

type Subagent interface {
	Name() string
	Description() string
	Run(ctx context.Context, task string) (string, error)
}

type Registry struct {
	mu        sync.RWMutex
	subagents map[string]Subagent
}

func NewRegistry() *Registry {
	return &Registry{
		subagents: make(map[string]Subagent),
	}
}

func (r *Registry) Register(s Subagent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subagents[s.Name()] = s
}

func (r *Registry) All() []Subagent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.subagents))
	for n := range r.subagents {
		names = append(names, n)
	}

	sort.Strings(names)
	out := make([]Subagent, 0, len(names))
	for _, n := range names {
		out = append(out, r.subagents[n])
	}

	return out
}

type tracker struct {
	mu     sync.Mutex
	active map[string]int
}

var trk = &tracker{active: make(map[string]int)}

func Begin(name string) func() {
	trk.mu.Lock()
	trk.active[name]++
	trk.mu.Unlock()

	return func() {
		trk.mu.Lock()
		trk.active[name]--
		if trk.active[name] == 0 {
			delete(trk.active, name)
		}
		trk.mu.Unlock()
	}
}

func Active() map[string]int {
	trk.mu.Lock()
	defer trk.mu.Unlock()
	out := make(map[string]int, len(trk.active))
	for k, v := range trk.active {
		out[k] = v
	}
	return out
}

var Default = NewRegistry()
