package compact

import (
	"context"
	"fmt"

	"github.com/emorydu/air-harness/ch09/internal/api"
	"github.com/emorydu/air-harness/ch09/internal/provider"
)

type NoCompaction struct{}

var _ CompactionStrategy = (*NoCompaction)(nil)

func (NoCompaction) Compact(_ context.Context, m []api.Message) ([]api.Message, error) {
	return m, nil
}

type SlidingWindow struct{ KeepLast int }

var _ CompactionStrategy = (*SlidingWindow)(nil)

func (s *SlidingWindow) Compact(_ context.Context, messages []api.Message) ([]api.Message, error) {
	if len(messages) <= s.KeepLast {
		return messages, nil
	}

	split := SafeSplitPoint(messages, len(messages)-s.KeepLast)

	return messages[split:], nil
}

type Summarize struct {
	Provider   provider.Provider
	Threshold  int
	KeepRecent int
}

var _ CompactionStrategy = (*Summarize)(nil)

const summarizeInstructions = `Summarize the conversation below concisely. ` +
	`Preserve facts, decisions, file paths, code identifiers, and anything else ` +
	`needed to continue. Output the summary directly with no preamble.`

func (s *Summarize) Compact(ctx context.Context, messages []api.Message) ([]api.Message, error) {
	if len(messages) < s.Threshold {
		return messages, nil
	}

	split := SafeSplitPoint(messages, len(messages)-s.KeepRecent)
	if split == 0 {
		return messages, nil
	}
	old, recent := messages[:split], messages[split:]

	resp, err := s.Provider.Send(ctx, []api.Message{{
		Role:    api.RoleUser,
		Content: []api.Block{{Type: api.BlockText, Text: summarizeInstructions + "\n\n" + api.RenderTranscript(old)}},
	}}, nil)
	if err != nil {
		return messages, fmt.Errorf("summarize: %w", err)
	}

	var summary string
	for _, b := range resp.Content {
		if b.Type == api.BlockText {
			summary = b.Text
			break
		}
	}
	if summary == "" {
		return messages, fmt.Errorf("summarize: empty response")
	}

	fmt.Printf("[compacted %d messages → summary]\n", len(old))

	return append([]api.Message{{
		Role:    api.RoleUser,
		Content: []api.Block{{Type: api.BlockText, Text: "[earlier conversation summary]\n" + summary}},
	}}, recent...), nil
}

type LoggingStrategy struct {
	Inner    CompactionStrategy
	FilePath string
}

var _ CompactionStrategy = (*LoggingStrategy)(nil)

func (l *LoggingStrategy) Compact(ctx context.Context, messages []api.Message) ([]api.Message, error) {
	before := messages
	after, err := l.Inner.Compact(ctx, messages)
	if err != nil || len(after) == len(before) {
		return after, err
	}
	logCompaction(l.FilePath, before, after)

	return after, nil
}
