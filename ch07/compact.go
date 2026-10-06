package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type CompactionStrategy interface {
	Compact(ctx context.Context, messages []Message) ([]Message, error)
}

func SafeSplitPoint(messages []Message, desired int) int {
	if desired <= 0 {
		return 0
	}
	if desired >= len(messages) {
		return len(messages)
	}
	for i := desired; i > 0; i-- {
		if messages[i].Role == RoleUser && !hasToolResult(messages[i]) {
			return i
		}
	}

	return 0
}

func hasToolResult(m Message) bool {
	for _, b := range m.Content {
		if b.Type == BlockToolResult {
			return true
		}
	}

	return false
}

type NoCompaction struct{}

var _ CompactionStrategy = NoCompaction{}

func (NoCompaction) Compact(_ context.Context, m []Message) ([]Message, error) { return m, nil }

type SlidingWindow struct {
	KeepLast int
}

var _ CompactionStrategy = &SlidingWindow{}

func (s *SlidingWindow) Compact(ctx context.Context, messages []Message) ([]Message, error) {
	if len(messages) <= s.KeepLast {
		return messages, nil
	}
	split := SafeSplitPoint(messages, len(messages)-s.KeepLast)

	return messages[split:], nil
}

type Summarize struct {
	Provider   Provider
	Threshold  int
	KeepRecent int
}

var _ CompactionStrategy = &Summarize{}

const summarizeInstructions = `Summarize the conversation below concisely. ` +
	`Preserve facts, decisions, file paths, code identifiers, and anything else ` +
	`needed to continue. Output the summary directly with no preamble`

func (s *Summarize) Compact(ctx context.Context, messages []Message) ([]Message, error) {
	if len(messages) < s.Threshold {
		return messages, nil
	}
	split := SafeSplitPoint(messages, len(messages)-s.KeepRecent)
	if split == 0 {
		return messages, nil
	}

	old, recent := messages[:split], messages[split:]

	resp, err := s.Provider.Send(ctx, []Message{{
		Role:    RoleUser,
		Content: []Block{{Type: BlockText, Text: summarizeInstructions + "\n\n" + renderTranscript(old)}},
	}}, nil)
	if err != nil {
		return messages, fmt.Errorf("summarize: %w", err)
	}

	var summary string
	for _, b := range resp.Content {
		if b.Type == BlockText {
			summary = b.Text
			break
		}
	}
	if summary == "" {
		return messages, fmt.Errorf("summarize: empty response")
	}

	fmt.Printf("[compacted %d messages → summary\n", len(old))

	return append([]Message{{
		Role:    RoleUser,
		Content: []Block{{Type: BlockText, Text: "[earlier conversation summary]\n" + summary}},
	}}, recent...), nil
}

func renderTranscript(msgs []Message) string {
	var sb strings.Builder

	for _, m := range msgs {
		sb.WriteString(string(m.Role))
		sb.WriteString(": ")
		for _, b := range m.Content {
			switch b.Type {
			case BlockText:
				sb.WriteString(b.Text)
			case BlockToolUse:
				fmt.Fprintf(&sb, "[called %s with %s]", b.ToolName, b.ToolInput)
			case BlockToolResult:
				fmt.Fprintf(&sb, "[tool result: %s]", b.ToolResult)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

type LoggingStrategy struct {
	Inner    CompactionStrategy
	FilePath string
}

var _ CompactionStrategy = &LoggingStrategy{}

func (l *LoggingStrategy) Compact(ctx context.Context, messages []Message) ([]Message, error) {
	before := messages
	after, err := l.Inner.Compact(ctx, messages)
	if err != nil {
		return after, err
	}
	if len(after) == len(before) {
		return after, nil
	}

	if l.FilePath != "" {
		f, ferr := os.OpenFile(l.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if ferr == nil {
			fmt.Fprintln(f, "=========================")
			fmt.Fprintf(f, "[%s] compaction event\n", time.Now().UTC().Format(time.RFC3339))
			fmt.Fprintf(f, "BEFORE (%d messages):\n%s\n", len(before), renderTranscript(before))
			fmt.Fprintln(f, "---")
			fmt.Fprintf(f, "AFTER (%d messages):\n%s\n", len(after), renderTranscript(after))
			f.Close()
		}
	}

	return after, nil
}
