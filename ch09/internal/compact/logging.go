package compact

import (
	"fmt"
	"os"
	"time"

	"github.com/emorydu/air-harness/ch09/internal/api"
)

func logCompaction(path string, before, after []api.Message) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintln(f, "=========================")
	fmt.Fprintf(f, "[%s] compaction event\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(f, "BEFORE (%d messages):\n%s\n", len(before), api.RenderTranscript(before))
	fmt.Fprintln(f, "---")
	fmt.Fprintf(f, "AFTER (%d messages):\n%s\n", len(after), api.RenderTranscript(after))
}
