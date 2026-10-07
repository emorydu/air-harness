package compact

import (
	"context"

	"github.com/emorydu/air-harness/ch11/internal/api"
)

type CompactionStrategy interface {
	Compact(ctx context.Context, messages []api.Message) ([]api.Message, error)
}

func SafeSplitPoint(messages []api.Message, desired int) int {
	if desired <= 0 {
		return 0
	}

	if desired >= len(messages) {
		return len(messages)
	}

	for i := desired; i > 0; i-- {
		if messages[i].Role == api.RoleUser && !messages[i].HasToolResult() {
			return i
		}
	}

	return 0
}
