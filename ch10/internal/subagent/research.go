package subagent

import (
	"context"

	"github.com/emorydu/air-harness/ch10/internal/agent"
	"github.com/emorydu/air-harness/ch10/internal/provider"
	"github.com/emorydu/air-harness/ch10/internal/tool"
)

type Research struct {
	Provider provider.Provider
	Tools    *tool.Registry
}

const researchSystem = `You are a research subagent. Your job is to investigate the
task you're given and return a concise, factual answer.

Rules:
- Use the tools available to look up information. Prefer fewer, more targeted
  reads over scanning everything.
- Return a short answer with the specific facts requested. No preamble.
- If the answer requires a path or identifier, include it verbatim.
- You have a limited number of tool calls; do not waste them.`

var _ Subagent = Research{}

func (Research) Name() string {
	return "research"
}

func (Research) Description() string {
	return "Investigate the codebase or filesystem and return a focused answer. " +
		"Its own context window means it can explore freely without polluting yours. " +
		"Always pass a concrete task description, not just the user's literal question."
}

func (r Research) Run(ctx context.Context, task string) (string, error) {
	done := Begin(r.Name())
	defer done()

	a := agent.New(r.Provider, researchSystem, r.Tools)
	a.Name = r.Name()

	a.Confirm = nil
	a.MaxTurns = 10

	return a.Send(ctx, task)
}
