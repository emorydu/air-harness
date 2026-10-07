package api

import (
	"fmt"
	"strings"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

type Block struct {
	Type BlockType

	Text string // BlockText

	ToolUseID string // BlockToolUse / BlockToolResult
	ToolName  string // BlockTooUse
	ToolInput string // BlockToolUse - RawJSON

	ToolResult string // BlockToolResult
	IsError    bool   // BlockToolResult
}

type Message struct {
	Role    Role
	Content []Block
}

func (m Message) HasToolResult() bool {
	for _, b := range m.Content {
		if b.Type == BlockToolResult {
			return true
		}
	}

	return false
}

type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
	Required    []string
}

type StopReason string

const (
	StopEndTurn StopReason = "end_turn"
	StopToolUse StopReason = "tool_use"
	StopOther   StopReason = "other"
)

type Response struct {
	Content    []Block
	StopReason StopReason
}

func RenderTranscript(msgs []Message) string {
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
